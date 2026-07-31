# PicHub 图片处理流程完整梳理

## 整体架构

```
客户端 → [中间件层] → [Handler] → [Engine] → [上游API] → 响应解析 → 返回客户端
```

---

## 一、请求生命周期

### 1.1 中间件链（按顺序执行）

```
请求进入
  │
  ▼
gin.Recovery()          ← panic 恢复
  │
  ▼
AccessLog()             ← 访问日志（记录到 data/logs/pichub.log）
  │
  ▼
CORS()                  ← 跨域头设置（Access-Control-Allow-Origin: *）
  │
  ▼
RateLimit(st)           ← 仅对 GET /random 生效（其他路由跳过）
  │
  ▼
Handler.RandomImage()   ← 业务逻辑
```

### 1.2 速率限制细节

- **作用范围：** 仅 `GET /random` 路由
- **算法：** 滑动窗口计数（1 分钟窗口）
- **数据结构：** `map[string][]time.Time`（IP → 请求时间戳列表）
- **限额来源：** SQLite settings 表 `rate_limit` 字段（默认 60 次/分钟）
- **反代支持：** 通过 `TRUSTED_PROXIES` 环境变量配置，从 `X-Forwarded-For` / `X-Real-IP` 读取真实 IP
- **超限响应：** `429 Too Many Requests`
- **自动清理：** 后台 goroutine 每 10 分钟清理超过 5 分钟的旧记录

---

## 二、Handler 层（`handler/random.go`）

### 2.1 参数解析

```go
category := c.Query("category")    // 分类过滤，如 "horizontal,vertical"
format := c.Query("format")        // 返回格式，"json" 或 空
clientUA := c.GetHeader("User-Agent")  // 客户端 UA，透传给上游
```

### 2.2 Bound Tags 注入

当 `category` 参数为空时，从后端 settings 读取 `bound_tags`：

```go
if category == "" {
    settings, _ := h.store.GetSettings()
    if len(settings.BoundTags) > 0 {
        category = strings.Join(settings.BoundTags, ",")
    }
}
```

### 2.3 响应分发

```
Engine 返回 (result, statusCode, err)
  │
  ├─ err != nil → 503 {"error": "..."}
  │
  ├─ format == "json" → 200 JSON {"url", "source", "categories"}
  │
  ├─ statusCode == 302 → 302 重定向到 result.URL（图片）
  │
  └─ 其他 → 200 JSON
```

---

## 三、Engine 层（`service/engine.go`）

### 3.1 核心流程 `RandomImage()`

```
1. 加载 settings（获取 timeout、proxy_mode 等）
2. 加载所有 sources
3. filterSources() → 筛选候选源
4. 若无候选 → 返回 "no available sources"
5. 重试循环（最多 3 次）：
   a. weightedPick() → 按权重随机选择一个源
   b. 构建 HTTP 请求（设置 Headers、User-Agent）
   c. 发送请求（http.Client，不跟随 302）
   d. 处理错误/5xx → 移除源，重试
   e. extractImageURL() → 提取图片 URL
   f. 可选：proxyCache 预缓存
   g. 返回结果
6. 3 次全部失败 → 返回 "all sources failed"
```

### 3.2 User-Agent 透传

```go
// 如果源没有自定义 User-Agent，使用客户端的 UA
if clientUA != "" {
    hasUA := false
    for k := range selected.Headers {
        if strings.EqualFold(k, "User-Agent") {
            hasUA = true
            break
        }
    }
    if !hasUA {
        req.Header.Set("User-Agent", clientUA)
    }
}
```

**效果：** 移动端访问时，上游 API 收到移动端 UA → 返回竖屏图片；桌面端收到桌面 UA → 返回横屏图片。

### 3.3 HTTP 客户端配置

```go
client := &http.Client{
    Timeout: timeout,  // 从 settings 读取，默认 3000ms
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        return http.ErrUseLastResponse  // 不跟随 302，捕获响应
    },
}
```

### 3.4 失败处理与自动降级

```
请求失败 / 5xx 响应
  │
  ├─ IncrementFailCount(id)     ← fail_count + 1
  ├─ removeSource()             ← 从候选列表移除
  ├─ checkAndSuspend()          ← fail_count >= 5 → status = "error"
  │
  └─ 继续重试下一个候选源
```

---

## 四、源筛选逻辑（`filterSources`）

### 4.1 筛选流程

```
遍历所有 sources
  │
  ├─ 跳过: !enabled || status == "error"
  │
  ├─ 有 Params（子分支）:
  │   │
  │   ├─ 遍历每个 Param:
  │   │   ├─ 跳过: key 为空
  │   │   ├─ 构建 URL: buildURL(baseURL, key, val) + default_query
  │   │   ├─ 继承权重: param.Weight || src.Weight
  │   │   ├─ 继承 categories: param.Categories || src.Categories
  │   │   ├─ 过滤: param.Categories 为空 → 跳过过滤（始终可用）
  │   │   └─ 通过 → 加入候选
  │   │
  │   └─ 追加源基础 URL 作为兜底:
  │       ├─ src.Categories 为空 → 始终加入
  │       └─ src.Categories 不为空 → 按 category 过滤
  │
  └─ 无 Params:
      ├─ src.Categories 为空 → 始终加入
      └─ src.Categories 不为空 → 按 category 过滤
```

### 4.2 子分支（Params）过滤规则

| param.Categories | category 参数 | 结果 |
|---|---|---|
| 空 `[]` | 任意 | **始终可用**（跳过过滤） |
| 有值 | 空（全部绑定） | 可用（不做过滤） |
| 有值 | 匹配 | 可用 |
| 有值 | 不匹配 | 排除 |

### 4.3 兜底机制

当源有子分支时，即使所有子分支都被过滤，源的基础 URL 仍作为候选加入（前提是源自身的 categories 匹配或为空）。

---

## 五、图片 URL 提取（`extractImageURL`）

### 5.1 处理优先级

```
1. 3xx 重定向响应（301/302/303/307/308）
   │
   ├─ Location 头存在:
   │   ├─ 解析相对路径 → resolveURL()
   │   ├─ Baidu 特殊处理 → 提取 url 参数
   │   └─ 返回 Location URL
   │
   └─ Location 头为空:
       └─ 读取 body → 提取纯文本 URL → Baidu 特殊处理

2. Content-Type: image/*
   └─ 返回请求 URL（直接返回图片流）

3. Content-Type: application/json 或 resp_type == "json"
   └─ 读取 body → gjson 提取 → 返回 URL

4. 其他（text/plain、text/html 等）
   └─ 读取 body → 尝试提取纯文本 URL → Baidu 特殊处理

5. Fallback
   └─ 返回请求 URL
```

### 5.2 Baidu URL 特殊处理

`image.baidu.com/search/down?url=REAL_IMAGE_URL` 会返回 `Content-Disposition: attachment`（强制下载），需要提取 `url` 参数中的真实图片链接。

```go
func resolveBaiduURL(rawURL string) string {
    if !strings.Contains(rawURL, "image.baidu.com/search/down") {
        return rawURL
    }
    u, _ := url.Parse(rawURL)
    realURL := u.Query().Get("url")  // 提取 url 参数
    if realURL != "" {
        return realURL  // 返回 sinaimg 等真实图片链接
    }
    return rawURL
}
```

### 5.3 各 API 类型处理示例

| API 类型 | 响应特征 | 提取方式 |
|---|---|---|
| 图片直链 | `Content-Type: image/*` | 返回请求 URL |
| 302 重定向 | `302 + Location: image.jpg` | 提取 Location |
| JSON 提取 | `Content-Type: application/json` | gjson 提取 `json_path` |
| 纯文本 URL | `200 + text/plain: https://...` | body 提取 |
| Baidu 包装 | `https://image.baidu.com/search/down?url=...` | 提取 url 参数 |
| tangdouz 风格 | `200 + body: https://...` | body 提取 + Baidu 处理 |

---

## 六、URL 构建

### 6.1 buildURL 支持三种模式

```
模式 1: 路径替换
  key="/" 或 value 以 "/" 开头
  → baseURL + value
  例: https://api.com/v1 + /random → https://api.com/v1/random

模式 2: 原始参数串
  key 包含 "=" 或 key 为空
  → baseURL + ?key
  例: https://api.com + return=302&type=pc → https://api.com?return=302&type=pc

模式 3: 标准查询参数
  key=value 标准格式
  → 使用 url.Parse + Query().Set()
  例: https://api.com?type=pc&r18=1
```

### 6.2 default_query 追加

在 `buildURL` 之后追加源的 `default_query`：

```
https://api.com?type=pc + default_query=num=1
→ https://api.com?type=pc&num=1
```

---

## 七、加权随机选择（`weightedPick`）

```
总权重 = Σ(source.Weight)
随机数 r = rand.Intn(totalWeight)

遍历 sources:
  r -= src.Weight
  if r < 0 → 选中此源
```

权重来源（优先级从高到低）：
1. `param.Weight`（子分支权重）
2. `src.Weight`（源权重）
3. Settings 中配置的默认值

---

## 八、完整请求示例

### 示例 1：标准 JSON API

```
GET /random?format=json

→ handler: category="" → 读取 bound_tags → category="horizontal"
→ engine: filterSources → 选择匹配 horizontal 的源
→ weightedPick → 选中 Source A
→ GET https://api.source-a.com/v1/random?type=pc
→ 响应: {"url": "https://example.com/image.jpg"}
→ extractImageURL: JSON 提取 data.url → https://example.com/image.jpg
→ 返回: {"url": "https://example.com/image.jpg", "source": "Source A"}
```

### 示例 2：302 重定向 API

```
GET /random

→ handler: category="" → bound_tags → category="vertical"
→ engine: filterSources → 选中 Source B
→ GET https://api.source-b.com/pic.php
→ 响应: 302, Location: https://cdn.example.com/img.jpg
→ extractImageURL: 提取 Location → https://cdn.example.com/img.jpg
→ 返回: 302 重定向到 https://cdn.example.com/img.jpg
```

### 示例 3：tangdouz 风格（纯文本 URL）

```
GET /random

→ handler: category="" → bound_tags → category="horizontal,vertical"
→ engine: filterSources → 选中 Source C
→ GET https://api.tangdouz.com/sjdmbz.php
→ 响应: 200, body: https://image.baidu.com/search/down?url=https://tvax3.sinaimg.cn/...
→ extractImageURL: body 提取 → Baidu 处理 → https://tvax3.sinaimg.cn/...
→ 返回: 302 重定向到 https://tvax3.sinaimg.cn/...
```

### 示例 4：子分支过滤

```
Source D 有子分支:
  - r18=0, categories=[]（空）
  - r18=1, categories=["r18"]

GET /random?category=horizontal

→ filterSources:
  - r18=0: categories 空 → 跳过过滤 → 可用 ✓
  - r18=1: hasAnyCategory(["r18"], ["horizontal"]) = false → 排除
  - 兜底: src.categories 空 → 加入候选 ✓
→ 候选: [r18=0, 兜底基础URL]
→ weightedPick → 随机选择一个
```

---

## 九、数据流总结

```
┌─────────────────────────────────────────────────────────────┐
│                        客户端请求                            │
│                 GET /random?category=horizontal              │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Handler: 读取 query params → 注入 bound_tags → 调用 Engine  │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Engine: 加载 sources → filterSources → weightedPick         │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  HTTP Client: GET 上游 API → 捕获响应（不跟随 302）           │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  extractImageURL: 按优先级提取图片 URL                        │
│  1. 3xx + Location → 提取                                   │
│  2. image/* → 返回请求 URL                                   │
│  3. JSON → gjson 提取                                       │
│  4. 其他 → body 提取 + Baidu 处理                            │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  返回客户端:                                                 │
│  - format=json → JSON 响应                                   │
│  - 其他 → 302 重定向到图片 URL                                │
└─────────────────────────────────────────────────────────────┘
```

---

## 十、配置项速查

| 配置 | 存储位置 | 默认值 | 说明 |
|---|---|---|---|
| `rate_limit` | settings 表 | 60 | 次/分钟/IP |
| `timeout` | settings 表 | 3000 | 上游请求超时（ms） |
| `bound_tags` | settings 表 | `[]` | 主接口绑定的 tags |
| `tags` | settings 表 | 3 个默认 | 标签定义 |
| `TRUSTED_PROXIES` | 环境变量 | 空 | 反代 IP 段 |
| `PUID/PGID` | 环境变量 | 1000 | 运行用户 ID |
| `PORT` | 环境变量 | 5721 | 监听端口 |
| `DB_PATH` | 环境变量 | `./data/pichub.db` | 数据库路径 |
