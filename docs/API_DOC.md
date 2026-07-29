# API Documentation

## Health Check

```
GET /ping
```

Response: `{"status":"ok"}`

## Random Image Distribution

```
GET /random?category=landscape&format=json
```

**Parameters:**
- `category` (optional): Filter by category. Comma-separated for multiple: `?category=avatar,anime`
- `format` (optional): Set to `json` for JSON response instead of 302 redirect

**Default response:** HTTP 302 redirect to image URL
**JSON response:** `{"url":"...","source":"...","categories":["..."]}`

**Note:** 速率限制基于内存实现（单实例），多实例负载均衡场景下各实例独立计数。多实例部署建议配合 sticky session 或外部 Redis。

## Smart Detect

```
POST /random/detect
Content-Type: application/json

{"url": "https://api.example.com/image"}
```

Response examines the URL and returns type, headers, and JSON tree if applicable.

## API Sources CRUD

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/sources` | List all sources |
| GET | `/api/sources/:id` | Get single source |
| POST | `/api/sources` | Create source |
| PUT | `/api/sources/:id` | Update source |
| DELETE | `/api/sources/:id` | Delete source |
| POST | `/api/sources/:id/toggle` | Enable/disable source |

**Source JSON:**
```json
{
  "name": "My Source",
  "url": "https://picsum.photos/200",
  "resp_type": "redirect",
  "json_path": "data.url",
  "weight": 10,
  "categories": ["avatar", "landscape"],
  "headers": {"Referer": "https://example.com"},
  "enabled": true
}
```

## Health Check

```
POST /api/sources/health-check
```

Returns array of health results with status code, latency, and availability.

## Settings

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/settings` | Get settings |
| PUT | `/api/settings` | Update settings |

**Settings JSON:**
```json
{
  "proxy_mode": false,
  "cache_max_mb": 200,
  "cache_ttl": 60,
  "min_resolution": "800x600",
  "rate_limit": 60,
  "timeout": 3000,
  "health_check_interval": 360,
  "bound_tags": ["horizontal"]
}
```

**Note:** `min_resolution` 仅在 `proxy_mode=true` 时生效。

## Tags

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/tags` | Get all tags |
| PUT | `/api/tags` | Update tags (full replacement) |

**Tags JSON:**
```json
[
  {"id": "horizontal", "name": "横屏"},
  {"id": "vertical", "name": "竖屏"},
  {"id": "adaptive", "name": "自适应"}
]
```

## Health Status

```
GET /api/health
```

Returns cached health check results with last run timestamp.

## Export / Import

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/export` | Export all rules (filters sensitive headers) |
| POST | `/api/import` | Import rules from JSON |
