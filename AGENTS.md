# Summary
This project (mcp-language-server) is an MCP server that exposes language server protocol to AI agents. It helps MCP enabled clients (agents) navigate codebases more easily by giving them access semantic tools like get definition, references, rename, and diagnostics.

The project is mature and almost feature-complete, but we will be making some modifications to it.

We will use Beads (bd) for issue tracking.

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

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
