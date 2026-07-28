# Recuerd0 CLI

Command-line client for [Recuerd0](https://recuerd0.ai) — preserve, version, and organize knowledge from AI conversations.

A product by [maquina](https://maquina.app).

## Installation

**`curl | sh`** (macOS + Linux) — detects your OS/arch, downloads the matching
`recuerd0-<os>-<arch>` release archive, verifies its sha256 against the release
`checksums.txt`, and installs to `/usr/local/bin` (falling back to
`$HOME/.local/bin` if that isn't writable):
```bash
curl -fsSL https://github.com/maquina-app/recuerd0-cli/releases/latest/download/install.sh | sh
```

**macOS (Homebrew)**
```bash
brew install maquina-app/tap/recuerd0
```

**Debian/Ubuntu**
```bash
# Download the .deb for your architecture (amd64 or arm64) from the latest release
curl -LO https://github.com/maquina-app/recuerd0-cli/releases/latest/download/recuerd0_VERSION_linux_amd64.deb
sudo dpkg -i recuerd0_VERSION_linux_amd64.deb
```

**Fedora/RHEL**
```bash
# Download the .rpm for your architecture (amd64 or arm64) from the latest release
curl -LO https://github.com/maquina-app/recuerd0-cli/releases/latest/download/recuerd0_VERSION_linux_amd64.rpm
sudo rpm -i recuerd0_VERSION_linux_amd64.rpm
```

**Windows**

Download `recuerd0-windows-amd64.zip` (or `-arm64`) from [GitHub Releases](https://github.com/maquina-app/recuerd0-cli/releases), unzip it, and add `recuerd0.exe` to your PATH.

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
recuerd0 account add <name> --token TOKEN [--api-url URL] [--skip-verify]
recuerd0 account list
recuerd0 account select <name>
recuerd0 account remove <name>

recuerd0 workspace list [--page N]
recuerd0 workspace show <id>
recuerd0 workspace create --name NAME [--description DESC]
recuerd0 workspace update <id> [--name NAME] [--description DESC]
recuerd0 workspace archive <id>
recuerd0 workspace unarchive <id>
recuerd0 workspace context <id> [--limit N] [--no-body] [--max-body-chars N]
  # Wake-up payload for AI agents: workspace metadata + your pinned
  # memories filtered to this workspace, in one call.

recuerd0 memory list [--workspace ID] [--page N] [--category CAT]
recuerd0 memory show [--workspace ID] <memory_id>
recuerd0 memory create [--workspace ID] [--title T] [--content C | --content -] [--source S] [--tags t1,t2] [--category CAT]
recuerd0 memory update [--workspace ID] <memory_id> [--title T] [--content C] [--source S] [--tags T] [--category CAT]
recuerd0 memory delete [--workspace ID] <memory_id>

recuerd0 memory read head <memory_id> [--workspace ID] [--lines N]
  # First N lines of a memory (default 20).
recuerd0 memory read tail <memory_id> [--workspace ID] [--lines N]
  # Last N lines of a memory (default 20).
recuerd0 memory read lines <memory_id> --start S --end E [--workspace ID]
  # Inclusive 1-based line window of a memory.
recuerd0 memory read grep <memory_id> <pattern> [--workspace ID] [--context N] [--before N] [--after N]
  # Grep memory content; returns matching lines with optional context (0-10).

recuerd0 memory link list <memory_id> [--workspace ID]
recuerd0 memory link add <memory_id> --to <other_id> [--workspace ID]
recuerd0 memory link remove <memory_id> --to <other_id> [--workspace ID]

recuerd0 memory version create [--workspace ID] <memory_id> [--title T] [--content C] [--source S] [--tags T] [--category CAT]

recuerd0 import propose <path> --workspace ID [--plan import.plan.yaml] [--ledger PATH] [--adapter obsidian_markdown|workspace_export] [--fresh]
recuerd0 import commit <plan> [--yes] [--ledger PATH]

recuerd0 search <query> [--workspace ID] [--page N] [--category CAT]
  # Supports FTS5 operators: AND, OR, NOT, "phrases", title:field, body:field
  # Raw FTS5 applies to REST /search.json only; workspace UI & MCP search are phrase-safe (docs/API.md → Search).

recuerd0 version
```

### Bulk imports

Import is propose → review → commit: `propose` writes a reviewable `import.plan.yaml` and never touches the server; `commit` executes the plan you approved.

`import propose` scans an Obsidian/Markdown directory or a Recuerd0 workspace-export v1 JSON file. It writes the plan atomically, uses GET requests only for title-conflict detection, and never creates or changes a memory. Review the digest and YAML together, keep each manifest `action` aligned with all exception `resolution` values for that path, and confirm every `target_memory_id` before approving versions.

```bash
recuerd0 import propose ./vault --workspace 12 --pretty

# Review import.plan.yaml, then commit. An interactive terminal asks once.
recuerd0 import commit import.plan.yaml --pretty

# Agents and other non-interactive callers pass --yes only after human approval.
recuerd0 import commit import.plan.yaml --yes --pretty
```

The default ledger is `import.ledger.jsonl` beside the plan. Keep it: it supplies immutable chain identity and exact resume state after interruption. See [docs/IMPORT.md](docs/IMPORT.md) for plan rules, review guidance, confirmation behavior, and recovery details.

### Categories

Memories can be tagged with one of four categories: `decision`, `discovery`, `preference`, `general`. The `--category` flag is optional on create, update, version create, list, and search — the server defaults new memories to `general` when omitted, and list/search return all categories when omitted.

### Memory Links

Memory links (tunnels) connect two memories that cover related topics across workspace boundaries within the same account. Links are undirected (a link from A to B is the same as B to A), unlabeled, and only allowed between memories owned by the same account. They are useful for expressing that two memories — possibly in different workspaces — discuss related subject matter without duplicating content. The destroy URL takes the *other* memory's id, not a join row id; the CLI hides that detail behind `--to`.

## Agent skills

The CLI bundles its canonical Recuerd0 agent skill, so it can be listed and installed without an account or Recuerd0 configuration:

```bash
recuerd0 skills list

# Install for the current project at ./.claude/skills/recuerd0
recuerd0 skills install recuerd0

# Install for all projects at $HOME/.claude/skills/recuerd0
recuerd0 skills install recuerd0 --global

# Install below a custom skills directory
recuerd0 skills install recuerd0 --target /path/to/skills
```

An existing destination is left untouched. Re-run with `--force` to replace that skill directory with the bundled canonical copy.

Claude Code users can alternatively install the marketplace plugin:

```text
/plugin marketplace add maquina-app/rails-claude-code
/plugin install recuerd0@maquina
```

The source of truth for the bundled and marketplace copies is [`skills/recuerd0/`](skills/recuerd0/). Marketplace updates mirror that directory byte-for-byte after CLI changes merge.

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

`account add` verifies the submitted token and API URL by listing workspaces before saving the account. Use `--skip-verify` only when preparing configuration offline or in an air-gapped environment; it saves the credentials without contacting the API.

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
make build            # Build binary to bin/recuerd0
make test-unit        # Run unit tests
make tidy             # Tidy go modules
make clean            # Remove build artifacts
make release-check    # Validate .goreleaser.yaml
make release-snapshot # Dry-run the full release pipeline (no publish) into dist/
```

## Release

Releases are cut **manually and locally from a Mac** with
[GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`) — there is no release
CI workflow. A release produces the cross-compiled `recuerd0-<os>-<arch>`
archives (darwin/linux/windows × amd64/arm64; `.tar.gz`, `.zip` for windows),
`checksums.txt`, `install.sh`, the `.deb`/`.rpm` packages (nfpm), and the Homebrew
cask (pushed to `maquina-app/homebrew-tap`). macOS binaries are ad-hoc codesigned
during the build so they run on Apple Silicon, so the release must be cut from a Mac.

Validate and dry-run before publishing — the snapshot builds every target and
generates the cask without touching GitHub:
```bash
make test-unit         # unit suite
go fmt ./...           # gofmt may realign struct fields; commit the result
make release-check     # or: goreleaser check
make release-snapshot  # or: goreleaser release --snapshot --clean (no publish)
```

Cut a real release:
```bash
brew install goreleaser   # one-time

# 1. Write and commit the release notes first (see below)
#    docs/releases/v1.2.0.md

# 2. Tag and push — the version is stamped from the tag
git tag v1.2.0 && git push origin v1.2.0

# 3. Build and publish the Release, packages, and Homebrew cask
HOMEBREW_TAP_TOKEN=$(gh auth token) GITHUB_TOKEN=$(gh auth token) goreleaser release --clean

# 4. Attach the notes — GoReleaser publishes an empty release body
gh release edit v1.2.0 --notes-file docs/releases/v1.2.0.md

# 5. Verify: 12 assets, not a draft, body populated
gh release view v1.2.0
```

- `GITHUB_TOKEN` needs `contents:write` on `maquina-app/recuerd0-cli` (publishes the Release here).
- `HOMEBREW_TAP_TOKEN` needs `contents:write` on `maquina-app/homebrew-tap` (pushes the cask cross-repo).
- A `gh` token with `repo` scope covers both, hence `$(gh auth token)` for each. Plain PATs work too.

### Release notes

Notes are hand-written and live in `docs/releases/vX.Y.Z.md`. Changelog generation
is intentionally disabled (`changelog: disable: true`), so **step 4 above is
required** — without it the release ships with an empty body and GoReleaser still
reports success.

Each file follows the same shape (see [`docs/releases/v0.6.0.md`](docs/releases/v0.6.0.md)):

- `# recuerd0 vX.Y.Z`, then `_Released YYYY-MM-DD_`
- `## Features` / `## Fixes`, each entry an `### Imperative headline (#PR)`
- A short paragraph on the problem the change solved, then bullets on what changed
- A closing `## Upgrading` section, even when nothing is required

Draft from merged pull request descriptions rather than commit subjects — they
carry the reasoning:
```bash
gh pr list --state merged --json number,title,body
```

## License

[MIT](LICENSE) - Mario Alberto Chávez Cárdenas
