#!/bin/bash

# Extract version from version.go
VERSION=$(grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' version.go)

if [ -z "$VERSION" ]; then
    echo "❌ Could not find version in version.go"
    exit 1
fi

echo "🚀 Releasing $VERSION..."
rm -rf dist && mkdir dist

echo "📦 Building..."
go build -o dist/lumina-mac .
GOOS=linux GOARCH=amd64 go build -o dist/lumina-linux .
GOOS=windows GOARCH=amd64 go build -o dist/lumina.exe .

git tag -a "$VERSION" -m "Release $VERSION"
echo "✅ Done. Tagged $VERSION locally."