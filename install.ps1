# Lumina-TUI installer for Windows.
#   irm https://raw.githubusercontent.com/shivarchit/Lumina-TUI/master/install.ps1 | iex
$ErrorActionPreference = "Stop"

$repo = "shivarchit/Lumina-TUI"
$asset = "lumina-windows-x64.exe"
$url = "https://github.com/$repo/releases/latest/download/$asset"

$installDir = Join-Path $env:USERPROFILE "bin"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
$target = Join-Path $installDir "lumina.exe"

Write-Host "Downloading $asset ..."
Invoke-WebRequest -Uri $url -OutFile $target

if (-not (Test-Path $target) -or (Get-Item $target).Length -eq 0) {
    throw "Download failed or empty file"
}

Write-Host "Installed lumina to $target"
& $target -v

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host ""
    Write-Host "Added $installDir to your user PATH. Open a new terminal, then run: lumina"
} else {
    Write-Host "Run: lumina"
}
