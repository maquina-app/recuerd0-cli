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
recuerd0 workspace context <id> [--limit N] [--no-body] [--max-body-chars N]
```

### Memories

```bash
recuerd0 memory list --workspace <ws_id> [--page N] [--category CAT]
recuerd0 memory show --workspace <ws_id> <memory_id>
recuerd0 memory create --workspace <ws_id> --title "Title" --content "Body" [--tags "a,b"] [--source "src"] [--category CAT]
recuerd0 memory update --workspace <ws_id> <memory_id> [--title "T"] [--content "C"] [--tags "a,b"] [--category CAT]
recuerd0 memory delete --workspace <ws_id> <memory_id>
recuerd0 memory read head <memory_id> [--workspace <ws_id>] [--lines N]           # First N lines (default 20)
recuerd0 memory read tail <memory_id> [--workspace <ws_id>] [--lines N]           # Last N lines (default 20)
recuerd0 memory read lines <memory_id> --start S --end E [--workspace <ws_id>]    # Inclusive 1-based window
recuerd0 memory read grep <memory_id> <pattern> [--workspace <ws_id>] [--context N] [--before N] [--after N]  # Grep with optional context
recuerd0 memory link list <memory_id> [--workspace <ws_id>]
recuerd0 memory link add <memory_id> --to <other_memory_id> [--workspace <ws_id>]
recuerd0 memory link remove <memory_id> --to <other_memory_id> [--workspace <ws_id>]
```

Content can be read from stdin with `--content -`.

For large memories, prefer `memory read` over `memory show` so you only pull the slice you need. The typical flow is grep first to locate relevant lines, then fetch a window with `memory read lines` around each match — the grep response's breadcrumbs already include the suggested `memory read lines` follow-ups for the first five matches.

### Memory Links

Memory links (tunnels) connect two memories that cover related topics, including across workspaces within the same account. Links are undirected, unlabeled, and same-account-only — cross-account or self-links are rejected with `422`. The CLI hides the join row id: `memory link remove` takes the *other* memory's id via `--to`. Memory and pinned-memory responses include a `links_count` field.

### Memory Versions

```bash
recuerd0 memory version create <memory_id> --workspace <ws_id> \
  [--title "T"] [--content "C"] [--tags "a,b"] [--source "src"] [--category CAT]
```

Creates a new version of a memory. Fields default to the parent version's values if omitted.

### Imports

Import is propose → review → commit: `propose` writes a reviewable `import.plan.yaml` and never touches the server; `commit` executes the plan you approved.

```bash
recuerd0 import propose <path> --workspace <ws_id> \
  [--plan import.plan.yaml] [--ledger PATH] \
  [--adapter obsidian_markdown|workspace_export] [--fresh]

recuerd0 import commit <plan> [--yes] [--ledger PATH] [--dry-run]
```

Directories auto-detect as Markdown/Obsidian imports; valid export-v1 JSON files auto-detect as workspace exports. Propose performs GET-only conflict detection and atomically saves the plan. Commit without `--yes`, or with `--dry-run`, validates and returns the same digest with exit 1 and no writes. `--dry-run` wins over `--yes`.

#### Import review protocol

1. Run propose and relay `adapter`, all `counts`, `titles_from_h1_pct`, `links_proposed`, `tags_proposed`, structured `exceptions`, `thin`, any `hint`, and `warnings` verbatim.
2. Review the YAML only. Keep every manifest `action` aligned with all exception `resolution` values for the same path.
3. After editing any scanner-owned field (`title`, `category`, `tags`, or `links`), re-run propose with the same source, workspace, plan, and ledger arguments. The edits are preserved and hashes refresh.
4. Confirm every `target_memory_id` before approving `version`; never attach one to `create`.
5. Re-run commit without `--yes` to validate and show the human the final digest.
6. Stop at the reviewed plan unless the human explicitly says go.
7. After approved execution, report `ops` and `plan.complete`; also surface `aborted`, `links_failed`, or `links_skipped_unresolvable`.

If present, relay this hint unchanged:

> This plan looks thin — refine it by hand or hand it to your agent (see the recuerd0 skill's import protocol).

The agent's job ends at the plan. Never import by writing memories one-by-one through MCP; always execute through `recuerdo import commit`, and pass `--yes` only after the human has seen the digest and said go.

### Search

```bash
recuerd0 search "<query>" [--workspace <ws_id>] [--page N] [--category CAT]
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

### Categories

Every memory carries a `category` — one of `decision`, `discovery`, `preference`, `general`. The `--category` flag is optional everywhere:

- On `memory create`: server defaults to `general` when omitted.
- On `memory version create`: server inherits from parent when omitted.
- On `memory update`: category changes only when explicitly set.
- On `memory list` and `search`: filters results to that category when set; returns all categories otherwise.

Invalid values (anything outside the four canonical ones) fail client-side with exit code 2 before any API call.

Use `decision` for choices the team made, `discovery` for findings/bug root-causes, `preference` for style/workflow preferences, `general` for everything else.

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
| GET | `/workspaces/:id/context` | `workspace context` |
| GET | `/workspaces/:ws/memories` | `memory list` |
| GET | `/workspaces/:ws/memories/:id` | `memory show`, `memory read head/tail/lines/grep` |
| POST | `/workspaces/:ws/memories` | `memory create` |
| PATCH | `/workspaces/:ws/memories/:id` | `memory update` |
| DELETE | `/workspaces/:ws/memories/:id` | `memory delete` |
| POST | `/workspaces/:ws/memories/:id/versions` | `memory version create` |
| GET | `/workspaces/:ws/memories/:id/links` | `memory link list` |
| POST | `/workspaces/:ws/memories/:id/links` | `memory link add` |
| DELETE | `/workspaces/:ws/memories/:id/links/:other_id` | `memory link remove` |
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
