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

**Exclusive Tag 说明：** 带有 `exclusive: true` 标记的独占标签（如 `nsfw` / `square`）不会被匿名/未指定标签的随机请求抽中。仅当客户端在请求参数中显式指定 `?category=nsfw` 或绑定该 Tag 时方可触发该类图源分发。独占标签由用户在控制台自行添加/标记，非系统预设。

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
  "resp_type": "json",
  "json_path": "data.url",
  "weight": 50,
  "default_query": "format=json",
  "categories": ["avatar", "landscape"],
  "headers": {"Referer": "https://example.com"},
  "params": [
    {"key": "type", "value": "pc", "categories": ["horizontal"], "weight": 50},
    {"key": "/mobile.php", "value": "手机专区", "categories": ["vertical"], "weight": 50}
  ],
  "enabled": true
}
```

**Sub-API & Query Params (子 API 链接与参数分支说明):**
- 支持在 `params` 中定义参数分支（如 `key: "type", value: "pc"`）或独立的子 API 链接（如 `key: "/mobile.php", value: "手机专区"` 或完整 URL `https://...`）。
- 子分支自动继承主图源属性，同时支持独立的 Tag 绑定与 50 初始基准权重设定。在分发历史流水中将精准记录如 `My Source › 手机专区 (/mobile.php)`。

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
  "proxy_enabled": false,
  "proxy_url": "http://127.0.0.1:7890",
  "pool_size": 10,
  "cache_max_mb": 200,
  "cache_max_images": 60,
  "cache_ttl": 60,
  "min_resolution": "1920x1080",
  "rate_limit": 60,
  "timeout": 3000,
  "health_check_interval": 360,
  "max_history_records": 60,
  "bound_tags": ["horizontal"],
  "admin_token": "",
  "saved_images_dir": "./data/saved",
  "login_enabled": false,
  "admin_username": "admin",
  "session_hours": 3
}
```

**Note:** `min_resolution` 仅在 `proxy_mode=true` 时生效，输入 `0` 关闭分辨率过滤。`proxy_enabled` 与 `proxy_url` 用于配置抓取图源时的 HTTP/HTTPS 代理。

**登录保护相关字段:**
- `login_enabled` (bool): 启用用户名+密码登录保护（默认关闭）。启用后所有写操作（POST/PUT/DELETE）需携带登录会话 token 或旧版 admin_token；GET 接口保持公开。
- `admin_username` (string): 登录用户名。
- `admin_password` (string, 仅写): 设置新密码时提交（**明文即可，前端自动转 MD5 摘要**），留空表示不修改。不会在响应中回显。
- `session_hours` (number): 会话有效时长（默认 3 小时），每次请求自动滑动续期；服务重启后会话清空需重新登录。

## Authentication (登录鉴权)

### 登录

```
POST /api/login
Content-Type: application/json

{"username": "admin", "password": "<MD5摘要>"}
```

成功返回 `{"token": "..."}`；失败返回 401（含 500~2000ms 随机延迟防爆破）。仅当 `login_enabled=true` 且已配置用户名密码时可用。

### 登出

```
POST /api/logout
Authorization: Bearer <token>
```

使当前会话 token 失效（幂等）。

### 鉴权规则

- **写操作**（POST/PUT/DELETE）：携带 `Authorization: Bearer <token>`。token 为登录会话令牌，或旧版 `admin_token`（`login_enabled=true` 时仍兼容）。
- **首次初始化**：`login_enabled=true` 但尚未配置用户名/密码（`admin_username` 为空）时，写操作临时放行，便于首次运行设置账号；配置完成后立即恢复校验。
- **凭据变更**（修改密码/用户名/开关登录）：所有已签发会话立即失效，需重新登录。
- **会话存储**：内存实现，重启失效；单实例部署限制。
- **前端行为**：收到 401 自动清除本地 token 并跳转 `/login` 登录页。
- **响应头**：所有接口返回 `X-Server-Time`（服务器毫秒时间戳），前端用于检测客户端/服务端时差漂移（>30 分钟告警）。

## Tags

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/tags` | Get all tags |
| PUT | `/api/tags` | Update tags (full replacement) |

**Tags JSON:**
```json
[
  {"id": "horizontal", "name": "横屏", "system": true},
  {"id": "vertical", "name": "竖屏", "system": true},
  {"id": "adaptive", "name": "自适应", "system": true},
  {"id": "square", "name": "正方形头像", "exclusive": true}
]
```

**Tags 分类说明：**
- `system: true`：系统内置硬编程只读规则标签（`横屏`、`竖屏`、`自适应`），前端归集于【系统内置标签框】展示。
- `exclusive: true`：独占隔离标签（如 `square`），仅在明确指定该标签时触发抽中分发。独占标签由用户自行添加/标记。

## Health Status

```
GET /api/health
```

Returns cached health check results with last run timestamp.

## Statistics & Weight Adjustment

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/stats` | Public | 获取统计数据（日请求量、命中图源排行、Tag分布） |
| GET | `/api/stats/history` | Public | 获取分发历史流水日志（包含 `is_saved` 实时保存高亮标记） |
| POST | `/api/images/:id/like` | Admin | 喜欢/提升权重 (+1 权重，钳位 30-70) |
| POST | `/api/images/:id/dislike` | Admin | 不喜欢/降低权重 (-1 权重，钳位 30-70) |

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

Response (包含 `is_saved` 用于前端点亮 ❤️ 按钮):
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
      "file_id": "a1b2c3d4e5f6...",
      "is_saved": true
    }
  ]
}
```

## Image Serving & Direct Download

```
GET /images/:file_id
```

Serves a cached image file directly. Returns `image/jpeg`, `image/png`, `image/gif`, or `image/webp` with `Cache-Control: public, max-age=86400`. Supports direct browser file downloading via `a.download`.

## Saved Images (离线保存图片库)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/images/:id/save` | Admin | 纯文件转存（复制到 saved_images_dir，不改变权重，支持 `file_id` / `image_id`） |
| DELETE | `/api/images/:id/save` | Admin | 取消保存 |
| GET | `/api/images/saved` | Public | 已保存图片列表（分页/无限滚动） |

**前端视图模式 (Saved Gallery Views):**
- 📋 **列表视图**：经典表格呈现，适合精细化管理与信息比对。
- 🖼️ **小图图墙**：高密度卡片，包含全量元数据与一键【下载】到本地功能。
- 🏙️ **大图图墙**：无圆角无边框巨幅全图瀑布流拼接，取消分页栏，采用 `IntersectionObserver` 无限滚动与 `loading="lazy"` 按需懒加载。

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
