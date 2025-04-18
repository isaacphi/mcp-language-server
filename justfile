# Help
help:
  just -l

# Build
build:
  go build -o mcp-language-server

# Install locally
install:
  go install

# Format code
fmt:
  gofmt -w .

# Generate LSP types and methods
generate:
  go generate ./...

# Run code audit checks
check:
  gofmt -l .
  test -z "$(gofmt -l .)"
  go tool staticcheck ./...
  go tool govulncheck ./...
  go tool errcheck ./...
  find . -path "./integrationtests/workspaces" -prune -o \
    -path "./integrationtests/test-output" -prune -o \
    -name "*.go" -print | xargs gopls check

# Run tests
test:
  go test ./...

# Update snapshot tests
snapshot:
  UPDATE_SNAPSHOTS=true go test ./integrationtests/...
