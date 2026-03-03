#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

TARGET="${1:-all}"

VERSION="$(grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' internal/version/version.go | head -n 1)"
if [[ -z "$VERSION" ]]; then
    echo "Could not find version in internal/version/version.go"
    exit 1
fi

if ! command -v go >/dev/null 2>&1; then
    echo "Go is required but was not found in PATH"
    exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
    echo "GitHub CLI (gh) is required for release upload"
    exit 1
fi

echo "Releasing $VERSION (target=$TARGET)"
rm -rf dist
mkdir -p dist

PLATFORMS=(
    "darwin/amd64:lumina-mac-x64"
    "darwin/arm64:lumina-mac-arm64"
    "linux/amd64:lumina-linux-x64"
    "linux/arm:lumina-linux-arm"
    "linux/arm64:lumina-linux-arm64"
    "windows/amd64:lumina-windows-x64.exe"
    "freebsd/amd64:lumina-freebsd-x64"
)

echo "Building binaries..."
built_any="false"
for platform in "${PLATFORMS[@]}"; do
    IFS=':' read -r goos_goarch output <<< "$platform"
    IFS='/' read -r goos goarch <<< "$goos_goarch"

    if [[ "$TARGET" != "all" && "$TARGET" != "$goos_goarch" ]]; then
        continue
    fi

    built_any="true"
    echo "  - $goos/$goarch -> $output"
    GOOS="$goos" GOARCH="$goarch" go build -ldflags "-s -w" -o "dist/$output" ./internal
done

if [[ "$built_any" != "true" ]]; then
    echo "No matching build target for '$TARGET'. Use 'all' or one of:"
    for platform in "${PLATFORMS[@]}"; do
        IFS=':' read -r goos_goarch _ <<< "$platform"
        echo "  - $goos_goarch"
    done
    exit 1
fi

echo "Generating checksums..."
(cd dist && sha256sum * > checksums.txt)

ARCHIVE="lumina-tui-$VERSION.tar.gz"
echo "Creating archive $ARCHIVE..."
tar -czf "$ARCHIVE" dist/

echo "Ensuring git tag $VERSION exists..."
if git rev-parse -q --verify "refs/tags/$VERSION" >/dev/null 2>&1; then
    echo "  - Tag already exists locally"
else
    git tag -a "$VERSION" -m "Release $VERSION"
    echo "  - Created local tag $VERSION"
fi

if git ls-remote --exit-code --tags origin "$VERSION" >/dev/null 2>&1; then
    echo "  - Tag already exists on origin"
else
    git push origin "$VERSION"
    echo "  - Pushed tag $VERSION to origin"
fi

echo "Publishing artifacts to GitHub Release..."
if gh release view "$VERSION" >/dev/null 2>&1; then
    gh release upload "$VERSION" dist/* "$ARCHIVE" --clobber
    echo "  - Uploaded assets to existing release $VERSION"
else
    gh release create "$VERSION" dist/* "$ARCHIVE" --title "$VERSION" --notes "Release $VERSION"
    echo "  - Created release $VERSION and uploaded assets"
fi

echo "Done. Artifacts are available in GitHub Releases for $VERSION"