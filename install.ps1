$ErrorActionPreference = "Stop"

$Repo = "AgusRdz/tally"
$InstallDir = if ($env:TALLY_INSTALL_DIR) { $env:TALLY_INSTALL_DIR } else { "$env:LOCALAPPDATA\Programs\tally" }

# Detect architecture
$Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq [System.Runtime.InteropServices.Architecture]::Arm64) {
    "arm64"
} else {
    "amd64"
}

$Binary = "tally-windows-$Arch.exe"

# Get latest version
if (-not $env:TALLY_VERSION) {
    $Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $env:TALLY_VERSION = $Release.tag_name
}

if (-not $env:TALLY_VERSION) {
    Write-Error "failed to determine latest version"
    exit 1
}

$BaseUrl = "https://github.com/$Repo/releases/download/$($env:TALLY_VERSION)"

Write-Host "installing tally $($env:TALLY_VERSION) (windows/$Arch)..."

# Create install dir
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# Download binary, checksums, and signature
$Destination = Join-Path $InstallDir "tally.exe"
Invoke-WebRequest -Uri "$BaseUrl/$Binary" -OutFile $Destination
Invoke-WebRequest -Uri "$BaseUrl/checksums.txt" -OutFile "$env:TEMP\tally-checksums.txt"
Invoke-WebRequest -Uri "$BaseUrl/checksums.txt.sig" -OutFile "$env:TEMP\tally-checksums.txt.sig"

# Verify signature using embedded public key
$PublicKeyPem = @"
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8qoTHgNH6uf+q8+EDQvgE64Xa2C6
8kwORstSYDaOG4TSW5XIArjkR4Ozi4DNDZ4F/Xs6iK2aNM83WMJeegLYyg==
-----END PUBLIC KEY-----
"@
$PublicKeyPem | Set-Content "$env:TEMP\tally-public.pem" -Encoding ASCII

$SigHex = Get-Content "$env:TEMP\tally-checksums.txt.sig" -Raw
$SigBytes = [byte[]]($SigHex.Trim() -split '(.{2})' | Where-Object { $_ } | ForEach-Object { [Convert]::ToByte($_, 16) })
[System.IO.File]::WriteAllBytes("$env:TEMP\tally-checksums.txt.sig.bin", $SigBytes)

$VerifyResult = & openssl pkeyutl -verify -pubin -inkey "$env:TEMP\tally-public.pem" -rawin `
    -in "$env:TEMP\tally-checksums.txt" -sigfile "$env:TEMP\tally-checksums.txt.sig.bin" 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Error "ERROR: signature verification failed — aborting`n$VerifyResult"
    Remove-Item $Destination -ErrorAction SilentlyContinue
    Remove-Item "$env:TEMP\tally-checksums.txt", "$env:TEMP\tally-checksums.txt.sig", "$env:TEMP\tally-checksums.txt.sig.bin", "$env:TEMP\tally-public.pem" -ErrorAction SilentlyContinue
    exit 1
}

# Verify checksum of the downloaded binary
$ChecksumLine = Get-Content "$env:TEMP\tally-checksums.txt" | Where-Object { $_ -match [regex]::Escape($Binary) }
if (-not $ChecksumLine) { Write-Error "Binary not found in checksums.txt"; exit 1 }
$ExpectedHash = ($ChecksumLine -split '\s+')[0].ToUpper()
$ActualHash = (Get-FileHash -Algorithm SHA256 $Destination).Hash.ToUpper()
if ($ActualHash -ne $ExpectedHash) {
    Write-Error "ERROR: checksum mismatch — aborting"
    Remove-Item $Destination -ErrorAction SilentlyContinue
    exit 1
}

Remove-Item "$env:TEMP\tally-checksums.txt", "$env:TEMP\tally-checksums.txt.sig", "$env:TEMP\tally-checksums.txt.sig.bin", "$env:TEMP\tally-public.pem" -ErrorAction SilentlyContinue

Write-Host "installed tally to $Destination"
Write-Host ""

# Add to user PATH if not already present
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
$CleanInstallDir = $InstallDir.TrimEnd("\")
$PathParts = $UserPath -split ";" | ForEach-Object { $_.TrimEnd("\") }

if ($PathParts -notcontains $CleanInstallDir) {
    $NewUserPath = "$InstallDir;$UserPath"
    [Environment]::SetEnvironmentVariable("PATH", $NewUserPath, "User")
    Write-Host "added $InstallDir to PATH"
}

# Update current session PATH so tally is usable immediately
$CurrentPathParts = $env:PATH -split ";" | ForEach-Object { $_.TrimEnd("\") }
if ($CurrentPathParts -notcontains $CleanInstallDir) {
    $env:PATH = "$InstallDir;$env:PATH"
}

# Notify running processes of PATH change
$HWND_BROADCAST = [IntPtr]0xffff
$WM_SETTINGCHANGE = 0x001a
$MethodDefinition = @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, IntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out IntPtr lpdwResult);
'@
$User32 = Add-Type -MemberDefinition $MethodDefinition -Name "User32" -Namespace "Win32" -PassThru
$result = [IntPtr]::Zero
$User32::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [IntPtr]::Zero, "Environment", 2, 100, [ref]$result) | Out-Null

# Register the Claude Code hooks
& $Destination init

Write-Host ""
Write-Host "done! tally will track context usage and warn before degradation."
