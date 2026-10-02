<div align="center">

# PicHub Aggregator

**单文件部署的图片 API 聚合与分发服务**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Vue3](https://img.shields.io/badge/Vue.js-v3.4-4FC08D?style=flat-square&logo=vue.js)](https://vuejs.org)
[![Tailwind CSS](https://img.shields.io/badge/TailwindCSS-v3.4-38BDF8?style=flat-square&logo=tailwindcss)](https://tailwindcss.com)
[![Release](https://img.shields.io/badge/Release-v0.5.0-emerald?style=flat-square)](https://github.com/untitled572/PicHub-Aggregator/releases)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker)](https://www.docker.com)
[![License](https://img.shields.io/badge/License-GPL--3.0-blue.svg?style=flat-square)](LICENSE)


[快速开始](#-快速开始) •
[核心特性](#-核心特性) •
[控制台预览](#-控制台预览) •
[API 使用指南](#-api-使用指南) •
[项目结构](#-项目结构)

</div>

---

## 🌟 简介

**PicHub** 用于统一管理和调用多个图片 API。

它可以统一管理第三方随机图片 API，包括图片直链、302 重定向和从 JSON 响应提取图片地址，并通过 `/random` 提供统一的分发入口和分类筛选。项目内置管理控制台，无需单独部署前端 Web 服务。

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

- ⚡ **单文件部署**
  Go + Gin 驱动，将打包后的前端页面嵌入至单个可执行二进制文件。无需 Node.js、Nginx 或外部前端环境，单文件/单镜像即开即用。
- 🎯 **多维分类 Tag 与专属分发**
  支持自定义分类 Tag（如默认 `#横屏`, `#竖屏`, `#自适应`）管理。支持为客户端绑定专属 Tag 分发链接，或在 `GET /random?category=tag1,tag2` 中动态多选过滤分发。
- 🏷️ **内置标签与独占标签**
  横屏、竖屏和自适应是内置标签。设置了 `exclusive: true` 的标签只在请求明确指定时参与分发。
- 🧩 **多参数与路径衍生分支**
  支持为图源配置参数变体（如 `type=pc`）或子 API 路径（如 `/pe.php`）。变体继承主图源设置，可单独绑定 Tag 和权重，分发记录会标明使用的变体。
- 💾 **本地缓存代理模式**
  开启 `proxy_mode` 后，服务会将抓取的图片保存到图片缓存目录（`./data/images`），并通过 `/images/:file_id` 提供图片。缓存可减少重复请求上游；图片尺寸过滤和离线保存也需要此模式。
- 📐 **按图片宽高筛选**
  基于 Go `image.DecodeConfig` 对图片流/缓存文件的真实宽高进行解码检测。在 `proxy_mode=true` 本地代理中转模式下，支持通过 `?orientation=horizontal|vertical` 按图片实际宽高筛选横屏或竖屏图片。
- 👍 **历史流水与权重动态微调**
  可以查看取图记录和图片预览，通过“喜欢”或“不喜欢”调整图源权重。
- 🖼️ **离线保存图库与图墙**
  可以将图片保存到本地。提供 **列表视图**、**小图展示** 与 **大图图墙** 3 种模式。大图模式取消传统分页栏，采用 **`IntersectionObserver` 无限滚动** 与 **`loading="lazy"` 按需懒加载**。
- 🛡️ **加权随机抽选与自动容错降级**
  分发时按权重选择图源，并可尝试其他候选图源。后台会按设定间隔检测已启用图源；检测结果不代表后续图片下载一定成功，服务可用性也取决于上游 API 和网络状态。

---

## 📸 控制台预览

### 🖼️ 大图图墙

![大图图墙](screenshots/saved_large.png)

<sub>滚动浏览保存的图片，也可以下载图片或取消保存。</sub>

### 🏷️ 接口与 Tag 标签管理

![接口与 Tag 标签管理](screenshots/saved_small.png)

<sub>独立归集的【系统内置标签框】（横屏/竖屏/自适应），支持独占标签（Exclusive Tag）安全隔离。</sub>

### 📦 图源管理库

![图源管理库](screenshots/sources.png)

<sub>配置图源地址、权重、子 API 路径和参数分支。</sub>

### 📊 使用统计与历史流水

![使用统计与历史流水](screenshots/endpoints.png)

<sub>查看请求趋势、各图源的使用次数和取图记录。</sub>

---

## 🚀 快速开始

### 方式一：Docker Compose 一键部署 (推荐)

项目已提供 [Docker Compose 配置文件](docs/docker-compose.yml)

```yaml
services:
  pichub:
    image: ghcr.nju.edu.cn/untitled572/pichub-aggregator:latest
    network_mode: host
    volumes:
      - ./data:/app/data
      - ./cache:/app/cache
      - ./pichub_saved:/app/data/saved
    environment:
      - PUID=1000
      - PGID=1000
      - PORT=5721
      - DB_PATH=/app/data/pichub.db
      - TRUSTED_PROXIES=127.0.0.1/32
    restart: unless-stopped
```

启动完成后，在浏览器中访问管理控制台：
👉 **http://localhost:5721**

> [`docs/docker-compose.yml`](docs/docker-compose.yml) 使用 `network_mode: host`，容器监听在宿主机 5721 端口。
> 如需自定义端口，可设置环境变量 `PORT` 并修改监听地址。
> 数据持久化在 `./data/`（SQLite 数据库与离线保存库）和 `./cache/`（图片缓存）。

---

### 方式二：使用预构建 Docker 镜像

```bash
docker run -d --name pichub --network host \
  -e PUID=1000 -e PGID=1000 \
  -v ./pichub_data:/app/data \
  -v ./pichub_cache:/app/cache \
  ghcr.io/untitled572/pichub-aggregator:latest
```

---

### 方式三：手动编译构建

#### 1. 编译 Dashboard 前端

```bash
cd dashboard
npm ci
npm run build
```

#### 2. 编译并运行 Go 后端

需要 Go 1.26 或更新版本，以及用于 CGO 的 C 编译器。先构建 Dashboard，再编译后端；后端会把已构建的前端文件嵌入到可执行文件中。

```bash
cd ../backend
go build -o pichub .
./pichub
```

### 方式四：下载 release 预构建包

前往 [GitHub Releases](https://github.com/Untitled572/PicHub-Aggregator/releases) 页面，下载对应平台（linux-amd64 / linux-arm64 / windows-amd64）的压缩包，解压后运行即可。不依赖任何运行时环境。

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

### 3. 独占标签 (Exclusive Tag) 安全隔离

```http
GET /random?category=nsfw
```
* **说明**：标记为 `exclusive: true` 的独占标签（如 `nsfw`）具有安全隔离特性。客户端必须在 URL 中显式指定该标签才会触发分发，未指定时系统自动屏蔽携带独占标签的所有图源，确保默认分发内容安全可控。系统内置的 `#横屏`、`#竖屏`、`#自适应` 为非独占标签。

### 4. 按真图片比例过滤 (Orientation Filter)

```http
GET /random?orientation=horizontal
GET /random?orientation=vertical
```
* **说明**：在 `proxy_mode=true` 模式下，基于图片真实宽高比（`image.DecodeConfig`）自动过滤横屏或竖屏图片。

### 5. JSON 数据返回格式

```http
GET /random?format=json
GET /random?category=anime&format=json
```
* **返回示例**：
```json
{
  "url": "https://images.unsplash.com/photo-1506744038136-46273834b3fb",
  "local_url": "/images/a1b2c3d4",
  "source": "Unsplash 高清风景源",
  "categories": ["landscape", "horizontal"],
  "file_id": "a1b2c3d4",
  "width": 1920,
  "height": 1080,
  "format": "jpeg",
  "image_id": 42
}
```

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
│   ├── embed/                # 前端静态资源嵌入 (go:embed)
│   ├── handler/              # API 路由处理函数 (Source / Settings / Health / Random / Image)
│   ├── logger/               # 文件日志支持 (200 行轮转)
│   ├── middleware/           # Gin 中间件 (CORS / Rate Limit / Access Log / Admin Auth)
│   ├── model/                # 数据模型与结构体定义 (Source, Tag, Settings, Image)
│   ├── service/              # 核心引擎 (Weighted Pick / Health Checker / Proxy / ImageStore)
│   ├── store/                # SQLite 持久化存储驱动
│   └── main.go               # 主入口
├── dashboard/                # Vue 3 + TypeScript 现代化 Dashboard
│   ├── src/
│   │   ├── components/       # 视图组件 (SourceCard, SourceForm, ParamVariantsModal)
│   │   ├── composables/      # 响应式状态管理 (useApi, useTags, useHealthCheck)
│   │   └── views/            # 核心页面 (Sources, Endpoints, HealthCheck, Settings, Stats, Saved)
│   └── tailwind.config.js    # Morandi 色系主题样式定义
├── screenshots/              # README 控制台预览截图库
├── devdocs/                  # 面向 AI 辅助开发的架构文档 (详见 devdocs/README.md)
└── docs/                     # API 用户接口文档与 docker-compose 配置
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
