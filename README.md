# Recuerd0 CLI

Command-line client for [Recuerd0](https://recuerd0.ai) — preserve, version, and organize knowledge from AI conversations.

A product by [maquina](https://maquina.app).

## Installation

**macOS (Homebrew)**
```bash
brew install maquina-app/homebrew-tap/recuerd0-cli
```

**Debian/Ubuntu**
```bash
# Download the .deb for your architecture (amd64 or arm64)
curl -LO https://github.com/maquina-app/recuerd0-cli/releases/latest/download/recuerd0-cli_VERSION_amd64.deb
sudo dpkg -i recuerd0-cli_VERSION_amd64.deb
```

**Fedora/RHEL**
```bash
# Download the .rpm for your architecture (x86_64 or aarch64)
curl -LO https://github.com/maquina-app/recuerd0-cli/releases/latest/download/recuerd0-cli-VERSION-1.x86_64.rpm
sudo rpm -i recuerd0-cli-VERSION-1.x86_64.rpm
```

**Windows**

Download `recuerd0-windows-amd64.exe` from [GitHub Releases](https://github.com/maquina-app/recuerd0-cli/releases), rename it to `recuerd0.exe`, and add it to your PATH.

**With Go**
```bash
go install github.com/maquina/recuerd0-cli/cmd/recuerd0@latest
```

**From binary**

Download the latest release for your platform from [GitHub Releases](https://github.com/maquina-app/recuerd0-cli/releases) and add it to your PATH.

**From source**
```bash
git clone https://github.com/maquina-app/recuerd0-cli.git
cd recuerd0-cli
make build
./bin/recuerd0 --help
```

## Quick Start

```bash
# Add your account
recuerd0 account add personal --token YOUR_API_TOKEN

# List workspaces
recuerd0 workspace list

# Create a memory
recuerd0 memory create --workspace 1 --title "Go patterns" --content "Always wrap errors..."

# Search
recuerd0 search "error handling"
```

## Commands

```
recuerd0 account add <name> --token TOKEN [--api-url URL]
recuerd0 account list
recuerd0 account select <name>
recuerd0 account remove <name>

recuerd0 workspace list [--page N]
recuerd0 workspace show <id>
recuerd0 workspace create --name NAME [--description DESC]
recuerd0 workspace update <id> [--name NAME] [--description DESC]
recuerd0 workspace archive <id>
recuerd0 workspace unarchive <id>

recuerd0 memory list [--workspace ID] [--page N]
recuerd0 memory show [--workspace ID] <memory_id>
recuerd0 memory create [--workspace ID] [--title T] [--content C | --content -] [--source S] [--tags t1,t2]
recuerd0 memory update [--workspace ID] <memory_id> [--title T] [--content C] [--source S] [--tags T]
recuerd0 memory delete [--workspace ID] <memory_id>

recuerd0 memory version create [--workspace ID] <memory_id> [--title T] [--content C] [--source S] [--tags T]

recuerd0 search <query> [--workspace ID] [--page N]
  # Supports FTS5 operators: AND, OR, NOT, "phrases", title:field, body:field

recuerd0 version
```

## Output

All output is structured JSON, designed for AI tool consumption:

```json
{
  "success": true,
  "data": { "id": "1", "title": "Go patterns" },
  "breadcrumbs": [
    { "action": "show", "cmd": "recuerd0 memory show --workspace 1 1", "description": "View memory" }
  ],
  "summary": "Memory created",
  "meta": { "timestamp": "2026-02-06T..." }
}
```

Use `--pretty` for indented output.

## Configuration

### Multi-account support

```bash
recuerd0 account add personal --token tok_abc123
recuerd0 account add work --token tok_xyz789 --api-url https://work.recuerd0.ai
recuerd0 account select work
```

### Per-project config

Create `.recuerd0.yaml` in your project root:

```yaml
account: work
workspace: "5"
```

### Resolution order

1. CLI flags (`--account`, `--token`, `--api-url`, `--workspace`)
2. Environment variables (`RECUERD0_ACCOUNT`, `RECUERD0_TOKEN`, `RECUERD0_API_URL`, `RECUERD0_WORKSPACE`)
3. Local `.recuerd0.yaml` (walked up from current directory)
4. Global `~/.config/recuerd0/config.yaml`

See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for details.

## Development

```bash
make build        # Build binary to bin/recuerd0
make test-unit    # Run unit tests
make tidy         # Tidy go modules
make clean        # Remove build artifacts
```

## License

[MIT](LICENSE) - Mario Alberto Chávez Cárdenas
