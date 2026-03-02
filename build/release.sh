#!/bin/bash

# Extract version from internal/version/version.go
VERSION=$(grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' internal/version/version.go)

if [ -z "$VERSION" ]; then
    echo "❌ Could not find version in version.go"
    exit 1
fi

echo "🚀 Releasing $VERSION..."
rm -rf dist && mkdir -p dist

# Build for multiple platforms
PLATFORMS=(
    "darwin/amd64:lumina-mac-arm64"
    "darwin/arm64:lumina-mac-arm64"
    "linux/amd64:lumina-linux-x64"
    "linux/arm:lumina-linux-arm"
    "linux/arm64:lumina-linux-arm64"
    "windows/amd64:lumina-windows-x64.exe"
    "freebsd/amd64:lumina-freebsd-x64"
)

echo "📦 Building for multiple platforms..."
for platform in "${PLATFORMS[@]}"; do
    IFS=':' read -r goos_goarch output <<< "$platform"
    IFS='/' read -r goos goarch <<< "$goos_goarch"

    echo "Building for $goos/$goarch -> $output"
    GOOS=$goos GOARCH=$goarch go build -ldflags "-s -w" -o "dist/$output" ./internal

    if [ $? -ne 0 ]; then
        echo "❌ Build failed for $goos/$goarch"
        exit 1
    fi
done

# Create checksums
echo "🔐 Generating checksums..."
cd dist
sha256sum * > checksums.txt
cd ..

# Create release archive
echo "📦 Creating release archive..."
tar -czf "lumina-tui-$VERSION.tar.gz" dist/

# Git operations
if [ -n "$(git status --porcelain)" ]; then
    echo "⚠️  Working directory not clean. Please commit changes first."
    exit 1
fi

echo "🏷️  Tagging release..."
git tag -a "$VERSION" -m "Release $VERSION"

echo "✅ Release $VERSION ready!"
echo "📁 Files created in dist/ directory"
echo "📋 Checksums in dist/checksums.txt"
echo "📦 Archive: lumina-tui-$VERSION.tar.gz"
echo ""
echo "To publish:"
echo "  git push origin $VERSION"
echo "  gh release create $VERSION --title \"$VERSION\" --notes \"Release $VERSION\" lumina-tui-$VERSION.tar.gz"