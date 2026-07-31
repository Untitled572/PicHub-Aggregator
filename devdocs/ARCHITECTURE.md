# PicHub-Aggregator Architecture

> Updated: 2026-07-31. Update this file when making significant backend changes.
## Tech Stack
- **Backend**: Go (Gin framework) + SQLite (mattn/go-sqlite3, CGO required)
- **Frontend**: Vue 3 + TypeScript + Tailwind CSS (embedded via `embed`); @vueuse/core (localStorage 状态/主题), crypto-js (登录 MD5), unplugin-auto-import (自动导入), vite-plugin-compression (gzip 预构建)
- **Deploy**: Docker (host network, port 5721)
- **Image**: GHCR (`ghcr.io/untitled572/pichub-aggregator`), ACR (`crpi-1ml0uqt093yc1v96.cn-beijing.personal.cr.aliyuncs.com/untitled572/pichub`)

## Directory Layout
```
backend/
├── main.go              # Entry point, route registration, middleware setup
├── handler/             # HTTP handlers per resource
│   ├── handler.go       # Handler struct (holds engine + store)
│   ├── random.go        # RandomImage, DetectURL, BatchHealthCheck, GetHealthStatus
│   ├── source.go        # CRUD + toggle for sources
│   ├── settings.go      # Get/Update settings (含密码保留/会话失效逻辑)
│   ├── auth.go          # Login / Logout (会话签发与销毁)
│   ├── tags.go          # Get/Update tags
│   ├── stats.go         # GetStats, GetImageHistory
│   ├── image.go         # ServeImage, SaveImage, UnsaveImage, LikeImage, DislikeImage, ListSavedImages
│   ├── export_import.go # Export/Import sources
│   └── health.go        # Simple health ping
├── service/
│   ├── engine.go        # Core: RandomImage, filterSources, weightedPick, extractImageURL, resolveSubEndpoint
│   ├── proxy.go         # ProxyCache: legacy disk cache (validation-only, kept for compat)
│   └── imstore.go       # ImageStore: download, cache, serve, save images
├── store/
│   ├── sqlite.go        # All DB operations: migration, CRUD, stats, images, history, system tags
│   └── session.go       # SessionManager: 内存会话 (Create/Validate 滑动续期/Delete/Clear)
├── middleware/
│   ├── adminauth.go     # 双通道鉴权: 登录会话 token 或旧版 admin_token (login_enabled 时)
│   ├── ratelimit.go     # In-memory per-IP rate limiter
│   ├── cors.go          # CORS middleware
│   ├── accesslog.go     # Request logging (跳过静态资源/首页/前端轮询接口)
│   ├── servertime.go    # 响应头 X-Server-Time (前端时差检测)
│   ├── requestid.go     # X-Request-Id 注入
│   └── recover.go       # PanicRecover 堆栈记录
└── model/
    ├── source.go        # Source, Tag, Settings, QueryParam, CachedImage, Stats/History types
    └── common.go        # Generic response types
dashboard/
├── src/
│   ├── views/
│   │   ├── EndpointsView.vue    # API Endpoints & System Tag Box
│   │   ├── SourcesView.vue      # Sources management & Branch variants
│   │   ├── SettingsView.vue     # 标签页布局: 网络代理/本地缓存/限流历史/安全鉴权(登录保护+Admin Token)
│   │   ├── StatsView.vue        # History log, heart save persistence & weight controls
│   │   ├── SavedView.vue        # Saved Gallery: List / Small Grid / Large Infinite Scroll Grid
│   │   ├── HealthCheckView.vue  # Health status overview
│   │   └── LoginView.vue        # 登录页 (用户名+密码, 记住用户名, 首次运行设置模式)
│   ├── composables/
│   │   ├── useApi.ts            # 统一 API 层: 401 自动登出/跳转, 时差检测, login/logout
│   │   └── useTheme.ts          # 主题骨架 (useColorMode)
│   ├── components/
│   │   ├── SourceForm.vue
│   │   ├── SourceCard.vue
│   │   └── ParamVariantsModal.vue
│   └── ...
```

## Authentication & Session (登录鉴权)

### 设计
- **Settings 字段**: `login_enabled` / `admin_username` / `admin_password_hash` (json 永不输出) / `session_hours` (默认 3h)
- **密码**: 前端 crypto-js 计算 MD5 后传输，后端存储 MD5 摘要（自部署工具定位，公网建议 HTTPS）
- **会话**: `store/session.go` 内存 map，token 为 crypto/rand 32 字节 hex；每次请求校验通过即滑动续期；服务重启全部失效
- **鉴权中间件** (`adminauth.go`) 双通道:
  - `login_enabled=true`: 校验 session token 或旧版 admin_token（兼容）；均无效返回 401
  - **首次初始化**: 登录已启用但未配置账号密码 → 写操作临时放行（允许设置凭据），配置后立即生效
  - `login_enabled=false`: 维持旧逻辑（admin_token 校验 / 无配置放行）
- **防爆破**: 登录失败随机延迟 500~2000ms
- **凭据变更**: 修改密码/用户名/登录开关 → `SessionManager.Clear()` 全量踢下线
- **前端**: token 存 `useLocalStorage('pichub_admin_token')`；`router.beforeEach` 全局守卫（30s 缓存登录态，受保护页面未登录一律重定向 `/login`，已登录访问 `/login` 回主页）；`useApi` 收到 401 → 清 token + 跳登录页
- **无布局路由**: `/login` 标记 `meta.bare`，`App.vue` 按 `route.meta.bare` 条件渲染（裸页面全屏无侧边栏/顶栏）
- **首次运行**: 登录页检测 `login_enabled && !admin_username` → 显示设置表单（用户名/密码/确认），保存后自动登录进入控制台
- **时差检测**: 所有响应带 `X-Server-Time` 头，前端偏差 >30 分钟 console.warn

### 请求链路（中间件顺序）
```
PanicRecover → RequestID → AccessLog → CORS → ServerTime
/api/login 不挂 AdminAuth; /api/logout 挂 AdminAuth
```

## Request Lifecycle: GET /random

### 1. Handler (handler/random.go)
```
GET /random?category=horizontal&format=json&orientation=vertical
```
- Reads `?category=`, `?format=`, `?orientation=`, `User-Agent` header
- Falls back to `settings.BoundTags` if no category provided
- Calls `engine.RandomImage(category, format, orientation, clientUA)`
- If `result.LocalURL` is set (cache mode), redirects to that instead of original URL

### 2. Engine (service/engine.go)
Stages:
1. **Load settings & sources** from DB
2. **filterSources()**
   - Exclude: disabled, status=="error"
   - Match categories requested vs source categories
   - Exclusive Tag Filter: `exclusive: true` tags (e.g. `nsfw`) are skipped unless explicitly requested in `?category=`
   - Expand `Params` sub-API links & query variants via `resolveSubEndpoint(mainURL, key, val)`
   - Append `DefaultQuery` to source URLs
3. **weightedPick()** — weighted random selection, up to 3 retries (8 if orientation filter active)
4. **HTTP fetch** selected source URL
   - Timeout from settings (default 3000ms)
   - HTTP Proxy forwarding if `proxy_enabled=true` via `proxy_url`
   - Custom headers from source config
   - Forward client User-Agent if source has none configured
5. **Failure handling**
   - On error/5xx: `monitor.Emit(sourceID, false)`; on success `monitor.Emit(sourceID, true)`
   - `SourceMonitor` 滑动窗口(20次)统计, 成功率 <30% → status="error"; 每 30 分钟恢复 error 源
6. **extractImageURL()** — see "URL Extraction" below
7. **ImageStore download** (only if `proxy_mode=true`)
   - `imageStore.DownloadAndStore(imageURL)`
   - Downloads image, checks MinResolution, decodes dimensions
   - If orientation filter active: checks actual image dimensions, retries if mismatch
   - Saves to `./data/images/{uuid}.{ext}`, writes metadata to `images` SQLite table
   - Returns `CachedImageInfo{ID, FileID, Width, Height, Format}`
8. **RecordStats async** `go e.store.RecordStats(queryCats, selected, url, &imageID, fileID)`
9. **Return Result{URL, LocalURL, SourceName, FileID, Width, Height, Format, ImageID}`

### 3. Sub-API Endpoint Resolution: resolveSubEndpoint()
- Full URLs (starts with `http://` or `https://`): Used directly as target URL.
- Relative sub-paths (starts with `/` or ends with `.php`/`.json` without `=`): Resolved against primary URL host using `url.ResolveReference`.
- Standard Query Params (e.g., `key=type`, `val=pc`): Appended via URL query building.

## Saved Gallery Architecture (`SavedView.vue`)
- **View Modes**:
  1. `list`: Tabular view with standard page controls.
  2. `grid_small`: Compact 4-5 column grid with detailed metadata tags.
  3. `grid_large`: Borderless 2-3 column sharp natural aspect ratio photo mosaic wall.
- **Infinite Scroll & Lazy Loading**:
  - `grid_large` mode disables traditional pagination bars.
  - Implements `IntersectionObserver` on sentinel element (`sentinelRef`).
  - Automatically fetches and appends next batch of items (`offset = images.length`) upon scrolling into view.
  - Native `loading="lazy"` decoding on all image elements.
- **Local File Download**:
  - One-click native browser file download via `downloadImage(img)` using `/images/${file_id}`.

## Custom Endpoints (/e/:name)
- **数据**: `endpoints` 表 (`name` UNIQUE, `bound_tags`, `enabled`), CRUD 在 `handler/endpoint.go`, 名称校验 `^[a-z0-9-]+$`。
- **分发**: `handler/random.go` 抽出 `serveRandom(c, category)` 供 `/random` 与 `/e/:name` 共用; 端点 `?category` 为空时 fallback 到自身 `bound_tags`, 不存在/禁用返回 404。
- **共享**: 与 `/random` 共用同一个 `RateLimit` 实例 (`rateLimitMW`)、同一分发池与统计。
- **前端**: `EndpointsView.vue`「自定义分发端点」区块, 行模式添加草稿→创建; 已保存端点绑定标签勾选即时保存; 绑定 UI 复用 `EndpointTagBinding.vue` (主接口与端点共用)。

## Distribution Pool & Cache Separation
- **池对用户不可见**: `pool_size` 不再暴露于设置 UI, 池参数退回内部默认 (DB 保留, 仅 API 可调)。
- **自适应单源额度** (`service/engine.go` `sourceCaps`): 按源近期被选中热度分配池额度 (夹在 `minPoolPerSource=1` ~ `maxPoolPerSource=8`), 替代旧的硬编码 `< 5`; 叠加 `maxFetchPerTick=8` 每轮拉取上限, 防止突发打爆图源触发风控。热度由 `SourceDemandTracker` (5 分钟窗口) 记录。
- **Exclusive 消费隔离**: 池中条目若携带 exclusive 标签, 仅显式 `?category=` / BoundTags 注入的请求 (`PopMatching`) 可消费; `PopAny` / `PopByOrientation` 会跳过这类条目。通用兜底预取 (`fetchSingleForTag("")`) 排除含 exclusive 标签的源。exclusive 内容仍按需求小额度预取, 保留显式请求的秒级响应。
- **池图与历史分离**: `images.pooled` 列标记"未分发预取池"图片 (池预取写 `1`, 分发命中置 `0` 并记入 `image_history`)。`image_history` 只含已分发流水。淘汰优先级: `pooled=1` 最旧 → 未保护的已分发最旧; `is_saved=1` 与历史最近 `MaxHistoryRecords` 条永不淘汰。启动时 `CleanupOrphanPooled` 清理磁盘文件缺失的孤儿池记录。
- **缓存淘汰覆盖子目录**: `service/cacheutil.go` 统一收集根目录 + 按源子目录文件, 修复旧逻辑只淘汰根目录导致池子目录无限增长的缺陷。

## Security Notes
- `admin_token` (旧版静态令牌) 仅写入, `GET /api/settings` 不再回显; `PUT /api/settings` 响应在鉴权后回传。源 Headers 中的敏感项 (`api-key/authorization/token/secret/cookie`) 在导出备份时被过滤。

## API Endpoints (main.go)

### Public (no auth)
| Method | Path | Handler | Notes |
|---|---|---|---|
| GET | /ping | HealthCheck | `{"status":"ok"}` |
| GET | /random | RandomImage | Core endpoint, supports `?category=&format=&orientation=` |
| GET | /e/:name | EndpointImage | Custom endpoint, fully equivalent to /random (fallback to endpoint's own bound tags) |
| POST | /random/detect | DetectURL | Analyze URL response type |
| POST | /api/sources/health-check | BatchHealthCheck | Health-check all sources |
| GET | /api/sources | ListSources | |
| GET | /api/sources/:id | GetSource | |
| GET | /api/endpoints | ListEndpoints | Custom distribution endpoints (public read) |
| GET | /api/settings | GetSettings | |
| GET | /api/tags | GetTags | |
| GET | /api/health | GetHealthStatus | Last health check results |
| GET | /api/stats | GetStats | `?range=today/7d/30d/all&start_date=&end_date=` |
| GET | /api/stats/history | GetImageHistory | `?limit=&offset=` (includes `is_saved` boolean status) |
| GET | /api/images/saved | ListSavedImages | Saved images gallery, `?limit=&offset=` |
| GET | /images/:file_id | ServeImage | Serve cached image file (binary) |

### Admin (requires `Authorization: Bearer <token>` — 登录会话或 admin_token)
| Method | Path | Handler |
|---|---|---|
| POST | /api/login | Login (公开, 无鉴权) |
| POST | /api/logout | Logout |
| POST | /api/sources | CreateSource |
| PUT | /api/sources/:id | UpdateSource |
| DELETE | /api/sources/:id | DeleteSource |
| POST | /api/sources/:id/toggle | ToggleSource |
| PUT | /api/settings | UpdateSettings |
| PUT | /api/tags | UpdateTags |
| POST | /api/endpoints | CreateEndpoint |
| PUT | /api/endpoints/:id | UpdateEndpoint |
| DELETE | /api/endpoints/:id | DeleteEndpoint |
| POST | /api/endpoints/:id/toggle | ToggleEndpoint |
| POST | /api/images/:id/save | SaveImage |
| DELETE | /api/images/:id/save | UnsaveImage |
| POST | /api/images/:id/like | LikeImage |
| POST | /api/images/:id/dislike | DislikeImage |
| POST | /api/export | ExportRules |
| POST | /api/import | ImportRules |

## Data Models (model/source.go)

### Source
| Field | Type | DB Column | Notes |
|---|---|---|---|
| ID | int64 | id PK AUTOINCREMENT | |
| Name | string | name | Default: `"{host} Source"` |
| URL | string | url | API endpoint |
| RespType | string | resp_type | "json"/"redirect"/"" |
| JsonPath | string | json_path | gjson path, default "url" |
| Weight | int | weight | 10-90, default 50 |
| Categories | []string | categories JSON TEXT | Tags, default [] |
| Headers | map[string]string | headers JSON TEXT | Custom request headers |
| Params | []QueryParam | params JSON TEXT | Query param & sub-API link variants |
| DefaultQuery | string | default_query | Appended query string |
| Enabled | bool | enabled INT | Default true |

### Tag
| Field | Type | Notes |
|---|---|---|
| ID | string | e.g. "horizontal" |
| Name | string | e.g. "横屏" |
| System | bool | `true` for system-only rules (`horizontal/vertical/adaptive`) |
| Exclusive | bool | `true` for exclusive tags (e.g. `nsfw`) requiring explicit opt-in |

### Settings
| Field | Type | Default | Notes |
|---|---|---|---|
| ProxyMode | bool | false | Enables full local caching (download + serve) |
| ProxyEnabled | bool | false | Enable HTTP proxy for outgoing requests |
| ProxyURL | string | "http://127.0.0.1:7890" | HTTP proxy address |
| PoolSize | int | 20 | 分发池总容量 (内部默认, UI 不暴露) |
| CacheMaxMB | int | 200 | Max cache size in MB (secondary eviction) |
| CacheMaxImages | int | 60 | Max cached image count (primary eviction) |
| MinResolution | string | "1920x1080" | Min resolution; "0" disables filter |
| RateLimit | int | 60 | Requests per minute per IP |
| RateLimitWindow | int | 60 | Rate limit sliding window (seconds) |
| Timeout | int | 3000 | HTTP timeout in ms |
| AdminToken | string | "" | Legacy auth token for admin endpoints (仍兼容, 仅写入不回显) |
| SavedImagesDir | string | "./data/saved" | Directory for saved images copy |
| LoginEnabled | bool | false | 启用用户名+密码登录保护 |
| AdminUsername | string | "" | 登录用户名 |
| AdminPasswordHash | string | "" | 登录密码 MD5 (json "-" 永不输出) |
| AdminPassword | string | "" | 仅写入: 新密码 MD5, 留空不修改 |
| SessionHours | int | 3 | 会话有效期 (小时), 滑动续期 |
