# xrmcp-cli

A Go CLI for managing and running the xrMCP runtime server.

## Installation

### macOS

#### Homebrew

```sh
brew tap xrmcp/homebrew-tap
brew install xrmcp
```

#### Manual install

Download the matching macOS archive from GitHub Releases, extract it, and move `xrmcp` into a directory on your `PATH`.

Verify downloads with `sha256sums.txt`.

### Linux

#### Homebrew

```sh
brew tap xrmcp/homebrew-tap
brew install xrmcp
```

#### Debian/Ubuntu

Download the matching `.deb` package from GitHub Releases and install it:

```sh
sudo dpkg -i xrmcp_<version>_amd64.deb
```

#### Install script

```sh
curl -fsSL https://raw.githubusercontent.com/xrmcp/cli/main/install.sh | sh
```

Pin a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/xrmcp/cli/main/install.sh | sh -s -- --version v0.1.1
```

#### Manual install

Download the matching Linux `.tar.gz`, extract it, and move `xrmcp` into a directory on your `PATH`.

Verify downloads with `sha256sums.txt`.

### Windows

#### Scoop

```powershell
scoop bucket add xrmcp https://github.com/xrmcp/scoop-bucket
scoop install xrmcp
```

#### Manual install

Download the matching Windows `.zip` from GitHub Releases, extract it, and place `xrmcp.exe` on your `PATH`.

Verify downloads with `sha256sums.txt`.

### Build from source

```sh
git clone https://github.com/xrmcp/cli xrmcp-cli
cd xrmcp-cli
go build -o xrmcp .
```

## Commands

### `xrmcp version`

Show the CLI, spec, and Go SDK versions.

```sh
xrmcp version
xrmcp --version
xrmcp -v
```

Example output:

```text
spec: xrmcp.v0.1.0
xrmcp/go-sdk: v0.1.1
xrmcp/cli: v0.1.1
```

---

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

REST management auth is env-driven:

- `XRMCP_API_AUTH_MODE=none|bearer`
- `XRMCP_API_TOKEN=<token>`

If neither is set, runtime startup warns that the REST admin API is running in development mode without auth.

**Examples:**

```sh
# stdio mode (default) — HTTP REST on :7373, MCP over stdin/stdout
xrmcp server start

# HTTP mode — REST + MCP Streamable HTTP on :8080
xrmcp server start -t http -p 8080

# HTTP mode with bearer auth enabled
XRMCP_API_AUTH_MODE=bearer XRMCP_API_TOKEN=my-secret-token xrmcp server start -t http -p 8080

# Load custom .env and persist tools
xrmcp server start --env /path/to/.env -s /path/to/tools.json
```

---

### `xrmcp tool ls`

List tools installed on the running server.

```sh
xrmcp tool ls [--url <base-url>] [--token <token>]
```

Env var fallback:

- `XRMCP_SERVER_URL` for the runtime base URL
- `XRMCP_API_TOKEN` for bearer auth

---

### `xrmcp tool install <manifest>`

Register a tool from a local manifest JSON file or registry identifier.

```sh
xrmcp tool install ./registry/tools/my-tool.xrmcp.json --token my-secret-token
```

You can also install directly from the official registry:

```sh
xrmcp tool install jira/get_jira_ticket
```

The installer supports interactive config prompting based on `configSchema`.

---

### `xrmcp tool search <keyword>`

Search the official registry index without downloading every manifest.

```sh
xrmcp tool search jira
xrmcp tool search project
```

---

### `xrmcp tool uninstall <name>`

Uninstall a tool by name.

```sh
xrmcp tool uninstall my-tool --token my-secret-token
```

---

### `xrmcp manifest generate`

Generate xrMCP `ToolRegistration` manifest files from external sources.

Current supported source:

- Postman collection JSON

```sh
xrmcp manifest generate --from postman --in ./collection.json --out ./generated-tools
```

Short flags:

```sh
xrmcp manifest generate -f postman -i ./collection.json -o ./generated-tools
```

> [!NOTE]
> Postman import is still experimental.
>
> Review each generated manifest carefully before installing it or using it in production. Postman collections often contain environment-specific assumptions, auth shortcuts, sample literals, or request patterns that cannot always be converted perfectly.
>
> Bulk install from a generated folder is intentionally not supported yet. The expected flow is to inspect each generated `.xrmcp.json` file first, then install only the manifests you approve.



---

### `xrmcp manifest new`

Create a minimal xrMCP manifest scaffold.

```sh
xrmcp manifest new ./my-tool
xrmcp manifest new ./tools/jira/get_jira_ticket.xrmcp.json
```


---

### `xrmcp manifest validate`

Validate a local xrMCP manifest file.

```sh
xrmcp manifest validate ./tools/jira/get_jira_ticket.xrmcp.json
```

The command validates the local file against the current xrMCP manifest schema and prints either a short success message or readable validation errors.

---

## Environment variables

| Variable | Description |
|----------|-------------|
| `XRMCP_TRANSPORT` | `stdio` or `http` |
| `XRMCP_ADDR` | Listen address, e.g. `:7373` |
| `XRMCP_STORE_PATH` | Path to the tool registry JSON file |
| `XRMCP_SERVER_URL` | Base URL for `tool` subcommands (default `http://localhost:7373`) |
| `XRMCP_API_AUTH_MODE` | REST admin auth mode: `none` or `bearer` |
| `XRMCP_API_TOKEN` | Bearer token for protected runtime admin endpoints and CLI tool commands |

## Release automation

Release packaging is driven by:

- `.goreleaser.yml`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`

Normal CI:

- runs `go test ./...`
- validates the GoReleaser configuration with `goreleaser check`

Tagged releases:

- trigger on tags like `v0.1.0`
- run tests
- build the supported OS/arch matrix
- package release archives
- generate `sha256sums.txt`
- generate Linux `.deb` packages
- publish artifacts to GitHub Releases

## Maintainer notes

Homebrew and Scoop publishing require external repositories.

Publishing repos:

- `xrmcp/homebrew-tap`
- `xrmcp/scoop-bucket`



Current behavior:

- release archives, checksums, and `.deb` packages are produced and published by GoReleaser
- Homebrew and Scoop definitions are generated by GoReleaser
- Homebrew and Scoop publication are handled through separate tap and bucket repositories
