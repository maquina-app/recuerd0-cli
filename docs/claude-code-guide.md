# Using Recuerd0 with Claude Code

## Why Recuerd0?

Claude Code's MEMORY.md is useful but limited:

- Hard 200-line cap, loaded into every prompt
- Project-scoped — knowledge stays siloed in one repo
- No search, no versioning, no structure
- Gets stale fast, hard to know what to prune

Recuerd0 fills these gaps as the deep, searchable, cross-project knowledge layer.

| Layer | Role | Scope |
|-------|------|-------|
| **MEMORY.md** | Fast, always-loaded cheat sheet | Per-project, ~50 lines ideal |
| **Recuerd0** | Deep, searchable knowledge with versioning | Cross-project, unlimited |
| **Transcripts** | Session logs that become searchable memories | Per-session |

The key advantage: Claude Code can *query* Recuerd0 for exactly the context it needs on demand, rather than cramming everything into MEMORY.md.

## Getting Started

### Project setup — one workspace per project

Create a workspace for each project and add a `.recuerd0.yaml` to the project root so all memory commands default to the right workspace:

```bash
recuerd0 workspace create --name "my-project" --description "Backend API service"
# Note the workspace ID from the response, then:
echo "workspace: <id>" > .recuerd0.yaml
```

### Add a hint to CLAUDE.md

Tell Claude Code to use Recuerd0 as part of its workflow:

```markdown
Before starting non-trivial work, search recuerd0 for relevant memories
using FTS operators (AND, OR, NOT, prefix*, title:, body:).
```

## Workflows

### Pre-session context loading

Before starting work, search for relevant memories:

```bash
# Broad topic
recuerd0 search "authentication"

# Targeted with FTS operators
recuerd0 search "auth* AND session NOT oauth"

# Cross-project — search a shared patterns workspace
recuerd0 search "error handling" --workspace 47
```

### Capture knowledge during a session

When a session produces a reusable insight, gotcha, or pattern:

```bash
# A pattern worth remembering
recuerd0 memory create \
  --title "Go: test stdin override" \
  --content "Use io.Reader type explicitly, not anonymous interface. Override stdinReader var in tests." \
  --tags "go,testing,gotcha" \
  --source "claude-code"

# A debugging insight
recuerd0 memory create \
  --title "Postgres: partial index not used with OR" \
  --content "Partial indexes on WHERE col IS NOT NULL won't be used when the query has OR conditions..." \
  --tags "postgres,debugging,performance" \
  --source "claude-code"
```

### Archive session transcripts

After generating a `/transcript`, store it as a searchable memory:

```bash
cat transcripts/2026-02-06-001.md | recuerd0 memory create \
  --title "Session: removed /api/v1 prefix from API paths" \
  --content - \
  --tags "session,refactor,api" \
  --source "claude-code"
```

Now past sessions are searchable across all projects.

### Track evolving decisions with versions

Architecture decisions change over time. Store the initial decision, then version it:

```bash
# Initial decision
recuerd0 memory create \
  --title "Auth strategy" \
  --content "Using Bearer tokens with API keys. No OAuth for now — single-tenant." \
  --tags "decision,auth,architecture"

# Later, when the approach changes (use the memory ID from above)
recuerd0 memory version create 42 \
  --title "Auth strategy — added OAuth" \
  --content "Added OAuth2 for third-party integrations. Bearer tokens still used for CLI." \
  --tags "decision,auth,architecture"
```

The version history captures *why* things changed — something MEMORY.md can't do.

### Pipe anything from stdin

```bash
# From a file
cat notes.md | recuerd0 memory create --title "Meeting notes" --content -

# Snapshot recent git history
git log --oneline -20 | recuerd0 memory create \
  --title "Recent commits snapshot" \
  --content - \
  --tags "git,snapshot"

# Capture a failing test output
make test-unit 2>&1 | recuerd0 memory create \
  --title "Test failures before refactor" \
  --content - \
  --tags "debug,snapshot"
```

## Search Tips

Search is backed by SQLite FTS5. Use operators for precise queries:

```bash
# Prefix matching — auth, authentication, authorization
recuerd0 search "auth*"

# AND — both terms required
recuerd0 search "rails AND caching"

# OR — either term
recuerd0 search "postgres OR sqlite"

# NOT — exclude terms
recuerd0 search "deploy NOT heroku"

# Exact phrases
recuerd0 search '"error handling"'

# Field-specific — search only title or body
recuerd0 search "title:authentication"
recuerd0 search "body:caching"

# Combined
recuerd0 search "rails AND (caching OR performance) NOT redis"
```

## Suggested Tag Conventions

Consistent tags make search more effective across workspaces:

| Tag | Purpose |
|-----|---------|
| `gotcha` | Things that tripped you up |
| `pattern` | Reusable code patterns |
| `decision` | Architecture choices and rationale |
| `debug` | Hard-won debugging insights |
| `session` | Session transcripts |
| `refactor` | Refactoring notes and strategies |
| `performance` | Optimization findings |
| `snapshot` | Point-in-time captures (git log, test output) |

## Example: Full Session Workflow

```bash
# 1. Start a session — search for context
recuerd0 search "auth* AND middleware"

# 2. Read a relevant memory
recuerd0 memory show 85

# 3. Do the work...

# 4. Capture what you learned
recuerd0 memory create \
  --title "JWT middleware: token refresh edge case" \
  --content "When the access token expires mid-request..." \
  --tags "gotcha,auth,middleware" \
  --source "claude-code"

# 5. Archive the session transcript
cat transcripts/2026-02-07-001.md | recuerd0 memory create \
  --title "Session: JWT middleware refactor" \
  --content - \
  --tags "session,auth" \
  --source "claude-code"
```
