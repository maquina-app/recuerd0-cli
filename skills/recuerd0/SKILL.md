# Recuerd0 CLI Skill

## Description

Recuerd0 CLI (`recuerd0`) is a command-line tool for preserving, versioning, and organizing knowledge from AI conversations. Use it to manage workspaces and memories in the Recuerd0 platform.

## Output Format

All commands output structured JSON:

```json
{
  "success": true,
  "data": { ... },
  "pagination": { "has_next": true, "next_url": "..." },
  "breadcrumbs": [
    { "action": "show", "cmd": "recuerd0 memory show --workspace 1 42", "description": "View memory" }
  ],
  "summary": "5 memory(ies)",
  "meta": { "timestamp": "2026-02-06T..." }
}
```

Errors:
```json
{
  "success": false,
  "error": { "code": "NOT_FOUND", "message": "...", "status": 404 }
}
```

## Commands

### Account Management

```bash
recuerd0 account add <name> --token TOKEN [--api-url URL]
recuerd0 account list
recuerd0 account select <name>
recuerd0 account remove <name>
```

### Workspaces

```bash
recuerd0 workspace list [--page N]
recuerd0 workspace show <id>
recuerd0 workspace create --name NAME [--description DESC]
recuerd0 workspace update <id> [--name NAME] [--description DESC]
recuerd0 workspace archive <id>
recuerd0 workspace unarchive <id>
```

### Memories

```bash
recuerd0 memory list [--workspace ID] [--page N]
recuerd0 memory show [--workspace ID] <memory_id>
recuerd0 memory create [--workspace ID] [--title TITLE] [--content CONTENT | --content -] [--source SRC] [--tags tag1,tag2]
recuerd0 memory update [--workspace ID] <memory_id> [--title T] [--content C] [--source S] [--tags T]
recuerd0 memory delete [--workspace ID] <memory_id>
```

- `--workspace` falls back to the workspace in `.recuerd0.yaml` or `RECUERD0_WORKSPACE`
- `--content -` reads content from stdin

### Memory Versions

```bash
recuerd0 memory version create [--workspace ID] <memory_id> [--title T] [--content C] [--source S] [--tags T]
```

### Search

```bash
recuerd0 search <query> [--workspace ID] [--page N]
```

Search is backed by SQLite FTS5 and supports operators:

```bash
# Prefix matching
recuerd0 search "auth*"

# AND — both terms required
recuerd0 search "rails AND caching"

# OR — either term
recuerd0 search "postgres OR sqlite"

# NOT — exclude terms
recuerd0 search "deploy NOT heroku"

# Phrases
recuerd0 search '"error handling"'
```

### Version

```bash
recuerd0 version
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--account` | Account name to use |
| `--token` | API token override |
| `--api-url` | API URL override |
| `--workspace` | Workspace ID override |
| `--verbose` | Show HTTP request/response details |
| `--pretty` | Pretty-print JSON output |

## Breadcrumbs

Every response includes `breadcrumbs` — suggested next actions as CLI commands. Use these to discover workflows:

```json
"breadcrumbs": [
  { "action": "show", "cmd": "recuerd0 workspace show 1", "description": "View workspace details" },
  { "action": "create", "cmd": "recuerd0 memory create --workspace 1 --title TITLE", "description": "Create a memory" }
]
```

## Usage Patterns

### Store a memory from AI conversation
```bash
recuerd0 memory create --workspace 1 --title "Go error handling" --content "Always wrap errors with context..." --tags "go,patterns"
```

### Pipe content from stdin
```bash
cat notes.md | recuerd0 memory create --workspace 1 --title "Session Notes" --content -
```

### Search and retrieve
```bash
recuerd0 search "error handling" --workspace 1
recuerd0 memory show --workspace 1 42
```

### Version a memory
```bash
recuerd0 memory version create --workspace 1 42 --title "Updated patterns" --content "Revised content..."
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Authentication failure |
| 4 | Forbidden |
| 5 | Not found |
| 6 | Validation error |
| 7 | Network error |
| 8 | Rate limited |
