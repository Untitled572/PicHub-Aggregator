# PicHub-Aggregator

统一图片 API 聚合器。一键聚合多个图源，对外提供单个 `/random` 接口，支持权重分发、302 重定向、JSON 提取、智能识别、健康检测和 Docker 部署。

## 快速开始

```bash
docker compose -f backend/docker-compose.yml up --build
```

打开 http://localhost:5721

## 对外接口

用户只需在浏览器或 HTML 中访问一个 URL：

```
http://localhost:5721/random?category=landscape
```

服务端自动完成：按权重选源 → 请求第三方 API → 解析响应 → **302 重定向**到最终图片（或返回 JSON）。

### 参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `category` | 分类筛选（多标签逗号分隔） | `?category=avatar,anime` |
| `format` | `json` 时返回结构化数据 | `?format=json` |

## 分类

avatar, anime, landscape, portrait, adaptive, ai-generated

## 功能

- 分发引擎：权重随机 + 容错重试（最多 3 次，连续失败 5 次自动挂起）
- 智能识别：粘贴 URL 自动探测响应类型（redirect/image/json）和 JSON 提取路径
- 代理缓存：可选模式，本地缓存图片并支持分辨率过滤
- Dashboard：内嵌 Web 管理界面（Vue3 + Tailwind），无需额外部署
- 健康检测：批量检测 + 定时任务（每 5 分钟）
- 规则导入导出：支持敏感 Header 过滤
- 社区 Hub：Serverless 架构，共享图源规则

## 手动构建

```bash
# 后端
cd backend && go build -o pichub . && ./pichub

# Dashboard（开发模式热重载）
cd dashboard && npm install && npm run dev
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `5721` | HTTP 监听端口 |
| `DB_PATH` | `./data/pichub.db` | SQLite 数据库路径 |

## 项目结构

```
backend/       Go + Gin 后端（API 路由、分发引擎、SQLite 存储）
dashboard/     Vue3 + TypeScript Dashboard（go:embed 嵌入二进制）
community/     Serverless 社区 Hub
docs/          文档
```
