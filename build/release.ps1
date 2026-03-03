param(
    [string]$Target = "all",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

if ($Target -in @("-h", "--help")) {
    Write-Host "Usage: .\build\release.ps1 [-Target all|goos/goarch] [-DryRun]"
    Write-Host "Examples:"
    Write-Host "  .\build\release.ps1"
    Write-Host "  .\build\release.ps1 -Target all"
    Write-Host "  .\build\release.ps1 -Target linux/amd64"
    Write-Host "  .\build\release.ps1 -Target all -DryRun"
    exit 0
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = Resolve-Path (Join-Path $scriptDir "..")
Set-Location $rootDir

$versionFile = Join-Path $rootDir "internal/version/version.go"
$versionContent = Get-Content $versionFile -Raw
$versionMatch = [regex]::Match($versionContent, 'v[0-9]+\.[0-9]+\.[0-9]+')
if (-not $versionMatch.Success) {
    throw "Could not find version in internal/version/version.go"
}
$version = $versionMatch.Value

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is required but was not found in PATH"
}

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    throw "GitHub CLI (gh) is required for release upload"
}

Write-Host "Releasing $version (target=$Target, dry-run=$($DryRun.IsPresent))"
$distDir = Join-Path $rootDir "dist"
if (Test-Path $distDir) {
    Remove-Item $distDir -Recurse -Force
}
New-Item -ItemType Directory -Path $distDir | Out-Null

$platforms = @(
    @{ GOOS = "darwin";  GOARCH = "amd64"; Output = "lumina-mac-x64" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; Output = "lumina-mac-arm64" },
    @{ GOOS = "linux";   GOARCH = "amd64"; Output = "lumina-linux-x64" },
    @{ GOOS = "linux";   GOARCH = "arm";   Output = "lumina-linux-arm" },
    @{ GOOS = "linux";   GOARCH = "arm64"; Output = "lumina-linux-arm64" },
    @{ GOOS = "windows"; GOARCH = "amd64"; Output = "lumina-windows-x64.exe" },
    @{ GOOS = "freebsd"; GOARCH = "amd64"; Output = "lumina-freebsd-x64" }
)

Write-Host "Building binaries..."
$builtAny = $false

foreach ($platform in $platforms) {
    $goosGoarch = "$($platform.GOOS)/$($platform.GOARCH)"
    if ($Target -ne "all" -and $Target -ne $goosGoarch) {
        continue
    }

    $builtAny = $true
    Write-Host "  - $goosGoarch -> $($platform.Output)"

    $env:GOOS = $platform.GOOS
    $env:GOARCH = $platform.GOARCH
    & go build -ldflags "-s -w" -o (Join-Path $distDir $platform.Output) ./internal
}

$env:GOOS = $null
$env:GOARCH = $null

if (-not $builtAny) {
    Write-Host "No matching build target for '$Target'. Use 'all' or one of:"
    foreach ($platform in $platforms) {
        Write-Host "  - $($platform.GOOS)/$($platform.GOARCH)"
    }
    exit 1
}

Write-Host "Generating checksums..."
$checksumLines = Get-ChildItem $distDir -File | ForEach-Object {
    $hash = (Get-FileHash -Algorithm SHA256 $_.FullName).Hash.ToLower()
    "$hash  $($_.Name)"
}
$checksumPath = Join-Path $distDir "checksums.txt"
Set-Content -Path $checksumPath -Value $checksumLines

$archive = "lumina-tui-$version.zip"
$archivePath = Join-Path $rootDir $archive
Write-Host "Creating archive $archive..."
if (Test-Path $archivePath) {
    Remove-Item $archivePath -Force
}
Compress-Archive -Path (Join-Path $distDir "*") -DestinationPath $archivePath

Write-Host "Ensuring git tag $version exists..."
$localTagExists = $false
& git rev-parse -q --verify "refs/tags/$version" *> $null
if ($LASTEXITCODE -eq 0) {
    $localTagExists = $true
}

if ($localTagExists) {
    Write-Host "  - Tag already exists locally"
} else {
    if ($DryRun) {
        Write-Host "  - DRY RUN: would create local tag $version"
    } else {
        & git tag -a $version -m "Release $version"
        Write-Host "  - Created local tag $version"
    }
}

$remoteTagExists = $false
& git ls-remote --exit-code --tags origin $version *> $null
if ($LASTEXITCODE -eq 0) {
    $remoteTagExists = $true
}

if ($remoteTagExists) {
    Write-Host "  - Tag already exists on origin"
} else {
    if ($DryRun) {
        Write-Host "  - DRY RUN: would push tag $version to origin"
    } else {
        & git push origin $version
        Write-Host "  - Pushed tag $version to origin"
    }
}

Write-Host "Publishing artifacts to GitHub Release..."
if ($DryRun) {
    & gh release view $version *> $null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  - DRY RUN: would upload assets to existing release $version"
    } else {
        Write-Host "  - DRY RUN: would create release $version and upload assets"
    }
} else {
    & gh release view $version *> $null
    if ($LASTEXITCODE -eq 0) {
        $assets = (Get-ChildItem $distDir -File | ForEach-Object { $_.FullName }) + $archivePath
        & gh release upload $version @assets --clobber
        Write-Host "  - Uploaded assets to existing release $version"
    } else {
        $assets = (Get-ChildItem $distDir -File | ForEach-Object { $_.FullName }) + $archivePath
        & gh release create $version @assets --title $version --notes "Release $version"
        Write-Host "  - Created release $version and uploaded assets"
    }
}

Write-Host "Done. Artifacts are available in GitHub Releases for $version"
