# PicHub-Aggregator Architecture

> Updated: 2026-07-30. Update this file when making significant backend changes.

## Tech Stack
- **Backend**: Go (Gin framework) + SQLite (mattn/go-sqlite3, CGO required)
- **Frontend**: Vue 3 + TypeScript + Tailwind CSS (embedded via `embed`)
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
│   ├── settings.go      # Get/Update settings
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
│   └── sqlite.go        # All DB operations: migration, CRUD, stats, images, history, system tags
├── middleware/
│   ├── adminauth.go     # Optional Bearer token check for write operations
│   ├── ratelimit.go     # In-memory per-IP rate limiter
│   ├── cors.go          # CORS middleware
│   └── accesslog.go     # Request logging middleware
└── model/
    ├── source.go        # Source, Tag, Settings, QueryParam, CachedImage, Stats/History types
    └── common.go        # Generic response types
dashboard/
├── src/
│   ├── views/
│   │   ├── EndpointsView.vue    # API Endpoints & System Tag Box
│   │   ├── SourcesView.vue      # Sources management & Branch variants
│   │   ├── SettingsView.vue     # Morandi 4-card domain settings & HTTP Proxy
│   │   ├── StatsView.vue        # History log, heart save persistence & weight controls
│   │   ├── SavedView.vue        # Saved Gallery: List / Small Grid / Large Infinite Scroll Grid
│   │   └── HealthCheckView.vue  # Health status overview
│   ├── components/
│   │   ├── SourceForm.vue
│   │   ├── SourceCard.vue
│   │   └── ParamVariantsModal.vue
│   └── ...
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
   - Exclusive Tag Filter: `exclusive: true` tags (e.g. `r18`) are skipped unless explicitly requested in `?category=`
   - Expand `Params` sub-API links & query variants via `resolveSubEndpoint(mainURL, key, val)`
   - Append `DefaultQuery` to source URLs
3. **weightedPick()** — weighted random selection, up to 3 retries (8 if orientation filter active)
4. **HTTP fetch** selected source URL
   - Timeout from settings (default 3000ms)
   - HTTP Proxy forwarding if `proxy_enabled=true` via `proxy_url`
   - Custom headers from source config
   - Forward client User-Agent if source has none configured
5. **Failure handling**
   - On error/5xx: increment fail_count; if >=5, mark status="error"; retry
   - On success: reset fail_count
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

## API Endpoints (main.go)

### Public (no auth)
| Method | Path | Handler | Notes |
|---|---|---|---|
| GET | /ping | HealthCheck | `{"status":"ok"}` |
| GET | /random | RandomImage | Core endpoint, supports `?category=&format=&orientation=` |
| POST | /random/detect | DetectURL | Analyze URL response type |
| POST | /api/sources/health-check | BatchHealthCheck | Health-check all sources |
| GET | /api/sources | ListSources | |
| GET | /api/sources/:id | GetSource | |
| GET | /api/settings | GetSettings | |
| GET | /api/tags | GetTags | |
| GET | /api/health | GetHealthStatus | Last health check results |
| GET | /api/stats | GetStats | `?range=today/7d/30d/all&start_date=&end_date=` |
| GET | /api/stats/history | GetImageHistory | `?limit=&offset=` (includes `is_saved` boolean status) |
| GET | /api/images/saved | ListSavedImages | Saved images gallery, `?limit=&offset=` |
| GET | /images/:file_id | ServeImage | Serve cached image file (binary) |

### Admin (requires `Authorization: Bearer <admin_token>`)
| Method | Path | Handler |
|---|---|---|
| POST | /api/sources | CreateSource |
| PUT | /api/sources/:id | UpdateSource |
| DELETE | /api/sources/:id | DeleteSource |
| POST | /api/sources/:id/toggle | ToggleSource |
| PUT | /api/settings | UpdateSettings |
| PUT | /api/tags | UpdateTags |
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
| Exclusive | bool | `true` for exclusive tags (e.g. `r18`) requiring explicit opt-in |

### Settings
| Field | Type | Default | Notes |
|---|---|---|---|
| ProxyMode | bool | false | Enables full local caching (download + serve) |
| ProxyEnabled | bool | false | Enable HTTP proxy for outgoing requests |
| ProxyURL | string | "http://127.0.0.1:7890" | HTTP proxy address |
| PoolSize | int | 10 | 0ms dispatch pool size |
| CacheMaxMB | int | 200 | Max cache size in MB (secondary eviction) |
| CacheMaxImages | int | 60 | Max cached image count (primary eviction) |
| MinResolution | string | "1920x1080" | Min resolution; "0" disables filter |
| RateLimit | int | 60 | Requests per minute per IP |
| Timeout | int | 3000 | HTTP timeout in ms |
| AdminToken | string | "" | Auth token for admin endpoints |
| SavedImagesDir | string | "./data/saved" | Directory for saved images copy |
