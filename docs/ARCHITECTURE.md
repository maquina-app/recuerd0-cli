# Architecture

## Project Structure

```
recuerd0-cli/
├── cmd/recuerd0/main.go           # Entry point, version injection
├── internal/
│   ├── client/                    # HTTP API client
│   │   ├── interface.go           # API interface (for mocking)
│   │   ├── client.go              # HTTP implementation
│   │   └── client_test.go
│   ├── commands/                  # Cobra command definitions
│   │   ├── root.go                # Root command, config loading, test infra
│   │   ├── mock_client.go         # Mock client for unit tests
│   │   ├── version.go             # version command
│   │   ├── account.go             # account add|list|select|remove
│   │   ├── workspace.go           # workspace list|show|create|update
│   │   ├── workspace_archive.go   # workspace archive|unarchive
│   │   ├── memory.go              # memory list|show|create|update|delete
│   │   ├── version_memory.go      # memory version create
│   │   ├── search.go              # search command
│   │   └── *_test.go              # Unit tests
│   ├── config/                    # Multi-account configuration
│   │   ├── config.go              # Config loading, saving, resolution
│   │   └── config_test.go
│   ├── errors/                    # Typed error system
│   │   ├── errors.go              # CLIError, constructors, exit codes
│   │   └── errors_test.go
│   └── response/                  # JSON response envelope
│       ├── response.go            # Response struct, builders, printing
│       └── response_test.go
├── skills/recuerd0/SKILL.md       # AI skill definition
├── docs/                          # Documentation
├── .github/workflows/             # CI/CD
└── Makefile
```

## Package Responsibilities

### `internal/errors`
Typed error system with HTTP-to-exit-code mapping. Every CLI error carries a machine-readable code, human-readable message, optional HTTP status, and process exit code. `FromHTTPStatus()` converts API errors to typed CLIErrors.

### `internal/response`
JSON envelope for all output. Every command produces a `Response` with `success`, `data`, optional `error`, `pagination`, `breadcrumbs`, `summary`, and `meta`. The `--pretty` flag controls indentation.

### `internal/config`
Multi-account configuration with cascading resolution. Global config at `~/.config/recuerd0/config.yaml` stores named accounts. Local `.recuerd0.yaml` provides per-project overrides. Resolution order: CLI flags > env vars > local config > global config.

### `internal/client`
HTTP client implementing the `API` interface. Handles auth headers, JSON serialization, Link header pagination, error extraction, and verbose logging. The interface enables mock-based testing.

### `internal/commands`
Cobra command tree. `root.go` sets up the root command, global flags, `PersistentPreRun` for config resolution, and test infrastructure. Each command file follows the pattern: validate → call client → format response with breadcrumbs.

## Data Flow

```
User Input → Cobra → PersistentPreRun (config resolution)
                   → Command Run
                       → requireAuth() / requireWorkspace()
                       → getClient() → API interface
                       → client.Get/Post/Patch/Delete
                       → exitWithError() or printSuccess*()
                       → Response.Print() → JSON to stdout
```

## Design Decisions

**Why Cobra?** Industry standard for Go CLIs. Provides argument validation, help generation, shell completions, and nested subcommands.

**Why JSON-only output?** AI-first design. Structured output with breadcrumbs enables AI agents to discover workflows, parse results, and chain commands. No table formatting or human-only output.

**Why interface-driven client?** Testability. The `API` interface enables mock-based unit tests without HTTP servers. Commands never construct clients directly — they use `getClient()` which is overridden in test mode.

**Why cascading config?** Supports multiple workflows: global account management for personal use, per-project overrides for team/workspace contexts, environment variables for CI/CD, and flags for one-off commands.

**Why typed errors?** Consistent error reporting. Every error maps to a JSON error response and a specific exit code. AI tools can programmatically handle errors by inspecting the `code` field.
