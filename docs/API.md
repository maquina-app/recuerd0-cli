# Recuerd0 API Reference

## Base URL

```
https://recuerd0.ai
```

## Authentication

All API requests require a Bearer token in the `Authorization` header:

```
Authorization: Bearer <token>
Content-Type: application/json
```

All requests must include `Content-Type: application/json` and `Accept: application/json` headers.

## Endpoints

### Workspaces

| Method | Path | Description |
|--------|------|-------------|
| GET | `/workspaces` | List workspaces |
| GET | `/workspaces/:id` | Show workspace |
| POST | `/workspaces` | Create workspace |
| PATCH | `/workspaces/:id` | Update workspace |
| PATCH | `/workspaces/:id/archive` | Archive workspace |
| PATCH | `/workspaces/:id/unarchive` | Unarchive workspace |

### Memories

| Method | Path | Description |
|--------|------|-------------|
| GET | `/workspaces/:workspace_id/memories` | List memories |
| GET | `/workspaces/:workspace_id/memories/:id` | Show memory |
| POST | `/workspaces/:workspace_id/memories` | Create memory |
| PATCH | `/workspaces/:workspace_id/memories/:id` | Update memory |
| DELETE | `/workspaces/:workspace_id/memories/:id` | Delete memory |

### Memory Versions

| Method | Path | Description |
|--------|------|-------------|
| POST | `/workspaces/:workspace_id/memories/:memory_id/versions` | Create version |

### Search

| Method | Path | Description |
|--------|------|-------------|
| GET | `/search?q=<query>` | Search memories |

Query parameters: `q` (required), `workspace_id` (optional), `page` (optional).

## Pagination

List endpoints support pagination via the `page` query parameter. The response includes a `Link` header with `rel="next"` when more pages are available.

## Error Responses

Error responses return JSON with an `error` or `message` field:

```json
{
  "error": "Not found"
}
```

HTTP status codes map to error types:
- `401` — Authentication error
- `403` — Forbidden
- `404` — Not found
- `422` — Validation error
- `429` — Rate limited
