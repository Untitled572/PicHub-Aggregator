<div align="center">

# 🖼️ PicHub Aggregator

**超高性能、单文件部署的第三方图片 API 聚合分发引擎**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Vue3](https://img.shields.io/badge/Vue.js-v3.4-4FC08D?style=flat-square&logo=vue.js)](https://vuejs.org)
[![Tailwind CSS](https://img.shields.io/badge/TailwindCSS-v3.4-38BDF8?style=flat-square&logo=tailwindcss)](https://tailwindcss.com)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker)](https://www.docker.com)
[![License](https://img.shields.io/badge/License-GPL--3.0-blue.svg?style=flat-square)](LICENSE)


[快速开始](#-快速开始) •
[核心特性](#-核心特性) •
[API 使用指南](#-api-使用指南) •
[架构设计](#-架构设计) •
[社区与扩展](#-社区规则广场)

</div>

---

## 🌟 简介

**PicHub** 是一个专为开发者、博客作者与前端应用打造的图片 API 聚合与分发引擎。

它可以将全网散落的各种第三方随机图片 API（包括图片直链、302 重定向、JSON 响应提取等）统一收归管理，对外提供极速、稳定且支持多维度过滤的单分发入口 `/random`。内置莫兰迪 (Morandi) 冷灰蓝风格的现代化控制台，无需额外部署 Web 服务。

```
		[前端 / Markdown / 博客 / APP]
			          │
			          ▼  GET /random
┌─────────────────────────────────────────┐
│           PicHub 聚合分发引擎             │
│  ┌───────────────────────────────────┐  │
│  │  智能类型解析 | 权重抽选 | 健康降级    │  │
│  └───────────────────────────────────┘  │
└────────────────────┬────────────────────┘
                     │
    ┌────────────────┼────────────────┐
    ▼                ▼                ▼
[二次元 API]    [Unsplash API]   [自建图床 API]
```

---

## ✨ 核心特性

- ⚡ **单文件极简部署**
  Go + Gin 驱动，配合 `go:embed` 技术将Vue 3 Dashboard 嵌入至单个可执行二进制文件，无需 Node.js 运行时即可轻量运行。
- 🎯 **多 Tag 标签与独立分发**
  支持自定义分类 Tag（如默认 `#横屏`, `#竖屏`, `#自适应`）管理。每个 Tag 拥有独立的专属分发链接，同时支持总分发接口动态绑定 Tags 范围。
- 🧩 **多参数与路径衍生分支 (Query Params & Path Variants)**
  支持为单个 API 图源添加多个参数分支（如 `type=pc` / `type=mobile` 或 `return=302&type=pc` 以及路径式 `/pc`），每个分支可独立绑定分类 Tag（如 `#横屏`, `#竖屏`）与权重。分发时各分支作为独立加权源参与抽选，总体统一进行巡检与统计。
- ⚡ **智能 URL 类型检测**
  输入或粘贴第三方 API 地址即可自动推导解析类型（`image` 图片直链、`redirect` 302重定向、`json` 数据节点提取），并支持自动补全相对 302 路径与在线报文测试。
- 🛡️ **健康检测与容错降级**
  内置加权随机算法与 3 次请求重试降级。配合默认 360 分钟（可调）后台批量健康检测，连续故障的图源节点自动挂起，确保对外服务始终高可用。
- 📋 **多行自定义请求头与行内确认**
  支持为图源独立配置多行 Request Headers（如 `Referer` / `User-Agent` / `Authorization`）；全站采用紧凑非侵入式行内确认 (`确认删除? [删除] [取消]`)，操作体验顺畅。


---

## 📸 控制台预览

* **图源管理**：一键切换简易 / 高级模式，动态设置 1 ~ 5 级抽选权重与分类标签。
* **接口管理**：Tag 标签库与总聚合接口绑定中心，直观预览并复制独立分发 URL。
* **健康检测**：实时连通性、HTTP 响应码及网络延迟统计看板。
* **系统设置**：代理中转、磁盘缓存模式（研发中）、自定义域名绑定与 Rate Limit 防刷保护。

---

## 🚀 快速开始

### 方式一：Docker Compose 一键部署 (推荐)

项目已提供 Docker Compose 配置文件

```bash
services:
  pichub:
    image: ghcr.io/untitled572/pichub-aggregator:latest
    network_mode: host
    volumes:
      - ./data:/app/data
      - ./cache:/app/cache
    environment:
      - PORT=5721
      - DB_PATH=/app/data/pichub.db
      - CACHE_PATH=/app/cache
    restart: unless-stopped
```

启动完成后，在浏览器中访问管理控制台：
👉 **http://localhost:5721**

> `docs/docker-compose.yml` 使用 `network_mode: host`，容器监听在宿主机 5721 端口。
> 如需自定义端口，可设置环境变量 `PORT` 并修改监听地址。
> 数据持久化在 `./pichub_data/`（SQLite 数据库）和 `./pichub_cache/`（图片缓存）。

---

### 方式二：使用预构建 Docker 镜像

```bash
docker run -d --name pichub --network host \
  -v ./pichub_data:/app/data \
  -v ./pichub_cache:/app/cache \
  ghcr.io/untitled572/pichub-aggregator:latest
```

---

### 方式三：手动编译构建

#### 1. 编译 Dashboard 前端

```bash
cd dashboard
npm install
npm run build
```

#### 2. 编译并运行 Go 后端

```bash
cd ../backend
go build -o pichub .
./pichub
```

---

## 📡 API 使用指南

 PicHub 为用户提供统一且极简的调用方式，只需在网页、博客 Markdown 或客户端中调用单个 URL：

### 1. 统一随机图片直链 (Master Endpoint)

```http
GET /random
```
* **说明**：按权重随机抽选可用图源，302 重定向到最终图片（或返回图片流）。

### 2. 按分类 Tag 筛选

```http
GET /random?category=landscape
GET /random?category=avatar,anime
```
* **说明**：仅从包含 `landscape`（风景）或 `avatar,anime`（头像/二次元）标签的可用图源中抽选。

### 3. JSON 数据返回格式

```http
GET /random?format=json
GET /random?category=anime&format=json
```
* **返回示例**：
```json
{
  "id": 1,
  "name": "Unsplash 高清风景源",
  "url": "https://images.unsplash.com/photo-1506744038136-46273834b3fb",
  "resp_type": "image",
  "latency_ms": 120
}
```

### 📋 查询参数详解

| 参数名 | 类型 | 默认值 | 说明 | 示例 |
| :--- | :--- | :--- | :--- | :--- |
| `category` | `string` | *无* | 筛选 Tag 标签（支持逗号分隔多个） | `?category=landscape,anime` |
| `format` | `string` | *无* | 设为 `json` 时返回结构化 JSON 数据 | `?format=json` |

---

## ⚙️ 环境变量配置

支持通过环境变量调整运行参数：

| 环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PORT` | `5721` | HTTP 服务监听端口 |
| `DB_PATH` | `./data/pichub.db` | SQLite 数据库文件存储路径 |

---

## 🛠️ 项目结构

```
PicHub/
├── backend/                  # Go + Gin 后端服务
│   ├── config/               # 种子配置与默认设置
│   ├── handler/              # API 路由处理函数 (Source / Settings / Health / Random)
│   ├── model/                # 数据模型与结构体定义
│   ├── service/              # 核心引擎 (Weighted Pick / Health Checker / Proxy)
│   ├── store/                # SQLite 持久化存储驱动
│   └── main.go               # 主入口 (go:embed 嵌入前端静态资源)
├── dashboard/                # Vue 3 + TypeScript 现代化 Dashboard
│   ├── src/
│   │   ├── components/       # 视图组件 (SourceCard, SourceForm, HealthStatusBadge)
│   │   ├── composables/      # 响应式状态管理 (useApi, useTags)
│   │   └── views/            # 核心页面 (SourcesView, EndpointsView, HealthCheckView, SettingsView)
│   └── tailwind.config.js    # Morandi 色系主题样式定义
└── community/                # Serverless 规则共享社区 (研发中)
```

---

## 🔗 相关资源

- [随机二次元图片 API 合集](https://blog.air-kevin.rf.gd/2025/random-pix-api-list/?i=2) - 汇总了大量可用的随机图片 API 接口，可作为 PicHub 图源配置的参考来源。

---

## 🤝 贡献与反馈

欢迎提交 Issue 或 Pull Request 来帮助改善 PicHub！
如果 PicHub 对你有帮助，欢迎点亮右上角的 ⭐️ **Star** 予以支持！

---

## 📄 开源协议

本项目采用 [GPL-3.0 License](LICENSE) 协议开源。

