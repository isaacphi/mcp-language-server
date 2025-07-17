#!/usr/bin/env bash

# Script to update vendorHash in flake.nix
set -e

echo "🔍 Checking vendorHash..."

# Get current hash from flake.nix
current_hash=$(grep -o 'vendorHash = "sha256-[^"]*"' flake.nix | cut -d'"' -f2)

if [ -z "$current_hash" ]; then
    echo "❌ Error: Could not find vendorHash in flake.nix"
    exit 1
fi

echo "Current vendorHash: $current_hash"

# Get the goModules derivation path
drv_path=$(nix eval --impure '.#packages.x86_64-linux.default.goModules.drvPath' 2>/dev/null | tr -d '"')

if [ -z "$drv_path" ]; then
    echo "❌ Error: Could not get goModules derivation path"
    exit 1
fi

# Extract the hash from the derivation
hash=$(nix derivation show "$drv_path" | jq -r '.[] | .outputs.out.hash')

if [ -z "$hash" ] || [ "$hash" = "null" ]; then
    echo "❌ Error: Could not extract hash from derivation"
    exit 1
fi

# Convert to SRI format
expected_hash=$(nix hash to-sri "sha256:$hash")

echo "Expected vendorHash: $expected_hash"

# Check if update is needed
if [ "$current_hash" = "$expected_hash" ]; then
    echo "✅ vendorHash is already up to date!"
    exit 0
fi

echo "🔄 Updating vendorHash..."

# Update flake.nix
sed -i "s/vendorHash = \"sha256-[^\"]*\"/vendorHash = \"$expected_hash\"/" flake.nix

echo "✅ Updated flake.nix with new vendorHash"

# Verify the build works
echo "🔨 Verifying build..."
if nix build --impure; then
    echo "✅ Build successful!"
else
    echo "❌ Build failed with new hash"
    exit 1
fi

echo "🎉 Done!"