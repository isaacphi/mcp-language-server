# Multi-LSP Mode

The MCP Language Server supports running multiple language servers simultaneously in a single session.

## Modes of Operation

### Legacy Mode (Single LSP)

Run with `--workspace` and `--lsp` flags:

```bash
mcp-language-server --workspace /path/to/project --lsp gopls
```

### Config Mode (Multi-LSP)

Run with `--config` flag:

```bash
mcp-language-server --config config.json
```

### Session Mode

Run with `--config` and `--session` flags to auto-load a session on startup:

```bash
mcp-language-server --config config.json --session my-session.json
```

In session mode, LSP management tools (`lsp_start`, `lsp_stop`, `lsp_save`, `lsp_load`) are not registered. Edit the session file and restart the server to change LSPs.

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

- **`lsp_start(workspace, language)`** - Start a new LSP instance
- **`lsp_stop(id)`** - Stop a running LSP by ID
- **`lsp_list()`** - List all running LSP instances
- **`lsp_select(id)`** - Set a different LSP as default
- **`lsp_languages()`** - List available languages from config (config mode only)
- **`lsp_save(filepath)`** - Save current LSP configuration to a session file (config mode only)
- **`lsp_load(filepath)`** - Load LSP configuration from a session file (config mode only)

### Language Tools

All existing tools accept an optional `id` parameter to target a specific LSP:

- `definition(symbolName, id?)`
- `references(symbolName, id?)`
- `diagnostics(filePath, id?)`
- `hover(filePath, line, column, id?)`
- `rename_symbol(filePath, line, column, newName, id?)`
- `edit_file(filePath, edits, id?)`

If `id` is omitted in config mode, the last started LSP is used.

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

Start server in config mode:
```bash
mcp-language-server --config config.json
```

Start LSPs:
```
lsp_start(workspace="/project/backend", language="go")
lsp_start(workspace="/project/frontend", language="typescript")
```

Use tools:
```
# Use Go LSP (explicitly)
definition(symbolName="MyFunc", id="550e8400...")

# Use last started LSP (TypeScript in this case)
definition(symbolName="myFunc")
```

Save session:
```
lsp_save(filepath="/home/user/.lsp-sessions/fullstack.json")
```

Later, restore:
```
lsp_load(filepath="/home/user/.lsp-sessions/fullstack.json")
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

### Claude Desktop (Config Mode)

```json
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": ["--config", "/path/to/config.json"],
      "env": {
        "LOG_LEVEL": "INFO"
      }
    }
  }
}
```

### Claude Desktop (Legacy Mode)

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

- **No LSP Started**: Call `lsp_start` before using language tools
- **Invalid Language**: Language must be defined in config file
- **Invalid ID**: LSP instance not found by the given ID
- **Session Load Error**: Check workspace paths exist and language names match config

## Notes

- Each LSP instance is isolated with its own workspace
- LSPs are cleaned up on shutdown
- The last started LSP is used as default when no ID specified
- Environment variables in config are merged with system environment
- All LSP communication happens over stdio
