# MCP Language Server

[![Go Tests](https://github.com/isaacphi/mcp-language-server/actions/workflows/go.yml/badge.svg)](https://github.com/isaacphi/mcp-language-server/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/isaacphi/mcp-language-server)](https://goreportcard.com/report/github.com/isaacphi/mcp-language-server)
[![GoDoc](https://pkg.go.dev/badge/github.com/isaacphi/mcp-language-server)](https://pkg.go.dev/github.com/isaacphi/mcp-language-server)
[![Go Version](https://img.shields.io/github/go-mod/go-version/isaacphi/mcp-language-server)](https://github.com/isaacphi/mcp-language-server/blob/main/go.mod)

This is an [MCP](https://modelcontextprotocol.io/introduction) server that runs and exposes a [language server](https://microsoft.github.io/language-server-protocol/) to LLMs. Not a language server for MCP, whatever that would be.

## Demo

- Run it on this repo.
- Find definition
- Edit function signature and update all references
- Rename function
- Check diagnostics

## Setup

1. **Install Go**: Follow instructions at <https://golang.org/doc/install>
2. **Fetch or update this server**: `go install github.com/isaacphi/mcp-language-server@latest`
3. **Install a language server**: _follow one of the guides below_
4. **Configure your MCP client**: _follow one of the guides below_

<details>
  <summary>Go (gopls)</summary>
**Install gopls**: `go install golang.org/x/tools/gopls@latest`
**Configure your MCP client**: This will be different but similar for each client. For Claude Desktop, add the following to `~/Library/Application\ Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": ["--workspace", "/Users/you/dev/yourproject/", "--lsp", "gopls"],
      "env": {
        "PATH": "/opt/homebrew/bin:/Users/you/go/bin",
        "GOPATH": "/users/you/go",
        "GOCACHE": "/users/you/Library/Caches/go-build",
        "GOMODCACHE": "/Users/you/go/pkg/mod"
      }
    }
  }
}
```

**Note**: Not all clients will need these environment variables. For Claude Desktop you will need to update the environment variables above based on your machine and username:

- `PATH` needs to contain the path to `go` and to `gopls`. Get this with `echo $(which go):$(which gopls)`
- `GOPATH`, `GOCACHE`, and `GOMODCACHE` may be different on your machine. These are the defaults.

</details>
<details>
  <summary>Rust (rust-analyzer)</summary>
**Install rust-analyzer**: `rustup component add rust-analyzer`
**Configure your MCP client**: This will be different but similar for each client. For Claude Desktop, add the following to `~/Library/Application\ Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "rust-analyzer"
      ]
    }
  }
}
```

</details>
<details>
  <summary>Python (pyright)</summary>
**Install pyright**: `npm install -g pyright`
**Configure your MCP client**: This will be different but similar for each client. For Claude Desktop, add the following to `~/Library/Application\ Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "pyright",
        "--",
        "--stdio"
      ]
    }
  }
}
```

</details>
<details>
  <summary>Typescript (typescript-language-server)</summary>
**Install typescript-language-server**: `npm install -g typescript typescript-language-server`
**Configure your MCP client**: This will be different but similar for each client. For Claude Desktop, add the following to `~/Library/Application\ Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "typescript-language-server",
        "--",
        "--stdio"
      ]
    }
  }
}
```

</details>
<details>
  <summary>Other</summary>
I have only tested this repo with the servers above but it should be compatible with many more. Note:
- The language server must communicate over stdio.
- Any aruments after `--` are sent as arguments to the language server.
- Any env variables are passed on to the language server.
</details>

## Tools

- `read_definition`: Retrieves the complete source code definition of any symbol (function, type, constant, etc.) from your codebase.
- `find_references`: Locates all usages and references of a symbol throughout the codebase.
- `get_diagnostics`: Provides diagnostic information for a specific file, including warnings and errors.
- `get_codelens`: Retrieves code lens hints for additional context and actions on your code.
- `execute_codelens`: Runs a code lens action.
- `hover`: Display documentation, type hints, or other hover information for a given location.
- `rename_symbol`: Rename a symbol across a project.
- `apply_text_edit`: Allows making multiple text edits to a file programmatically.

Behind the scenes, this MCP server can act on `workspace/applyEdit` requests from the language server.

## About

This codebase makes use of edited code from [gopls](https://go.googlesource.com/tools/+/refs/heads/master/gopls/internal/protocol) to handle LSP communication. See ATTRIBUTION for details. Everything here is covered by a permissive BSD style license.

[mcp-go](https://github.com/mark3labs/mcp-go) is used for MCP communication. Thank you for your service.

This is beta software. Please let me know by creating an issue if you run into any problems or have suggestions of any kind.

## Contributing

Please keep PRs small and open Issues first for anything substantial. AI slop OK as long as it is tested, passes checks, and doesn't smell too bad.

### Setup

Clone the repo:

```bash
git clone https://github.com/isaacphi/mcp-language-server.git
cd mcp-language-server
```

A [justfile](https://just.systems/man/en/) is included for convenience:

```bash
just -l
Available recipes:
    build    # Build
    check    # Run code audit checks
    fmt      # Format code
    generate # Generate LSP types and methods
    help     # Help
    install  # Install locally
    snapshot # Update snapshot tests
    test     # Run tests
```

Configure your Claude Desktop (or similar) to use the local binary:

```json
{
  "mcpServers": {
    "language-server": {
      "command": "/full/path/to/your/clone/mcp-language-server/mcp-language-server",
      "args": [
        "--workspace",
        "/path/to/workspace",
        "--lsp",
        "language-server-executable"
      ],
      "env": {
        "LOG_LEVEL": "DEBUG"
      }
    }
  }
}
```

Rebuild after making changes.

### Logging

Setting the `LOG_LEVEL` environment variable to DEBUG enables verbose logging for all components including messages to and from the language server and the language server's logs.

### LSP interaction

- `internal/lsp/methods.go` contains generated code to make calls to the connected language server
- `internal/protocol/tsprotocol.go` contains generated code for LSP types. I borrowed this from `gopls`'s source code. Thank you for your service.
- LSP allows language servers to return different types for the same methods. Go doesn't like this so there are some ugly workarounds in `internal/protocol/interfaces.go`.

### Local Development and Snapshot Tests

There is a snapshot test suite that makes it a lot easier to try out changes to tools. These run actual language servers on mock workspaces and capture output and logs.

You will need the language servers installed locally to run them. There are tests for go, rust, python, and typescript.

```
integrationtests/
├── tests/        # Tests are in this folder
├── snapshots/    # Snapshots of tool outputs
├── test-output/  # Gitignored folder showing the final state of each workspace and logs after each test run
└── workspaces/   # Mock workspaces that the tools run on
```

To update snapshots, run `UPDATE_SNAPSHOTS=true go test ./integrationtests/...`
