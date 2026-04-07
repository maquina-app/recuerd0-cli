# Recuerd0 API

The Recuerd0 API provides programmatic access to workspaces and memories. All responses are in JSON format.

## Authentication

All API requests require authentication via Bearer token and a JSON content type. Include these headers with every request:

```
Authorization: Bearer your_token_here
Content-Type: application/json
```

All responses use `Content-Type: application/json`.

### Token Permissions

- **read_only**: Can access GET endpoints only
- **full_access**: Can access all endpoints (GET, POST, PATCH, DELETE)

### Errors

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid or missing access token",
    "status": 401
  }
}
```

## Rate Limiting

API requests are limited to 100 requests per minute per token. When exceeded:

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Rate limit exceeded. Please try again later.",
    "status": 429
  }
}
```

---

## Workspaces

### List Workspaces

Returns all active workspaces for the current account.

```
GET /workspaces.json
```

**Parameters**

| Name | Type | Description |
|------|------|-------------|
| page | integer | Page number (default: 1) |

**Response**

```json
[
  {
    "id": 1,
    "name": "Project Alpha",
    "description": "Main project workspace",
    "memories_count": 42,
    "archived": false,
    "created_at": "2026-01-15T10:30:00Z",
    "updated_at": "2026-02-04T14:22:00Z",
    "url": "https://recuerd0.com/workspaces/1"
  }
]
```

**Headers**

| Header | Description |
|--------|-------------|
| X-Page | Current page number |
| X-Per-Page | Items per page |
| X-Total | Total item count |
| X-Total-Pages | Total pages |
| Link | Pagination links (first, prev, next, last) |

---

### Get Workspace

Returns a single workspace with memory count and pinned status.

```
GET /workspaces/:id.json
```

**Response**

```json
{
  "id": 1,
  "name": "Project Alpha",
  "description": "Main project workspace",
  "memories_count": 42,
  "archived": false,
  "created_at": "2026-01-15T10:30:00Z",
  "updated_at": "2026-02-04T14:22:00Z",
  "url": "https://recuerd0.com/workspaces/1"
}
```

---

### Create Workspace

Creates a new workspace. Requires `full_access` token.

```
POST /workspaces.json
```

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| workspace[name] | string | Yes | Workspace name (max 100 characters) |
| workspace[description] | string | No | Workspace description |

**Request**

```json
{
  "workspace": {
    "name": "New Project",
    "description": "A new workspace for the team"
  }
}
```

**Response** `201 Created`

```json
{
  "id": 2,
  "name": "New Project",
  "description": "A new workspace for the team",
  "memories_count": 0,
  "archived": false,
  "created_at": "2026-02-04T15:00:00Z",
  "updated_at": "2026-02-04T15:00:00Z",
  "url": "https://recuerd0.com/workspaces/2"
}
```

**Errors**

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Name can't be blank",
    "details": {
      "name": ["can't be blank"]
    },
    "status": 422
  }
}
```

---

### Update Workspace

Updates an existing workspace. Requires `full_access` token.

```
PATCH /workspaces/:id.json
```

**Parameters**

| Name | Type | Description |
|------|------|-------------|
| workspace[name] | string | Workspace name |
| workspace[description] | string | Workspace description |

**Request**

```json
{
  "workspace": {
    "name": "Updated Name"
  }
}
```

**Response** `200 OK`

Returns the updated workspace object.

---

### Archive Workspace

Archives a workspace. Requires `full_access` token. Archived workspaces become read-only.

```
POST /workspaces/:id/archive.json
```

**Response** `200 OK`

Returns the updated workspace object with `archived: true`.

---

### Unarchive Workspace

Restores an archived workspace. Requires `full_access` token.

```
DELETE /workspaces/:id/archive.json
```

**Response** `200 OK`

Returns the updated workspace object with `archived: false`.

---

### Get Workspace Context

Returns a wake-up payload for AI agents: workspace metadata plus the current user's pinned memories filtered to this workspace, in one call. Designed to be loaded into a system prompt as a compact snapshot. Requires `read_only` or `full_access` token. Active and archived workspaces both return 200; deleted workspaces return 404.

```
GET /workspaces/:id/context.json
```

**Query parameters**

| Name | Type | Default | Description |
|------|------|---------|-------------|
| limit | integer | `10` | Max pinned memories to include. Clamped to `1..50`. |
| include_body | boolean | `true` | When `false`, omits the memory `body` field. |
| max_body_chars | integer | `500` | When `include_body=true`, truncate each body to this many characters. Clamped to `100..5000`. |

**Response** `200 OK`

```json
{
  "workspace": {
    "id": "1",
    "name": "Rails Patterns",
    "description": "Architecture, patterns, decisions",
    "memories_count": 42,
    "state": "active",
    "updated_at": "2026-04-06T12:00:00Z",
    "url": "https://recuerd0.ai/workspaces/1"
  },
  "pinned_memories": [
    {
      "id": "12",
      "title": "FTS5 error handling",
      "source": "manual",
      "tags": ["fts5", "errors"],
      "pinned_at": "2026-03-01T10:00:00Z",
      "updated_at": "2026-03-15T08:00:00Z",
      "url": "https://recuerd0.ai/workspaces/1/memories/12",
      "body": "SQLite FTS5 syntax errors surface as ActiveRecord::StatementInvalid…",
      "body_truncated": true
    }
  ],
  "stats": {
    "total_memories": 42,
    "total_pinned": 5,
    "returned_pinned": 5
  },
  "generated_at": "2026-04-07T04:55:00Z"
}
```

The endpoint supports HTTP caching: `If-None-Match` and `If-Modified-Since` request headers return `304 Not Modified` when nothing has changed.

---

## Memory Categories

Every memory carries a `category` field drawn from a locked enum:

- `decision` — choices the team made, with rationale
- `discovery` — findings, bug root-causes, new facts about the system
- `preference` — style/taste/workflow preferences
- `general` — default bucket for anything else

The server defaults new memories to `general` when omitted. New versions inherit the parent's category unless explicitly overridden. List and search endpoints accept a `category` query parameter to filter by a single category.

---

## Memories

### List Memories

Returns all memories (latest versions only) for a workspace.

```
GET /workspaces/:workspace_id/memories.json
```

**Parameters**

| Name | Type | Description |
|------|------|-------------|
| page | integer | Page number (default: 1) |
| category | string | Filter by category (`decision`, `discovery`, `preference`, `general`) |

**Response**

```json
[
  {
    "id": 1,
    "title": "Meeting Notes",
    "version": 1,
    "source": "manual",
    "tags": ["meetings", "q1"],
    "created_at": "2026-01-20T09:00:00Z",
    "updated_at": "2026-02-03T16:45:00Z",
    "url": "https://recuerd0.com/workspaces/1/memories/1"
  }
]
```

**Headers**

Same pagination headers as workspace list.

---

### Get Memory

Returns a memory with its content.

```
GET /workspaces/:workspace_id/memories/:id.json
```

**Response**

```json
{
  "id": 1,
  "title": "Meeting Notes",
  "version": 1,
  "source": "manual",
  "tags": ["meetings", "q1"],
  "created_at": "2026-01-20T09:00:00Z",
  "updated_at": "2026-02-03T16:45:00Z",
  "url": "https://recuerd0.com/workspaces/1/memories/1",
  "content": {
    "body": "# Meeting Notes\n\nDiscussed Q1 goals..."
  },
  "workspace": {
    "id": 1,
    "name": "Project Alpha",
    "url": "https://recuerd0.com/workspaces/1"
  }
}
```

---

### Create Memory

Creates a new memory with content. Requires `full_access` token.

```
POST /workspaces/:workspace_id/memories.json
```

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| memory[title] | string | No | Memory title (max 255 characters) |
| memory[content] | string | No | Memory body (Markdown) |
| memory[source] | string | No | Source identifier |
| memory[tags] | array | No | Array of tag strings |
| memory[category] | string | No | One of `decision`, `discovery`, `preference`, `general` (defaults to `general`) |

**Request**

```json
{
  "memory": {
    "title": "API Documentation",
    "content": "# Overview\n\nThis document describes...",
    "tags": ["docs", "api"]
  }
}
```

**Response** `201 Created`

Returns the created memory object with content.

---

### Update Memory

Updates an existing memory. Requires `full_access` token.

```
PATCH /workspaces/:workspace_id/memories/:id.json
```

**Parameters**

| Name | Type | Description |
|------|------|-------------|
| memory[title] | string | Memory title |
| memory[content] | string | Memory body |
| memory[source] | string | Source identifier |
| memory[tags] | array | Array of tags |
| memory[category] | string | One of `decision`, `discovery`, `preference`, `general` |

**Response** `200 OK`

Returns the updated memory object with content.

---

### Delete Memory

Deletes a memory and all its versions. Requires `full_access` token.

```
DELETE /workspaces/:workspace_id/memories/:id.json
```

**Response** `204 No Content`

---

## Memory Links

Memory links (also called *tunnels*) connect two memories that cover related topics across workspace boundaries. Links are undirected, unlabeled, and only allowed between memories owned by the same account; cross-account links return `422 Unprocessable Entity`. Self-links (linking a memory to itself) also return `422`. Memory and pinned-memory responses now include a `links_count` field reflecting how many tunnels a given memory participates in.

### List Links

Lists memories linked to the given memory. Accepts `read_only` or `full_access` tokens.

```
GET /workspaces/:workspace_id/memories/:memory_id/links.json
```

### Create Link

Creates a link between `:memory_id` and another memory. Requires `full_access`. The other memory may live in a different workspace, but must belong to the same account.

```
POST /workspaces/:workspace_id/memories/:memory_id/links.json
```

**Body**
```json
{ "to_memory_id": 99 }
```

Returns `422` if `to_memory_id` is missing, equals `:memory_id`, or points to a memory owned by a different account.

### Delete Link

Removes the link between `:memory_id` and the memory whose id appears as `:id` in the URL. Note that `:id` is the **other memory's id**, not a join row id. Requires `full_access`.

```
DELETE /workspaces/:workspace_id/memories/:memory_id/links/:id.json
```

**Response** `204 No Content`

---

## Memory Versions

### Create Version

Creates a new version of a memory. Requires `full_access` token.

```
POST /workspaces/:workspace_id/memories/:memory_id/versions.json
```

**Parameters**

| Name | Type | Description |
|------|------|-------------|
| version[title] | string | Version title (defaults to parent) |
| version[content] | string | Version body (defaults to parent) |
| version[source] | string | Source identifier (defaults to parent) |
| version[tags] | array | Tags (defaults to parent) |
| version[category] | string | One of `decision`, `discovery`, `preference`, `general` (defaults to parent) |

**Request**

```json
{
  "version": {
    "content": "# Updated Content\n\nRevised version..."
  }
}
```

**Response** `201 Created`

```json
{
  "id": 5,
  "title": "Meeting Notes",
  "version": 2,
  "source": "manual",
  "tags": ["meetings", "q1"],
  "created_at": "2026-02-04T16:00:00Z",
  "updated_at": "2026-02-04T16:00:00Z",
  "url": "https://recuerd0.com/workspaces/1/memories/5",
  "content": {
    "body": "# Updated Content\n\nRevised version..."
  },
  "workspace": {
    "id": 1,
    "name": "Project Alpha",
    "url": "https://recuerd0.com/workspaces/1"
  }
}
```

---

## Search

### Search Memories

Full-text search across all memories in active workspaces. Supports FTS5 query operators for advanced search patterns. Requires `read_only` or `full_access` token.

```
GET /search.json?q=<query>
```

**Parameters**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| q | string | Yes | Search query (3-100 characters) |
| page | integer | No | Page number (default: 1) |
| workspace_id | integer | No | Filter results to a specific workspace |
| category | string | No | Filter results by category (`decision`, `discovery`, `preference`, `general`) |

**Query Operators**

The search query supports full FTS5 syntax:

| Operator | Example | Description |
|----------|---------|-------------|
| Term | `architecture` | Matches documents containing the substring |
| AND | `architecture AND design` | Both terms must appear |
| OR | `meeting OR standup` | Either term can appear |
| NOT | `design NOT draft` | First term must appear, second must not |
| Phrase | `"project timeline"` | Exact phrase match |
| Column filter | `title:architecture` | Search only in title field |
| Column filter | `body:implementation` | Search only in body field |
| Grouping | `(meeting OR standup) AND notes` | Parentheses for precedence |

**Response**

```json
{
  "query": "architecture AND design",
  "total_results": 3,
  "results": [
    {
      "id": 1,
      "title": "Design Doc",
      "version": 1,
      "version_label": "v1",
      "has_versions": false,
      "tags": ["design"],
      "source": "manual",
      "snippet": "Initial architecture overview. The system uses a layered design...",
      "created_at": "2026-01-20T09:00:00Z",
      "updated_at": "2026-02-03T16:45:00Z",
      "url": "https://recuerd0.com/workspaces/1/memories/1",
      "workspace": {
        "id": 1,
        "name": "Project Notes",
        "url": "https://recuerd0.com/workspaces/1"
      }
    }
  ]
}
```

**Headers**

Same pagination headers as other list endpoints. Pagination links preserve the `q` parameter.

**Errors**

Missing or empty query:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Query parameter is required",
    "status": 422
  }
}
```

Query too short:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Query must be at least 3 characters",
    "status": 422
  }
}
```

Invalid FTS5 syntax:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid search query syntax",
    "status": 422
  }
}
```

---

## Common Errors

### 401 Unauthorized

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid or missing access token",
    "status": 401
  }
}
```

### 403 Forbidden

Returned when using a `read_only` token for write operations:

```json
{
  "error": {
    "code": "FORBIDDEN",
    "message": "Insufficient permissions",
    "status": 403
  }
}
```

Or when accessing an inactive workspace:

```json
{
  "error": {
    "code": "FORBIDDEN",
    "message": "Workspace is not active",
    "status": 403
  }
}
```

### 404 Not Found

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Resource not found",
    "status": 404
  }
}
```

### 422 Unprocessable Entity

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Name can't be blank",
    "details": {
      "name": ["can't be blank"]
    },
    "status": 422
  }
}
```

### 429 Too Many Requests

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Rate limit exceeded. Please try again later.",
    "status": 429
  }
}
```
