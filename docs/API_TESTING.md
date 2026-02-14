# API Testing Report

## Run 3 — 2026-02-14 (all issues resolved)

Environment: local account (`http://localhost:3820`)
Binary: `bin/recuerd0` (built from source with all CLI fixes applied)
Server: local Rails server with server-side fixes for version create and workspace create

### Test Results Summary

| # | Operation | Command | Run 1 | Run 2 | Run 3 |
|---|-----------|---------|-------|-------|-------|
| 1 | Create workspace | `workspace create` | PASS | PASS | PASS |
| 2 | Show workspace | `workspace show` | PASS | PASS | PASS |
| 3 | Update workspace | `workspace update` | PASS | PASS | PASS |
| 4 | List workspaces | `workspace list` | PASS | PASS | PASS |
| 5 | Archive workspace | `workspace archive` | FAIL (404) | PASS | PASS |
| 6 | Unarchive workspace | `workspace unarchive` | NOT TESTED | PASS | PASS |
| 7 | Create memory | `memory create` | PASS | PASS | PASS |
| 8 | Show memory | `memory show` | PASS | PASS | PASS |
| 9 | Update memory | `memory update` | PASS | PASS | PASS |
| 10 | Delete memory | `memory delete` | PASS | PASS | PASS |
| 11 | Create memory version | `memory version create` | PASS (content ignored) | PASS (content ignored) | PASS |
| 12 | Update latest version | `memory update` on version | PASS | PASS | PASS |
| 13 | Search memories | `search` | PASS (wrong count) | PASS | PASS |

All 13 scenarios pass with no remaining issues.

---

## Test Scenarios

### 1. Create workspace

```bash
recuerd0 workspace create --name "Test Operations" \
  --description "Workspace for testing all CLI operations" \
  --account local --pretty
```

**Result:** PASS — Returns full workspace object with `created_at`, `url`, and `memories_count: 0`.

---

### 2. Show workspace

```bash
recuerd0 workspace show <id> --account local --pretty
```

**Result:** PASS — Returns workspace details including name, description, archived status, and memories count.

---

### 3. Update workspace

```bash
recuerd0 workspace update <id> \
  --name "Test Operations Updated" \
  --description "Updated description for testing" \
  --account local --pretty
```

**Result:** PASS — Both name and description updated. `updated_at` timestamp changed.

---

### 4. List workspaces

```bash
recuerd0 workspace list --account local --pretty
```

**Result:** PASS — Returns paginated list of workspaces. Pagination works (`has_next: true`, `next_url` present when more pages exist).

---

### 5. Archive workspace

```bash
recuerd0 workspace archive <id> --account local --pretty
```

**Result:** PASS — Returns workspace with `archived: true`.

**History:** Run 1 failed with 404 because CLI sent `PATCH` but server expects `POST /workspaces/:id/archive`. Fixed in CLI.

---

### 6. Unarchive workspace

```bash
recuerd0 workspace unarchive <id> --account local --pretty
```

**Result:** PASS — Returns workspace with `archived: false`.

**History:** Not tested in run 1 (blocked by archive failure). Server route is `DELETE /workspaces/:id/archive`. Fixed in CLI.

---

### 7. Create memory

```bash
recuerd0 memory create --workspace <ws_id> \
  --title "CLI Architecture" \
  --content "The recuerd0 CLI is built with Go and Cobra." \
  --tags "architecture,go" \
  --account local --pretty
```

**Result:** PASS — Returns full memory object with content, tags, version info (`version: 1`, `version_label: "v1"`), and workspace reference.

---

### 8. Show memory

```bash
recuerd0 memory show --workspace <ws_id> <memory_id> --account local --pretty
```

**Result:** PASS — Returns full memory including content body, tags, version info, and workspace reference.

---

### 9. Update memory

```bash
recuerd0 memory update --workspace <ws_id> <memory_id> \
  --title "CLI Architecture Overview" \
  --content "Updated content here." \
  --tags "architecture,go,patterns" \
  --account local --pretty
```

**Result:** PASS — Title, content, and tags all updated. `updated_at` timestamp changed. Version remains `v1`.

---

### 10. Delete memory

```bash
recuerd0 memory delete --workspace <ws_id> <memory_id> --account local --pretty
```

**Result:** PASS — Returns `{"deleted": "<id>"}`. Deleting a memory deletes all its versions (attempting to delete a version ID after the parent is deleted returns 404).

---

### 11. Create memory version

```bash
# First create the base memory
recuerd0 memory create --workspace <ws_id> \
  --title "Test Versioning" \
  --content "Version 1: Initial content." \
  --tags "test,versioning" \
  --account local --pretty

# Then create version 2
recuerd0 memory version create <memory_id> --workspace <ws_id> \
  --content "Version 2: Updated content." \
  --title "Test Versioning v2" \
  --tags "test,versioning,v2" \
  --account local --pretty
```

**Result:** PASS — Version v2 created with correct content, title, and tags. Verified by fetching the new version separately.

**History:** Runs 1-2 failed because the server's version create action ignored submitted content/title (strong params issue). Fixed server-side.

**CLI payload:** `POST /workspaces/:ws_id/memories/:memory_id/versions` with body `{"version":{"title":"...","content":"...","tags":["..."]}}`

---

### 12. Update latest version

```bash
recuerd0 memory update --workspace <ws_id> <version_id> \
  --content "Version 2: Updated with new details." \
  --title "Test Versioning Updated" \
  --account local --pretty
```

**Result:** PASS — Content and title both changed on the latest version. Tags preserved from original. `updated_at` changed.

---

### 13. Search memories

```bash
recuerd0 search "Rails" --account local --pretty
```

**Result:** PASS — Summary correctly shows result count (e.g., "25 result(s) for \"Rails\""). Returns matching memories with snippets, tags, workspace reference, and version info.

**History:** Run 1 showed "0 result(s)" while results were present because CLI used `countItems()` on a map instead of reading `total_results`. Fixed in CLI.

---

## Fixes Applied

### CLI Fixes

#### 1. `extractErrorMessage` in `internal/client/client.go`

Updated to handle the API's nested error format:

- **Before:** Only handled `{"error": "string"}`, `{"message": "string"}`, and `{"errors": ["string"]}`.
- **After:** Also handles:
  - `{"error": {"code": "...", "message": "...", "details": {"field": ["msg"]}}}` — extracts field-level validation details when present, falls back to `message`.
  - `{"errors": {"field": ["msg"]}}` — Rails-style validation errors at top level.

#### 2. Archive/Unarchive HTTP methods in `internal/commands/workspace_archive.go`

- **Archive:** Changed from `apiClient.Patch("/workspaces/:id/archive")` to `apiClient.Post("/workspaces/:id/archive")`.
- **Unarchive:** Changed from `apiClient.Patch("/workspaces/:id/unarchive")` to `apiClient.Delete("/workspaces/:id/archive")`.

#### 3. Search summary in `internal/commands/search.go`

- **Before:** Used `countItems(resp.Data)` which cast the response as an array (returns 0 since search data is a map with `query`, `results`, `total_results`).
- **After:** Uses `countSearchResults(resp.Data)` which reads `total_results` from the response map, falling back to counting the `results` array length.

#### 4. API docs in `docs/API.md`

- Updated with complete endpoint documentation including request/response examples, error formats, pagination headers, search operators, and token permissions.

### Server Fixes (applied separately)

1. **Workspace create** — Production server was returning 422 for all POST requests (server-side issue, not CLI).
2. **Version create** — Server's versions controller was not applying submitted `content`, `title`, `source`, and `tags` to new versions (strong params issue).

## API Endpoint Reference

| Method | Path | CLI Command |
|--------|------|-------------|
| GET | `/workspaces` | `workspace list` |
| GET | `/workspaces/:id` | `workspace show <id>` |
| POST | `/workspaces` | `workspace create` |
| PATCH | `/workspaces/:id` | `workspace update <id>` |
| POST | `/workspaces/:id/archive` | `workspace archive <id>` |
| DELETE | `/workspaces/:id/archive` | `workspace unarchive <id>` |
| GET | `/workspaces/:ws/memories` | `memory list --workspace <ws>` |
| GET | `/workspaces/:ws/memories/:id` | `memory show --workspace <ws> <id>` |
| POST | `/workspaces/:ws/memories` | `memory create --workspace <ws>` |
| PATCH | `/workspaces/:ws/memories/:id` | `memory update --workspace <ws> <id>` |
| DELETE | `/workspaces/:ws/memories/:id` | `memory delete --workspace <ws> <id>` |
| POST | `/workspaces/:ws/memories/:id/versions` | `memory version create <id> --workspace <ws>` |
| GET | `/search?q=<query>` | `search "<query>"` |
