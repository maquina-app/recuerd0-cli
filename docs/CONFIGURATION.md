# Configuration

## Multi-Account Config

Recuerd0 CLI supports multiple named accounts. Each account has its own API token and optional custom API URL.

### Global Config

Location: `~/.config/recuerd0/config.yaml`

```yaml
current: personal
accounts:
  personal:
    token: "tok_abc123"
    api_url: "https://recuerd0.ai"
  work:
    token: "tok_xyz789"
    api_url: "https://work.recuerd0.ai"
```

### Local Config

Location: `.recuerd0.yaml` (searched upward from current directory)

```yaml
account: work
workspace: "5"
```

This allows per-project overrides — e.g., a work project always uses the `work` account and workspace `5`.

## Resolution Order

Configuration is resolved with the following priority (highest wins):

1. **CLI flags**: `--account`, `--token`, `--api-url`, `--workspace`
2. **Environment variables**: `RECUERD0_ACCOUNT`, `RECUERD0_TOKEN`, `RECUERD0_API_URL`, `RECUERD0_WORKSPACE`
3. **Local config**: `.recuerd0.yaml` (walked up from current directory)
4. **Global config**: `~/.config/recuerd0/config.yaml` (uses the `current` account)

## Environment Variables

| Variable | Description |
|----------|-------------|
| `RECUERD0_ACCOUNT` | Account name to use |
| `RECUERD0_TOKEN` | API token (overrides account token) |
| `RECUERD0_API_URL` | API base URL (overrides account URL) |
| `RECUERD0_WORKSPACE` | Default workspace ID |

## Account Management

```bash
# Add an account (first account becomes default)
recuerd0 account add personal --token tok_abc123

# Add with custom API URL
recuerd0 account add work --token tok_xyz789 --api-url https://work.recuerd0.ai

# List accounts
recuerd0 account list

# Switch active account
recuerd0 account select work

# Remove an account
recuerd0 account remove old-account
```

## Example Workflows

### Personal Use
```bash
recuerd0 account add personal --token tok_abc123
recuerd0 workspace list
recuerd0 memory list --workspace 1
```

### Team Project with Local Config
```bash
# In project root, create .recuerd0.yaml:
# account: work
# workspace: "5"

recuerd0 memory list          # Uses work account, workspace 5
recuerd0 memory create --title "API patterns" --content "..."
```

### CI/CD
```bash
export RECUERD0_TOKEN=tok_ci_token
export RECUERD0_WORKSPACE=10
recuerd0 memory create --title "Build notes" --content -  < notes.md
```

### One-Off Override
```bash
recuerd0 --account work --workspace 3 memory list
```
