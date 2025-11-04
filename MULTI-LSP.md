# Multi-LSP Mode

The MCP Language Server supports running multiple language servers simultaneously in a single session.

## Modes of Operation

| Mode | Command | Use Case | Runtime Changes |
|------|---------|----------|-----------------|
| **Single-MCP** | `--workspace <path> --lsp <cmd>` | Single language server for one workspace | Not supported |
| **Unbounded** | `--config <file>` or auto-detected | Multiple LSPs, manage dynamically | All tools available |
| **Session** | `--config <file> --session <file>` or auto-detected config | Multiple LSPs, fixed configuration | Only `lsp_list` and `lsp_select` |

### Single-MCP Mode

Run with `--workspace` and `--lsp` flags:

```bash
mcp-language-server --workspace /path/to/project --lsp gopls
```

Single-MCP mode runs a single language server. LSP management tools are not available.

### Unbounded Mode

Run with `--config` flag:

```bash
mcp-language-server --config config.json
```

Unbounded mode allows you to dynamically start and stop language servers. All LSP management tools are available.

### Session Mode

Run with `--config` and `--session` flags to auto-load a session on startup:

```bash
mcp-language-server --config config.json --session my-session.json
```

Session mode loads a predefined set of LSPs. LSP management tools (`lsp_start`, `lsp_stop`, `lsp_save`, `lsp_load`) are not registered. Only `lsp_list` and `lsp_select` are available. Edit the session file and restart the server to add/remove LSPs.

## Auto-Detection of Config File

The server automatically looks for a config file in standard locations (in order):

1. `~/.mcp-language-server.json`
2. `~/.config/mcp-language-server.json`

This is used when:
- No parameters provided: `mcp-language-server`
- Only `--session` provided: `mcp-language-server --session my-session.json`

If no config file is found in these cases, an error is raised. To use a custom path, explicitly specify `--config`.

## Configuration File

JSON file defining LSP configurations by language:

```json
{
  "lsps": {
    "go": {
      "command": "gopls",
      "args": [],
      "env": {
        "GOPATH": "/home/user/go"
      }
    },
    "python": {
      "command": "pyright-langserver",
      "args": ["--", "--stdio"]
    },
    "rust": {
      "command": "rust-analyzer",
      "args": []
    }
  }
}
```

### Fields

- **`command`** (required): LSP executable name or full path
- **`args`** (optional): Command-line arguments passed to the LSP
- **`env`** (optional): Environment variables (merged with system environment)

## Tools

### LSP Management

- **`lsp_start(workspace, language)`** - Start a new LSP instance (unbounded mode only)
- **`lsp_stop(id)`** - Stop a running LSP by ID (unbounded mode only)
- **`lsp_list()`** - List all running LSP instances with their selection status
- **`lsp_select(id)`** - Set a different LSP as default
- **`lsp_languages()`** - List available languages from config (unbounded mode only)
- **`lsp_save(filepath)`** - Save current LSP configuration to a session file (unbounded mode only)
- **`lsp_load(filepath)`** - Load LSP configuration from a session file (unbounded mode only)

### Language Tools

All existing tools accept an optional `id` parameter to target a specific LSP:

- `definition(symbolName, id?)`
- `references(symbolName, id?)`
- `diagnostics(filePath, id?)`
- `hover(filePath, line, column, id?)`
- `rename_symbol(filePath, line, column, newName, id?)`
- `edit_file(filePath, edits, id?)`

In unbounded mode with multiple LSPs, either:
- Call `lsp_select(id)` to set the default, then omit `id` parameter in tools, OR
- Provide `id` parameter in every tool call to specify which LSP to use

## Session File Format

Save and restore LSP configurations:

```json
{
  "lsps": [
    {
      "workspace": "/path/to/workspace1",
      "language": "go"
    },
    {
      "workspace": "/path/to/workspace2",
      "language": "typescript"
    }
  ]
}
```

## Usage Example

### Auto-detected Unbounded Mode
```bash
# Uses ~/.mcp-language-server.json or ~/.config/mcp-language-server.json
mcp-language-server
```

Then dynamically start LSPs:
```
lsp_start(workspace="/project/backend", language="go")
lsp_start(workspace="/project/frontend", language="typescript")
lsp_select(id="<typescript-lsp-id>")  # Select an LSP
definition(symbolName="MyFunc")  # Uses selected LSP (TypeScript)
definition(symbolName="MyFunc", id="<go-lsp-id>")  # Use specific LSP
```

Save the current setup:
```
lsp_save(filepath="~/.config/my-session.json")
```

### Session Mode (Auto-detected config)
```bash
mcp-language-server --session ~/.config/my-session.json
```

Restore a saved session:
```
lsp_load(filepath="~/.config/my-session.json")
lsp_select(id="<lsp-id>")  # Switch to a different LSP
```

### Single-MCP Mode
```bash
mcp-language-server --workspace /path/to/project --lsp gopls
```

## Configuration Examples

Edit `config.json` with your system paths. For Go:

```bash
which gopls
go env GOPATH
go env GOCACHE
```

For clangd, verify installation:

```bash
which clangd
```

For Node.js-based LSPs:

```bash
which pyright-langserver
which typescript-language-server
```

## MCP Client Configuration

### Claude Desktop (Auto-detected Unbounded Mode)

```json
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server"
    }
  }
}
```

### Claude Desktop (Explicit Unbounded Mode)

```json
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": ["--config", "/path/to/config.json"]
    }
  }
}
```

### Claude Desktop (Session Mode)

```json
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": ["--session", "~/.config/my-session.json"]
    }
  }
}
```

### Claude Desktop (Single-MCP Mode)

```json
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": ["--workspace", "/path/to/project", "--lsp", "gopls"],
      "env": {
        "GOPATH": "/home/user/go"
      }
    }
  }
}
```

## Error Handling

- **No config file found**: Create `~/.mcp-language-server.json` or use `--config` flag
- **No LSP Started**: Call `lsp_start` before using language tools (unbounded mode)
- **Invalid Language**: Language must be defined in config file
- **Invalid ID**: LSP instance not found or no longer running
- **Session Load Error**: Check workspace paths exist and language names match config

## Notes

- Each LSP instance isolated with its own workspace
- If only one LSP is running, it is automatically selected; if multiple LSPs are running, explicit `lsp_select(id)` or `id` parameter in tools is required
- `lsp_list` shows all running instances (id, language, workspace, status, selected)
- When the selected LSP is stopped, no LSP is selected (must call `lsp_select` again)
- In session mode with multiple LSPs, none are auto-selected on startup (call `lsp_select` to choose)
- In session mode, `lsp_select` switches between pre-loaded LSPs
- Environment variables in config are merged with system environment
