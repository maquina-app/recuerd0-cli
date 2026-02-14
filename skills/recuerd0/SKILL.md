---
name: recuerd0
description: Manages workspaces and memories in the Recuerd0 platform. Use when user asks to save, search, version, or organize knowledge using recuerd0. Also use proactively to search for project context before starting complex tasks.
---

# recuerd0

Persistent, searchable memory for AI coding agents. Query context on demand instead of cramming everything into project files.

## Output format

JSON envelope with `success`, `data`, `breadcrumbs`, `pagination`, `summary`, `meta`.

## CLI Reference

```bash
recuerd0 --help                    # All commands and global flags
recuerd0 <command> --help          # Command-specific help
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--account NAME` | Account to use (from config) |
| `--workspace ID` | Workspace ID (overrides config) |
| `--pretty` | Pretty-print JSON output |
| `--verbose` | Show HTTP request/response details |
| `--token TOKEN` | API token (overrides config) |
| `--api-url URL` | API base URL (overrides config) |

### Workspaces

```bash
recuerd0 workspace list [--page N]
recuerd0 workspace show <id>
recuerd0 workspace create --name "Name" [--description "Desc"]
recuerd0 workspace update <id> --name "Name" [--description "Desc"]
recuerd0 workspace archive <id>
recuerd0 workspace unarchive <id>
```

### Memories

```bash
recuerd0 memory list --workspace <ws_id> [--page N]
recuerd0 memory show --workspace <ws_id> <memory_id>
recuerd0 memory create --workspace <ws_id> --title "Title" --content "Body" [--tags "a,b"] [--source "src"]
recuerd0 memory update --workspace <ws_id> <memory_id> [--title "T"] [--content "C"] [--tags "a,b"]
recuerd0 memory delete --workspace <ws_id> <memory_id>
```

Content can be read from stdin with `--content -`.

### Memory Versions

```bash
recuerd0 memory version create <memory_id> --workspace <ws_id> \
  [--title "T"] [--content "C"] [--tags "a,b"] [--source "src"]
```

Creates a new version of a memory. Fields default to the parent version's values if omitted.

### Search

```bash
recuerd0 search "<query>" [--workspace <ws_id>] [--page N]
```

Supports FTS5 query operators:

| Operator | Example | Description |
|----------|---------|-------------|
| Term | `architecture` | Substring match |
| AND | `architecture AND design` | Both terms must appear |
| OR | `meeting OR standup` | Either term can appear |
| NOT | `design NOT draft` | Exclude term |
| Phrase | `"project timeline"` | Exact phrase match |
| Column | `title:architecture` | Search only title field |
| Column | `body:implementation` | Search only body field |
| Group | `(meeting OR standup) AND notes` | Parentheses for precedence |

### Accounts

```bash
recuerd0 account list
recuerd0 account add <name> --token TOKEN --api-url URL
recuerd0 account remove <name>
recuerd0 account switch <name>
```

## Config

Config cascade (highest priority wins): CLI flags > env vars > local `.recuerd0.yaml` > global `~/.config/recuerd0/config.yaml`

A `.recuerd0.yaml` in the project root auto-selects account and workspace:

```yaml
account: work
workspace: 22
```

## API Routes

| Method | Path | CLI Command |
|--------|------|-------------|
| GET | `/workspaces` | `workspace list` |
| GET | `/workspaces/:id` | `workspace show` |
| POST | `/workspaces` | `workspace create` |
| PATCH | `/workspaces/:id` | `workspace update` |
| POST | `/workspaces/:id/archive` | `workspace archive` |
| DELETE | `/workspaces/:id/archive` | `workspace unarchive` |
| GET | `/workspaces/:ws/memories` | `memory list` |
| GET | `/workspaces/:ws/memories/:id` | `memory show` |
| POST | `/workspaces/:ws/memories` | `memory create` |
| PATCH | `/workspaces/:ws/memories/:id` | `memory update` |
| DELETE | `/workspaces/:ws/memories/:id` | `memory delete` |
| POST | `/workspaces/:ws/memories/:id/versions` | `memory version create` |
| GET | `/search?q=<query>` | `search` |

## Instructions

1. **Use the recuerd0 CLI directly** via the Bash tool — do not use curl or raw HTTP
2. **Always use `--pretty`** for readable output when presenting to the user
3. **Parse JSON output** and present results in a readable format with relevant IDs
4. **Search before creating** to avoid duplicate memories
5. **Use `--workspace`** flag or ensure `.recuerd0.yaml` exists in the project root
6. **For large content**, write to a temp file and pipe via stdin: `cat file.md | recuerd0 memory create --workspace <id> --content -`
7. **Deleting a memory deletes all its versions** — there is no way to delete a single version

## Workflows

### Pre-session context loading

Before starting a complex task, search recuerd0 for relevant project knowledge:

```bash
recuerd0 search "authentication" --pretty
recuerd0 search "database schema" --workspace 22 --pretty
```

### Capture knowledge during a session

Save discoveries, patterns, and decisions as memories:

```bash
recuerd0 memory create --workspace 22 \
  --title "Redis caching pattern" \
  --content "Use read-through caching with 5min TTL for..." \
  --tags "caching,redis,patterns" \
  --pretty
```

### Version evolving knowledge

When a decision or pattern changes, create a new version instead of updating:

```bash
recuerd0 memory version create 42 --workspace 22 \
  --content "Updated: Now using write-behind caching..." \
  --title "Redis caching pattern v2" \
  --pretty
```

### Organize with workspaces

Create project-specific workspaces to keep knowledge organized:

```bash
recuerd0 workspace create --name "my-rails-app" \
  --description "Architecture decisions and patterns for the Rails app" \
  --pretty
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Authentication error |
| 4 | Forbidden |
| 5 | Not found |
| 6 | Validation error |
| 7 | Network error |
| 8 | Rate limited |
