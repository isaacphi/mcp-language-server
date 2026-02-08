# Summary
This project (mcp-language-server) is an MCP server that exposes language server protocol to AI agents. It helps MCP enabled clients (agents) navigate codebases more easily by giving them access semantic tools like get definition, references, rename, and diagnostics.

# Build
go build -o mcp-language-server

# Install locally
go install

# Format code
gofmt -w .

# Generate LSP types and methods
go run ./cmd/generate

# Run code audit checks
  gofmt -l .
  test -z "$(gofmt -l .)"
  go tool staticcheck ./...
  go tool errcheck ./...
  find . -path "./integrationtests/workspaces" -prune -o \
    -path "./integrationtests/test-output" -prune -o \
    -name "*.go" -print | xargs gopls check
  go tool govulncheck ./...

# Run tests
go test ./...

# Update snapshot tests
UPDATE_SNAPSHOTS=true go test ./integrationtests/...
