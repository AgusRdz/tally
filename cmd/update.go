package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const repo = "AgusRdz/tally"

// Update checks for a newer version and replaces the binary if one is found.
func Update(current string) {
	latest, err := latestVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: failed to check for updates: %v\n", err)
		os.Exit(1)
	}

	if latest == current || (current != "dev" && normalize(latest) == normalize(current)) {
		fmt.Printf("tally %s is already up to date\n", current)
		return
	}

	if current == "dev" {
		fmt.Printf("tally dev build — latest release is %s\n", latest)
		fmt.Println("install a release build to enable updates")
		return
	}

	fmt.Printf("updating tally %s → %s...\n", current, latest)

	url := assetURL(latest)
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tally: cannot determine executable path: %v\n", err)
		os.Exit(1)
	}
	exe, _ = filepath.EvalSymlinks(exe)

	if err := downloadReplace(url, exe); err != nil {
		fmt.Fprintf(os.Stderr, "tally: update failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("updated to %s\n", latest)
}

func latestVersion() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

func assetURL(version string) string {
	goos := runtime.GOOS
	arch := runtime.GOARCH

	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}

	binary := fmt.Sprintf("tally-%s-%s%s", goos, arch, ext)
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, binary)
}

func downloadReplace(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	tmp := dest + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", tmp, err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()

	// On Windows, rename current → .old then tmp → current
	// (can't delete a running .exe, but can rename it)
	if runtime.GOOS == "windows" {
		old := dest + ".old"
		os.Remove(old)
		if err := os.Rename(dest, old); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("cannot rename current binary: %w", err)
		}
		if err := os.Rename(tmp, dest); err != nil {
			os.Rename(old, dest)
			return fmt.Errorf("cannot replace binary: %w", err)
		}
		return nil
	}

	return os.Rename(tmp, dest)
}

func normalize(v string) string {
	return strings.TrimPrefix(v, "v")
}
