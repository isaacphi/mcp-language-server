# Nix Automation

This project includes automated workflows to keep the Nix `vendorHash` up to date.

## 🤖 Automated Workflows

### 1. Update Vendor Hash (`.github/workflows/update-vendor-hash.yml`)

**Triggers:**
- Push to `main` branch with changes to `go.mod` or `go.sum`
- Manual dispatch

**What it does:**
- Calculates the correct `vendorHash` from current Go dependencies
- Compares with the hash in `flake.nix`
- If different, creates a PR with the updated hash
- Verifies the build works with the new hash

### 2. Verify Build (`.github/workflows/verify-build.yml`)

**Triggers:**
- Pull requests affecting `flake.nix`, `go.mod`, `go.sum`, or Go files
- Push to `main` branch

**What it does:**
- Runs `nix flake check`
- Builds the package with `nix build`
- Tests the binary with `nix run`
- Verifies the `vendorHash` is correct

## 🛠️ Local Development

### Update vendorHash manually

```bash
./update-vendor-hash.sh
```

This script:
- ✅ Checks if update is needed
- 🔄 Updates `flake.nix` if hash differs
- 🔨 Verifies build works
- 📝 Shows current vs expected hash

### Quick commands

```bash
# Build the package
nix build --impure

# Run the binary
nix run --impure . -- --help

# Enter development shell
nix develop

# Check flake
nix flake check --impure
```

## 📋 How it works

1. **Hash Calculation**: The correct `vendorHash` is calculated from the Go module dependencies without building
2. **Smart Updates**: Only creates PRs when the hash actually needs updating
3. **Verification**: All changes are verified to build successfully
4. **Automation**: Runs automatically when Go dependencies change

## 🔧 Manual Hash Update

If you need to update the hash manually:

```bash
# Get the current hash
current_hash=$(grep -o 'vendorHash = "sha256-[^"]*"' flake.nix | cut -d'"' -f2)

# Calculate expected hash
drv_path=$(nix eval --impure '.#packages.x86_64-linux.default.goModules.drvPath' | tr -d '"')
expected_hash=$(nix derivation show "$drv_path" | jq -r '.[] | .outputs.out.hash' | xargs -I {} nix hash to-sri "sha256:{}")

# Update flake.nix
sed -i "s/vendorHash = \"sha256-[^\"]*\"/vendorHash = \"$expected_hash\"/" flake.nix
```

## 📈 Benefits

- **Automated**: No manual intervention needed for hash updates
- **Reliable**: Always uses the correct hash calculated from actual dependencies
- **Fast**: Doesn't require failed builds to get the hash
- **Verified**: All updates are tested before merging
- **Transparent**: Clear PR descriptions explain what changed