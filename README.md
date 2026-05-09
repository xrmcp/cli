# xrmcp-cli

A Go CLI for managing and running the xrMCP runtime server.

## Installation

```sh
git clone https://github.com/xrmcp/cli xrmcp-cli
cd xrmcp-cli
go build -o xrmcp .
```

## Commands

### `xrmcp server start`

Start the xrMCP runtime server.

```sh
xrmcp server start [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--transport` | `-t` | `stdio` | Transport mode: `stdio` or `http` |
| `--port` | `-p` | `7373` | Port to listen on |
| `--store` | `-s` | | Path to the tool registry JSON file |
| `--env` | | `.env` | Path to a `.env` file to load (silently skipped if missing) |

Flags take precedence over values in the `.env` file. Supported env vars: `XRMCP_TRANSPORT`, `XRMCP_ADDR`, `XRMCP_STORE_PATH`.

**Examples:**

```sh
# stdio mode (default) — HTTP REST on :7373, MCP over stdin/stdout
xrmcp server start

# HTTP mode — REST + MCP Streamable HTTP on :8080
xrmcp server start -t http -p 8080

# Load custom .env and persist tools
xrmcp server start --env /path/to/.env -s /path/to/tools.json
```

---

### `xrmcp tool ls`

List tools installed on the running server.

```sh
xrmcp tool ls [--url <base-url>]
```

Env var fallback: `XRMCP_SERVER_URL` (default: `http://localhost:7373`).

---

### `xrmcp tool install <manifest>`

Register a tool from a local manifest JSON file.

```sh
xrmcp tool install ./registry/tools/my-tool.xrmcp.json
```

---

### `xrmcp tool uninstall <name>`

Uninstall a tool by name.

```sh
xrmcp tool uninstall my-tool
```

---

## Environment variables

| Variable | Description |
|----------|-------------|
| `XRMCP_TRANSPORT` | `stdio` or `http` |
| `XRMCP_ADDR` | Listen address, e.g. `:7373` |
| `XRMCP_STORE_PATH` | Path to the tool registry JSON file |
| `XRMCP_SERVER_URL` | Base URL for `tool` subcommands (default `http://localhost:7373`) |
