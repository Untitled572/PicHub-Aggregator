<div align="center">

# 🖼️ PicHub Aggregator

**超高性能、单文件部署的第三方图片 API 聚合分发引擎**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Vue3](https://img.shields.io/badge/Vue.js-v3.4-4FC08D?style=flat-square&logo=vue.js)](https://vuejs.org)
[![Tailwind CSS](https://img.shields.io/badge/TailwindCSS-v3.4-38BDF8?style=flat-square&logo=tailwindcss)](https://tailwindcss.com)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker)](https://www.docker.com)
[![License](https://img.shields.io/badge/License-MIT-blue.style=flat-square)](LICENSE)

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
          ▼  GET /random?category=landscape
┌─────────────────────────────────────────┐
│           PicHub 聚合分发引擎           │
│  ┌───────────────────────────────────┐  │
│  │ 智能类型解析 | 权重抽选 | 健康降级 │  │
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
  Go + Gin 驱动，配合 `go:embed` 技术将全功能 Vue 3 莫兰迪色系 Dashboard 嵌入至单个可执行二进制文件，无需 Node.js 运行时即可轻量运行。

- 🎯 **多 Tag 标签与独立分发**
  支持自定义分类 Tag（如 `#头像`, `#二次元`, `#风景`, `#人像`, `#自适应`, `#AI生成`）管理。每个 Tag 拥有独立的专属分发链接，同时支持总分发接口动态绑定 Tags 范围。

- 🌐 **自定义对外域名绑定**
  系统设置支持绑定自定义对外域名（如 `https://pic.example.com`），保存后控制台内所有生成与一键复制的 API 接口链接均会自动替换域名前缀。

- ⚡ **智能 URL 类型检测**
  输入或粘贴第三方 API 地址即可自动推导解析类型（`image` 图片直链、`redirect` 302重定向、`json` 数据节点提取），并支持一键 HEAD 报文在线类型实测。

- 🛡️ **健康检测与容错降级**
  内置加权随机算法与 3 次请求重试降级。配合后台 5 分钟定时批量健康检测，连续故障的图源节点自动挂起，确保对外服务始终高可用。

- 📋 **多行自定义请求头 (Headers)**
  支持为图源独立配置多行 Request Headers（如 `Referer` / `User-Agent` / `Authorization`），轻松应对各类防盗链与鉴权限制。

---

## 📸 控制台预览

* **图源管理**：一键切换简易 / 高级模式，动态设置 1 ~ 5 级抽选权重与分类标签。
* **接口管理**：Tag 标签库与总聚合接口绑定中心，直观预览并复制独立分发 URL。
* **健康检测**：实时连通性、HTTP 响应码及网络延迟统计看板。
* **系统设置**：代理中转、磁盘缓存模式（研发中）、自定义域名绑定与 Rate Limit 防刷保护。

---

## 🚀 快速开始

### 方式一：Docker Compose 一键部署 (推荐)

项目根目录已准备好 Compose 配置文件：

```bash
# 启动容器服务
docker compose -f backend/docker-compose.yml up -d --build
```

启动完成后，在浏览器中访问管理控制台：
👉 **http://localhost:5721**

---

### 方式二：手动编译构建

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
│   │   ├── composables/      # 响应式状态管理 (useApi, useTags, useDomain)
│   │   └── views/            # 核心页面 (SourcesView, EndpointsView, HealthCheckView, SettingsView)
│   └── tailwind.config.js    # Morandi 色系主题样式定义
└── community/                # Serverless 规则共享社区 (研发中)
```

---

## 🤝 贡献与反馈

欢迎提交 Issue 或 Pull Request 来帮助改善 PicHub！
如果 PicHub 对你有帮助，欢迎点亮右上角的 ⭐️ **Star** 予以支持！

---

## 📄 开源协议

本项目采用 [MIT License](LICENSE) 协议开源。
