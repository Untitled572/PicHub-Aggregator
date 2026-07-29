# API Documentation

## Health Check

```
GET /ping
```

Response: `{"status":"ok"}`

## Random Image Distribution

```
GET /random?category=landscape&format=json&orientation=horizontal
```

**Parameters:**
- `category` (optional): Filter by category. Comma-separated for multiple: `?category=avatar,anime`
- `format` (optional): Set to `json` for JSON response instead of 302 redirect
- `orientation` (optional): Filter by image orientation (`horizontal` / `vertical`). Requires `proxy_mode=true`.

**Default response:** HTTP 302 redirect to image URL
**JSON response:** `{"url":"...","local_url":"...","source":"...","categories":["..."],"file_id":"...","width":1920,"height":1080,"format":"jpeg","image_id":1}`

**orientation 说明：** 基于图片真实宽高过滤（`image.DecodeConfig`），不依赖分类标签。仅在 `proxy_mode=true` 时生效。不匹配时自动重试其他图源。

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
  "cache_max_images": 60,
  "cache_ttl": 60,
  "min_resolution": "640x480",
  "rate_limit": 60,
  "timeout": 3000,
  "health_check_interval": 360,
  "max_history_records": 60,
  "bound_tags": ["horizontal"],
  "admin_token": "",
  "saved_images_dir": ""
}
```

**Note:** `min_resolution` 仅在 `proxy_mode=true` 时生效，输入 `0` 关闭分辨率过滤。

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

## Statistics

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/stats` | Get today's statistics and total |
| GET | `/api/stats/history` | Get recent image history |

**GET /api/stats Response:**
```json
{
  "today": {
    "total": 428,
    "tags": [
      {"tag_id": "horizontal", "count": 215},
      {"tag_id": "vertical", "count": 150}
    ],
    "sources": [
      {"source_id": 1, "source_name": "Picsum", "hit_count": 150}
    ]
  },
  "total": {
    "total_requests": 28471
  }
}
```

**GET /api/stats/history?limit=20&offset=0**
- `limit`: Number of records (max 200, default 20)
- `offset`: Pagination offset (default 0)

Response (cache mode 开启时返回 `image_id` / `file_id`):
```json
{
  "history": [
    {
      "id": 1,
      "image_url": "/images/a1b2c3d4",
      "source_id": 1,
      "source_name": "Picsum",
      "categories": "[\"horizontal\"]",
      "created_at": "2026-07-29 14:30:22",
      "image_id": 1,
      "file_id": "a1b2c3d4e5f6..."
    }
  ]
}
```

All successful `/random` requests are automatically recorded in the statistics.

## Image Serving

```
GET /images/:file_id
```

Serves a cached image file directly. Returns `image/jpeg`, `image/png`, `image/gif`, or `image/webp` with `Cache-Control: public, max-age=86400`. Requires `proxy_mode=true` to have cached images available.

## Saved Images (本地缓存图片保存/收藏)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/images/:id/save` | Admin | 保存图片（复制到 saved_images_dir） |
| DELETE | `/api/images/:id/save` | Admin | 取消保存 |
| GET | `/api/images/saved` | Public | 已保存图片列表（分页） |

**GET /api/images/saved?limit=20&offset=0**
```json
{
  "images": [
    {
      "id": 1,
      "file_id": "a1b2c3d4",
      "source_name": "Picsum",
      "width": 1920,
      "height": 1080,
      "format": "jpeg",
      "file_size": 245760,
      "original_url": "https://picsum.photos/1920/1080",
      "saved_at": "2026-07-29T15:00:00Z"
    }
  ],
  "total": 5,
  "limit": 20,
  "offset": 0
}
```

## Export / Import

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/export` | Export all rules (filters sensitive headers) |
| POST | `/api/import` | Import rules from JSON |
