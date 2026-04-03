#!/bin/sh
set -e

REPO="AgusRdz/tally"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *) echo "unsupported OS: $OS" >&2; exit 1 ;;
esac

# Set default install dir
if [ -z "$TALLY_INSTALL_DIR" ]; then
  if [ "$OS" = "windows" ]; then
    INSTALL_DIR="$(cygpath "$LOCALAPPDATA/Programs/tally" 2>/dev/null || echo "$HOME/AppData/Local/Programs/tally")"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
else
  INSTALL_DIR="$TALLY_INSTALL_DIR"
fi

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

EXT=""
if [ "$OS" = "windows" ]; then
  EXT=".exe"
fi

BINARY="tally-${OS}-${ARCH}${EXT}"

# Get latest version
if [ -z "$TALLY_VERSION" ]; then
  TALLY_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//')
fi

if [ -z "$TALLY_VERSION" ]; then
  echo "failed to determine latest version" >&2
  exit 1
fi

BASE_URL="https://github.com/${REPO}/releases/download/${TALLY_VERSION}"

echo "installing tally ${TALLY_VERSION} (${OS}/${ARCH})..."

mkdir -p "$INSTALL_DIR"

# Download binary, checksums, and signature
curl -fsSL "${BASE_URL}/${BINARY}" -o "${INSTALL_DIR}/tally${EXT}"
curl -fsSL "${BASE_URL}/checksums.txt" -o /tmp/tally-checksums.txt
curl -fsSL "${BASE_URL}/checksums.txt.sig" -o /tmp/tally-checksums.txt.sig

# Verify signature using embedded public key
PUBLIC_KEY='-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8qoTHgNH6uf+q8+EDQvgE64Xa2C6
8kwORstSYDaOG4TSW5XIArjkR4Ozi4DNDZ4F/Xs6iK2aNM83WMJeegLYyg==
-----END PUBLIC KEY-----'

echo "$PUBLIC_KEY" > /tmp/tally-public.pem
SIG_HEX="$(cat /tmp/tally-checksums.txt.sig)"
printf '%s' "$SIG_HEX" | xxd -r -p > /tmp/tally-checksums.txt.sig.bin

if ! openssl pkeyutl -verify -pubin -inkey /tmp/tally-public.pem -rawin \
     -in /tmp/tally-checksums.txt -sigfile /tmp/tally-checksums.txt.sig.bin 2>/dev/null; then
  echo "ERROR: signature verification failed — aborting" >&2
  rm -f "${INSTALL_DIR}/tally${EXT}" /tmp/tally-checksums.txt /tmp/tally-checksums.txt.sig /tmp/tally-checksums.txt.sig.bin /tmp/tally-public.pem
  exit 1
fi

# Verify checksum of the downloaded binary
(cd "$INSTALL_DIR" && grep "${BINARY}" /tmp/tally-checksums.txt | sha256sum -c -) || \
  { echo "ERROR: checksum mismatch — aborting" >&2; rm -f "${INSTALL_DIR}/tally${EXT}"; exit 1; }

rm -f /tmp/tally-checksums.txt /tmp/tally-checksums.txt.sig /tmp/tally-checksums.txt.sig.bin /tmp/tally-public.pem

chmod +x "${INSTALL_DIR}/tally${EXT}"

echo "installed tally to ${INSTALL_DIR}/tally${EXT}"
echo ""

# Add to PATH if not already present
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    if [ "$OS" = "windows" ]; then
      WIN_DIR=$(cygpath -w "$INSTALL_DIR" 2>/dev/null || echo "$INSTALL_DIR")
      powershell.exe -NoProfile -Command "\$p = [Environment]::GetEnvironmentVariable('Path', 'User'); \$d = '${WIN_DIR}'.TrimEnd('\\'); if ((\$p -split ';' | ForEach-Object { \$_.TrimEnd('\\') }) -notcontains \$d) { [Environment]::SetEnvironmentVariable('Path', \"\$d;\$p\", 'User'); Write-Host \"Added \$d to User PATH\" }"
      export PATH="${INSTALL_DIR}:$PATH"
    else
      SHELL_NAME="$(basename "${SHELL:-}")"
      case "$SHELL_NAME" in
        zsh)  SHELL_RC="$HOME/.zshrc" ;;
        bash) SHELL_RC="$HOME/.bashrc" ;;
        *)    SHELL_RC="" ;;
      esac

      PATH_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""

      if [ -n "$SHELL_RC" ]; then
        if ! grep -qF "$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
          printf '\n# tally\n%s\n' "$PATH_LINE" >> "$SHELL_RC"
          echo "added ${INSTALL_DIR} to PATH in $SHELL_RC"
          echo "reload your shell: source $SHELL_RC"
        fi
      else
        echo "NOTE: add ${INSTALL_DIR} to your PATH:"
        echo "  $PATH_LINE"
      fi
      export PATH="${INSTALL_DIR}:$PATH"
      echo ""
    fi
    ;;
esac

# Register the Claude Code hooks
"${INSTALL_DIR}/tally${EXT}" init

echo ""
echo "done! tally will track context usage and warn before degradation."
