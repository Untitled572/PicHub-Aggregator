# 📄 PicHub-Aggregator 产品需求文档 (PRD v0.1)

## TODO

- ~~add: 重复添加检测~~
- ~~fix: 只有randomAPI计入限额~~
- ~~add：https://api.tangdouz.com/sjdmbz.php 302返回连接~~
- fix: ~~tags逻辑 不勾选=匹配隐藏tags，确保勾选其余tags时不会选中未勾选的源~~
- ~~add: 使用统计~~
- ~~add：本地缓存~~
- ~~fix: **本地图片和展示图片不一致**~~
- ~~后台始终获取全部图片，根据源api给图片打tag，若等待n-1轮没用上则直接删除
  获取到的图片移动到历史缓存~~
- 图片本地缓存记录：源，分类，分辨率，横/竖
- json返回 本地连接/分类/源链接
  - ~~用户发送请求~~
  - ~~是否包含指定分类~~
  - ~~是，直接返回包含指定分类的图片~~
  - ~~否，是否包含ua~~
    - ~~是，根据ua返回包含横屏/竖屏图片 横屏竖屏设置为不可更改tags~~
    - ~~否，返回任意图片~~
- ~~add:  判断横竖屏返回~~
- ~~动态权重计算~~
- ~~代理服务器~~
- ~~保存图片界面列表/小图/大图展示，下载到本地功能~~，批量下载
- ~~手机ui~~

- ~~fix: 更改到json后需要手动配置识别点~~

- ~~fix: **分支无法保存tags**~~

- ~~fix: 自适应tag使用实际分辨率~~

- ~~fix: 同一源下分配了两种不同tags的子源，切换tags时会先消耗完现有tags的缓存再生效~~

- ~~去除启动检查可用性或异步，增加启动速度~~

- ~~不显示url~~，只在缓存模式支持全部历史，非缓存模式只支持json源

- add:  多条分发接口（/e/{name} 自定义端点，与 /random 完全同功能）

- add:  登录功能

- 

  

  



## 一、 用户交互流程 (User Interaction Flow)
系统的使用者分为三类角色：**API 调用者**、**本地节点管理员**、**社区参与者**。
```text
【API 调用者】 ──► 请求 /random?category=landscape ──► 获得 302 直链 / JSON 响应
                                ▲
                                │ 订阅/导入
【社区参与者】 ──► 访问 Serverless 社区 Hub ──► 浏览/点赞/评论/提交 API 规则
                                │
                                ▼ 导入规则
【节点管理员】 ──► 登录 Docker Dashboard ──► 智能检测/添加 API ──► 设置分类/缓存/权重

```
### 1. 场景一：API 调用者（前端应用 / Markdown / 个人网站）
 * **日常使用**：用户将统一接口 URL（如 https://my-domain.com/random?category=avatar）直接写在网页的 <img> 标签、Markdown 图片语法或 App 页面中。
 * **无感跳转**：服务端在后台完成“API 抽选 ➔ 异构解析 ➔ 获取真实图片 URL”，并直接以 **HTTP 302** 形式将浏览器重定向至最终图片 CDN，调用者感知不到中间的转换过程。
### 2. 场景二：本地节点管理员（部署 Docker 的用户）
 * **初始化**：在浏览器打开本地或 VPS 的 Dashboard 界面。
 * **添加与测试 API**：
   * **智能一键添加**：管理员粘贴任意第三方 API 链接，点击“智能识别”，系统自动发送请求探测响应格式：
     * 识别为图片流/302 重定向 ➔ 自动填写类型；
     * 识别为 JSON ➔ 弹出 JSON 树状预览，管理员点击包含图片 URL 的字段即可自动填充提取路径。
   * **高级添加**：手动配置该 API 的请求头（Referer / User-Agent 防盗链）、权重、适用分类等。
 * **批量运维**：在单页列表中点击“一键健康检测”，查看当前所有 API 的延迟（ms）与可用状态（绿色正常/红色异常），可一键开启或禁用特定 API。
 * **规则导出与订阅**：将本地整理好的 API 列表导出为 JSON 文件或一键分享码；或粘贴 community 社区链接一键拉取更新。
### 3. 场景三：社区参与者（使用 Serverless 社区 Hub）
 * **浏览与发现**：无需安装 Docker，直接访问托管在 GitHub Pages / Vercel 上的社区 Hub 页面，按 Tag（如 二次元、风景、横屏）筛选优质图源。
 * **互动与评价**：基于 Cloudflare KV / Supabase 后端，用户可以对社区中的 API 规则进行**点赞、点踩、发表评论**。
 * **一键复制/提交**：
   * 点击任意图源卡片上的“复制规则码”或“一键导入”，即可直接导入到自己的本地 Dashboard。
   * 具备新图源的用户可以通过可视化表单提交新 API 规则，经 Serverless 后端存储后呈现在社区广场。
## 二、 详细功能规格 (Functional Requirements)

### 1. API 源管理与智能识别 (Dashboard Panel)
 * **单页平铺展示**：列表展示 API 名称、域名、响应类型、绑定分类、权重、成功率、平均延迟与状态开关。
 * **智能识别引擎**：支持输入 URL 后自动解析 Response Header (Content-Type / Location) 与 Body (JSON 节点点击选择)。
 * **高级添加配置**：支持手动配置绑定分类、支持为每个第三方 API 单独配置请求头（Referer、User-Agent、Cookie 、api key等）。
### 2. 统一分发引擎
 * **对外统一接口**：GET /random
 * **随机与权重算法**：根据候选 API 的权重（1~100）按概率抽选。
 * **容错与重试 (Failover)**：若被抽中的 API 响应超时（默认 3 秒）或返回 50x 错误，系统自动剔除该 API 并尝试下一个可用源，最多重试 3 次；若连续失败 5 次，自动挂起该 API 并标记为健康异常。
 * **响应格式**：默认 302 重定向，支持通过参数 ?format=json 返回包含真实图片 URL 及源名称的结构化文本。
### 3. 系统设置与中转策略
 * **中转模式切换**：
   * **轻量模式（默认开启）**：主机仅做 Header / JSON 解析并吐出 302 重定向，不经过本地内存与磁盘，流量消耗极低。
   * **代理/本地缓存模式（可选开启）**：主机将第三方图片下载并缓存至本地磁盘（可设定最大缓存容量与过期 TTL），适合隐藏客户端 IP 或解决第三方 API 跨域/防盗链问题，支持通过分辨率过滤，只输出设定分辨率及以上的图片。
 * **安全防护**：支持设置全局速率限制（Rate Limit），防止接口被恶意刷流量。
### 4. 分类与多标签过滤
 * **分类预设**：支持 横屏、竖屏、自适应、头像、动漫、风景、ai生成 等标签。
 * **多标签关联**：单个 API 源可勾选多个分类标签。
 * **请求过滤**：支持通过 GET /random?category=landscape 或 ?category=avatar,anime 进行精确匹配过滤。
### 5. 社区中心 (Serverless Community Hub)
 * **托管架构**：前端部署于 GitHub Pages / Vercel，后端使用 **Cloudflare Workers + Cloudflare KV** 或 **Supabase（免费额度）** 存储交互数据。
 * **核心功能**：
   * **图源广场**：展示所有公开的 API 规则卡片，支持按分类与 Tag 筛选。
   * **点赞/点踩与评论**：记录规则的受欢迎程度与稳定度。
   * **提交与审核**：用户可提交新的 API 解析规则（支持公开或私密）。
   * **一键导入/订阅**：提供规则一键复制功能或订阅 URL。
## ⚠️ 社区部分（暂缓开发）

社区 Hub（Serverless Community Hub）当前处于公测预览阶段，暂不正式上线。以下为已知待解决问题：

1. **Turnstile 验证未集成**：前端 `SubmitForm.vue` 和 `CommentSection.vue` 硬编码了占位 token。Worker 配置 `TURNSTILE_SECRET` 后所有写操作将被拒绝。
2. **`VOTES_KV` 是死代码**：wrangler.toml 定义了该绑定但 Worker 代码未使用。投票数据直接存在 `RULES_KV` 的 Rule 对象中。
3. **投票无去重**：同一 IP 可无限制点赞/点踩，存在刷票漏洞。需用 `VOTES_KV` 实现 `vote:{rule_id}:{ip}` 去重。
4. **`comment_count` 竞态条件**：read-modify-write 未加锁，并发评论可能丢失计数。
5. **分类不一致**：浏览页（SquareView）有 8 个分类（含横屏/竖屏），提交页（SubmitForm）只有 6 个（缺少横屏/竖屏）。
6. **评论排序为升序**：最旧评论排在最前，与主流社区习惯相反。

---

## 三、 交付物清单 (Deliverables)
项目开发完成后，将交付以下 **4 部分** 完整内容：
### 1. 核心后端与打包文件 (Core Backend)
 * **Go 源代码**：基于 Gin + httpx/gjson 编写的完整后端源码。
 * **Dockerfile 与 docker-compose.yml**：多阶段构建的 Docker 打包文件。
 * **默认配置文件模板 (config.json.example)**：包含预设分类与样例 API 规则。
### 2. Dashboard 前端源码 (Embedded Web UI)
 * **Dashboard 静态资源**：包含 API 管理、智能识别、健康检测、系统设置的单页 UI 源码（编译后直接使用 go:embed 嵌入二进制中，无需额外部署 Nginx）。
### 3. 社区 Hub 部署包 (Serverless Community Hub)
 * **社区前端代码**：可直接一键部署至 GitHub Pages / Vercel 的静态网页源码。
 * **Serverless 后端脚本**：
   * Cloudflare Worker 脚本（worker.js）及 KV 绑定的配置文件；
   * 或 Supabase 数据表结构 SQL 初始化脚本（init.sql）。
### 4. 项目文档与指南 (Documentation)
 * **README.md**：Docker 一键启动指南、环境变量说明。
 * **API_DOC.md**：统一分发接口的参数使用说明（分类筛选、格式切换等）。
 * **COMMUNITY_DEPLOY.md**：如何 0 成本部署自己的 Serverless 社区 Hub 指南。
