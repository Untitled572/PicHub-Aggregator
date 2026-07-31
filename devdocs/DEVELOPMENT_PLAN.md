# PicHub-Aggregator 详细开发计划

> 基于 PRD v0.1 | 2026-07-27  
> 技术选型：Go(Gin) + Vue3/TS/Tailwind + Cloudflare Workers/KV

---

## 一、项目结构

```
PicHub/
├── backend/                    # Go 后端
│   ├── main.go                 # 入口
│   ├── go.mod / go.sum
│   ├── config/
│   │   ├── config.go           # 配置结构体与加载
│   │   └── config.json         # 运行时配置
│   ├── handler/
│   │   ├── random.go           # /random 分发 + 智能识别
│   │   ├── source.go           # API 源 CRUD（含单个获取）
│   │   ├── health.go           # 服务健康 + 批量健康检测
│   │   ├── settings.go         # 系统设置
│   │   └── community.go        # 规则导入导出（JSON）
│   ├── service/
│   │   ├── engine.go           # 分发引擎（权重+容错）
│   │   ├── detector.go         # 智能识别引擎
│   │   ├── proxy.go            # 代理/缓存模式
│   │   └── healthcheck.go      # 健康检测定时任务
│   ├── model/
│   │   └── source.go           # API 源数据模型
│   ├── store/
│   │   └── sqlite.go           # SQLite 持久化层
│   ├── middleware/
│   │   └── ratelimit.go        # 速率限制中间件
│   ├── embed/                  # 编译时嵌入的前端资源
│   │   └── embed.go
│   ├── Dockerfile
│   └── docker-compose.yml
│
├── dashboard/                  # Vue3 Dashboard 前端
│   ├── package.json
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   ├── tsconfig.json
│   ├── index.html
│   ├── public/
│   └── src/
│       ├── main.ts
│       ├── App.vue
│       ├── router/index.ts
│       ├── views/
│       │   ├── SourcesView.vue           # API 源列表管理
│       │   ├── SmartDetectView.vue        # 智能识别添加
│       │   ├── HealthCheckView.vue        # 健康检测面板
│       │   └── SettingsView.vue           # 系统设置
│       ├── components/
│       │   ├── SourceCard.vue             # API 源卡片
│       │   ├── SourceForm.vue             # 添加/编辑表单
│       │   ├── JsonTreeSelector.vue       # JSON 树状选择器
│       │   ├── HealthStatusBadge.vue      # 健康状态标签
│       │   └── ExportImportModal.vue      # 导出/导入弹窗
│       ├── composables/
│       │   ├── useApi.ts                  # API 请求封装
│       │   └── useHealthCheck.ts          # 健康检测逻辑
│       └── types/
│           └── index.ts                   # TypeScript 类型定义
│
├── community/                  # 社区 Hub（Serverless）
│   ├── frontend/               # 静态站点，部署到 GitHub Pages / Vercel
│   │   ├── package.json
│   │   ├── vite.config.ts
│   │   ├── tailwind.config.js
│   │   ├── index.html
│   │   └── src/
│   │       ├── main.ts
│   │       ├── App.vue
│   │       ├── router/index.ts
│   │       ├── views/
│   │       │   ├── SquareView.vue         # 图源广场
│   │       │   └── SubmitView.vue         # 提交新规则
│   │       ├── components/
│   │       │   ├── RuleCard.vue           # 规则卡片
│   │       │   ├── RuleFilter.vue         # 分类筛选器
│   │       │   ├── CommentSection.vue     # 评论区
│   │       │   └── SubmitForm.vue         # 提交表单
│   │       ├── composables/
│   │       │   └── useCommunityApi.ts     # API 调用
│   │       └── types/
│   │           └── index.ts
│   └── worker/                 # Cloudflare Worker 后端
│       ├── wrangler.toml       # Worker 配置 + KV 绑定
│       └── src/
│           └── index.ts        # Worker 入口
│
└── docs/
    ├── README.md               # 项目总览 + 一键启动
    ├── API_DOC.md              # API 接口文档
    └── COMMUNITY_DEPLOY.md     # 社区 Hub 部署指南
```

---

## 二、后端详细设计 (Go + Gin)

### 2.1 入口与启动

`main.go` 启动时：
- 从环境变量 `PORT` 读取监听端口（默认 `5721`）
- 初始化 SQLite 连接（`DB_PATH`，默认 `./data/pichub.db`）
- 注册路由、中间件（速率限制、CORS）
- 挂载 `//go:embed` 的 Dashboard 静态资源
- 启动健康检测定时任务（每 5 分钟自动检测一次）

### 2.2 数据模型

```go
type Source struct {
    ID          int64     `json:"id"`
    Name        string    `json:"name"`         // 显示名称
    URL         string    `json:"url"`           // API 地址
    RespType    string    `json:"resp_type"`     // "image" | "redirect" | "json"
    JsonPath    string    `json:"json_path"`     // JSON 提取路径，如 "data.url"
    Weight      int       `json:"weight"`        // 权重 1-100
    Categories  []string  `json:"categories"`    // 多标签 ["avatar","anime"]
    Headers     map[string]string `json:"headers"` // 自定义请求头
    Enabled     bool      `json:"enabled"`       // 启用/禁用
    FailCount   int       `json:"fail_count"`    // 连续失败次数
    SuccessRate float64   `json:"success_rate"`  // 成功率
    AvgLatency  int64     `json:"avg_latency"`   // 平均延迟 ms
    Status      string    `json:"status"`        // "normal" | "warning" | "error"
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type Settings struct {
    ProxyMode     bool    `json:"proxy_mode"`      // 代理/缓存模式
    CacheMaxMB    int     `json:"cache_max_mb"`     // 最大缓存容量
    CacheTTL      int     `json:"cache_ttl"`        // 缓存过期时间（分钟）
    MinResolution string  `json:"min_resolution"`   // 最小分辨率
    RateLimit     int     `json:"rate_limit"`       // 全局速率限制 req/min
    Timeout       int     `json:"timeout"`          // 请求超时 ms
}
```

### 2.3 API 路由设计

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 服务健康检查 |
| GET | `/random` | 统一分发接口（主） |
| POST | `/random/detect` | 智能识别探测（Dashboard 调用） |
| GET | `/api/sources` | 获取所有源 |
| GET | `/api/sources/:id` | 获取单个源 |
| POST | `/api/sources` | 添加源 |
| PUT | `/api/sources/:id` | 更新源 |
| DELETE | `/api/sources/:id` | 删除源 |
| POST | `/api/sources/:id/toggle` | 启用/禁用 |
| POST | `/api/sources/health-check` | 批量健康检测 |
| GET | `/api/settings` | 获取系统设置 |
| PUT | `/api/settings` | 更新系统设置 |
| POST | `/api/export` | 导出规则（JSON） |
| POST | `/api/import` | 导入规则（JSON） |

### 2.4 分发引擎核心逻辑

```
/random?category=avatar&format=json

1. 筛选候选源：category 匹配 + Enabled + Status != "error"
2. 按权重随机抽取一个候选源
3. 发送 HTTP 请求至候选源的 URL
4. 解析响应：
   - 302 重定向 → 提取 Location 头 → 返回 302
   - 图片流 → 提取 Content-Type → 返回 302（原 URL）
   - JSON → 按 json_path 提取 → 返回 302
5. 失败处理：
   - 超时/50x → 标记失败 +1，从候选池移除，重试（最多3次）
   - 连续失败 5 次 → 自动挂起，标记 status="error"
6. 响应：
   - 默认：HTTP 302 + Location 头
   - ?format=json：{ "url": "...", "source": "...", "categories": ["..."] }
```

### 2.5 智能识别引擎

```
POST /random/detect  { "url": "https://api.example.com/image" }

1. 发送 HEAD/GET 请求到目标 URL
2. 检查 Response Headers:
   - Content-Type: image/* → resp_type = "image"
   - Location 头存在 → resp_type = "redirect"
   - Content-Type: application/json → resp_type = "json"
3. 若为 JSON：
   - 解析 body，返回 JSON 树结构
   - 标记所有值为 URL 的叶子节点
4. 返回：{ "resp_type": "json", "headers": {...}, "body_tree": {...}, "url_hints": [...] }
```

### 2.6 代理/缓存模式与分辨率过滤

代理模式下，分发引擎下载第三方图片至本地缓存后返回。分辨率过滤逻辑：
- 使用 Go 标准库 `image.DecodeConfig` 读取图片宽高（仅解码头部元信息，开销极低）
- 若宽高均低于 `MinResolution` 设定值（如 "800x600"），跳过该源并重试下一个

### 2.7 导出敏感信息过滤

导出规则时自动过滤以下 Header 的 key（大小写不敏感）：
`api-key`、`authorization`、`token`、`secret`、`cookie`
前端提示用户手动补充被过滤的敏感字段。导入时若 Header 值为空，不覆盖已有值。

### 2.8 SQLite 存储设计

```sql
CREATE TABLE sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    resp_type TEXT DEFAULT 'json',
    json_path TEXT DEFAULT '',
    weight INTEGER DEFAULT 10,
    categories TEXT DEFAULT '[]',       -- JSON array
    headers TEXT DEFAULT '{}',          -- JSON object
    enabled INTEGER DEFAULT 1,
    fail_count INTEGER DEFAULT 0,
    success_rate REAL DEFAULT 100.0,
    avg_latency INTEGER DEFAULT 0,
    status TEXT DEFAULT 'normal',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

---

## 三、Dashboard 前端详细设计 (Vue3)

### 3.1 路由

| 路径 | 视图 | 说明 |
|------|------|------|
| `/` | SourcesView | 默认页，API 源列表 |
| `/sources` | SourcesView | 源管理列表 |
| `/detect` | SmartDetectView | 智能识别添加 |
| `/health` | HealthCheckView | 健康检测面板 |
| `/settings` | SettingsView | 系统设置 |

### 3.2 页面功能详述

#### SourcesView（源列表管理）
- 表格/卡片列表，每行显示：名称、域名、类型、分类标签、权重、成功率、延迟、状态开关
- 操作按钮：编辑、删除、启用/禁用、健康检测（单个）
- 顶部操作栏：搜索、分类筛选、添加按钮、一键健康检测、导出/导入
- 状态颜色：绿色=正常、黄色=警告（连续失败 2-4 次）、红色=异常（连续失败 ≥5）

#### SmartDetectView（智能识别）
- URL 输入框 + "智能识别"按钮
- 识别结果展示区：
  - 非 JSON：显示类型、响应头信息，自动填入表单
  - JSON：左侧 JSON 树视图，点击叶子节点自动填入 json_path
- 底部配置表单自动填入识别结果，用户补充：名称、分类、权重、请求头

#### HealthCheckView（健康检测）
- 批量检测按钮 + 进度条
- 结果列表：每个源显示延迟、HTTP 状态码、可用性
- 成功/失败统计汇总

#### SettingsView（系统设置）
- 中转模式开关（轻量/代理缓存）
- 缓存配置（最大容量、过期时间）
- 速率限制配置
- 请求超时配置
- 保存按钮

### 3.3 构建与嵌入

Dashboard 构建后输出至 `dashboard/dist/`，Go 后端在编译时通过 `//go:embed` 将 `dashboard/dist/*` 嵌入二进制。Gin 在路由中挂载静态文件服务。

---

> ⚠️ **社区 Hub（Phase 8）暂缓开发。** 以下为已知问题清单，恢复开发时应优先处理。

## 四、社区 Hub 详细设计（暂缓）

### 4.1 社区前端（静态站点）

- **SquareView（图源广场）**：卡片网格布局，展示所有公开 API 规则。每个卡片包含：名称、描述、分类标签、URL、点赞数、评论数。支持按分类筛选、按热门排序。
- **SubmitView（提交规则）**：可视化表单，提交新 API 规则（名称、URL、分类、响应类型、JSON 路径等）。支持公开/私密选项。
- **RuleCard 组件**：展示规则详情，包含点赞/点踩按钮、"一键复制规则码"按钮、"一键导入"（复制 JSON）。
- **CommentSection 组件**：评论区，支持发表评论。

### 4.2 Cloudflare Worker 后端

```typescript
// KV Namespaces
// RULES_KV:  存储 API 规则（key: rule:{rule_id}, value: JSON）
// VOTES_KV:  存储点赞/点踩（key: vote:{rule_id}, value: {up: n, down: n}）
// COMMENTS_KV: 每条评论独立存储（key: comment:{rule_id}:{ts}:{uuid}, value: {author, content, time}）
// COMMENTS_LIST_KV: 规则评论索引（key: comments_index:{rule_id}, value: [comment_key,...]）
// RATELIMIT_KV: IP 速率限制计数器

// API 路由（所有响应带 CORS 头 Access-Control-Allow-Origin: *）
OPTIONS /api/*                  # CORS 预检
GET    /api/rules               # 获取规则列表（支持 ?category= & ?sort=）
POST   /api/rules               # 提交新规则（需通过 Turnstile 验证）
POST   /api/rules/:id/vote      # 点赞/点踩 { type: "up"|"down" }（Turnstile）
POST   /api/rules/:id/comments  # 发表评论 { author, content, turnstile_token }
GET    /api/rules/:id/comments  # 获取评论（prefix scan comment:{id}:* 按时间排序）
```

**安全防护**：
- 提交规则、点赞/点踩、发表评论均需传入 Cloudflare Turnstile token 验证
- 同一 IP 每分钟最多 10 次写操作（POST/PUT），通过 KV 计数实现简易速率限制

**评论存储**：每条评论为独立 KV key `comment:{rule_id}:{timestamp}:{uuid}`，读取时用 `list({prefix: "comment:{rule_id}:"})` 按时间排序分页。避免单 key 并发写覆盖问题。

### 4.3 部署流程（暂缓）

1. 社区前端：`npm run build` → 部署至 GitHub Pages / Vercel
2. Worker 后端：`npx wrangler deploy` → Cloudflare Workers
3. KV 命名空间创建 + 绑定配置在 `wrangler.toml`

### 4.4 已知问题（恢复开发时优先处理）

| # | 问题 | 严重度 |
|---|------|--------|
| 1 | Turnstile 未集成：前端硬编码 `placeholder` token，生产环境配置密钥后所有写操作将失败 | 高 |
| 2 | `VOTES_KV` 死代码：wrangler.toml 绑定但 Worker 未使用；投票数据内联在 RULES_KV 中 | 中 |
| 3 | 投票无 IP 去重：同一 IP 可无限刷票 | 高 |
| 4 | `comment_count` 竞态：read-modify-write 无锁，并发评论可能丢计数 | 中 |
| 5 | 分类不一致：SquareView 有 8 个分类，SubmitForm 只有 6 个（缺横屏/竖屏） | 低 |
| 6 | 评论排序升序：最旧排最前，与主流社区习惯相反 | 低 |

---

## 五、Docker 部署设计

### Dockerfile（多阶段构建）

```dockerfile
# Stage 1: 构建 Dashboard 前端
FROM node:20-alpine AS dashboard-builder
WORKDIR /dashboard
COPY dashboard/package*.json ./
RUN npm ci
COPY dashboard/ ./
RUN npm run build

# Stage 2: 构建 Go 后端
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app
COPY backend/go.* ./
RUN go mod download
COPY backend/ ./
COPY --from=dashboard-builder /dashboard/dist ./embed/dist
RUN CGO_ENABLED=0 go build -o pichub .

# Stage 3: 运行镜像
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend-builder /app/pichub .
COPY backend/config.json .
EXPOSE 5721
CMD ["./pichub"]
```

### docker-compose.yml

```yaml
services:
  pichub:
    build:
      context: .
      dockerfile: backend/Dockerfile
    ports:
      - "5721:5721"
    volumes:
      - ./data:/app/data          # SQLite 持久化
      - ./cache:/app/cache        # 图片缓存（代理模式）
    environment:
      - PORT=5721
      - DB_PATH=/app/data/pichub.db
      - CACHE_PATH=/app/cache
```

---

## 六、开发顺序

| 阶段 | 内容 | 预估 |
|------|------|------|
| **Phase 1** | Go 后端骨架 + 数据层（model/store/config） | Step 1-2 |
| **Phase 2** | API 源 CRUD 接口 + Settings 接口 | Step 3 |
| **Phase 3** | 分发引擎（/random + 权重 + 容错） | Step 4 |
| **Phase 4** | 智能识别引擎 | Step 5 |
| **Phase 5** | 健康检测 + 速率限制 + 代理模式 | Step 6 |
| **Phase 6** | Dashboard 前端全部页面 | Step 7-8 |
| **Phase 7** | 前端构建嵌入 + Docker 完整打包 | Step 9 |
| **Phase 8** | 社区 Hub 前端 + Worker 后端 | Step 10-11 | ⚠️ 暂缓 |
| **Phase 9** | 文档编写 | Step 12 |

---

## 七、关键技术决策记录

| 决策项 | 选择 | 理由 |
|--------|------|------|
| 前端框架 | Vue 3 + TypeScript + Tailwind | 用户指定 |
| Dashboard 部署 | go:embed 嵌入二进制 | PRD 要求无需额外 Nginx |
| 社区后端 | Cloudflare Workers + KV | 用户指定，Serverless 零成本 |
| 数据存储 | SQLite | 轻量嵌入式，无需外部数据库 |
| 项目结构 | 独立子目录 | 用户指定 backend/dashboard/community/ |
