# PicHub-Aggregator

A unified image API aggregator with embedded Dashboard, distribution engine, and community hub.

## Quick Start (Docker)

```bash
# Pull image
docker pull ghcr.io/untitled572/pichub-aggregator:latest

# Start with docker-compose (host network mode)
docker compose -f docs/docker-compose.yml up -d

# Or run directly
docker run -d --name pichub --network host \
  -v ./pichub_data:/app/data \
  -v ./pichub_cache:/app/cache \
  ghcr.io/untitled572/pichub-aggregator:latest
```

Open http://localhost:5721

> Default port: 5721 (host network mode).
> Data persists in `./pichub_data/` (SQLite) and `./pichub_cache/` (image cache).

## Pull from GHCR

The image is hosted at GitHub Container Registry:

```bash
docker pull ghcr.io/untitled572/pichub-aggregator:latest
```

> ⚠️ Make sure the package visibility is set to **Public** in the repository settings:
> `https://github.com/Untitled572/PicHub-Aggregator/settings/packages`

## Environment Variables

| Variable   | Default               | Description          |
|------------|-----------------------|----------------------|
| `PORT`     | `5721`                | HTTP listen port     |
| `DB_PATH`  | `./data/pichub.db`    | SQLite database path |

## Manual Build

```bash
# Build backend
cd backend && go build -o pichub .

# Build dashboard (optional, for dev)
cd dashboard && npm install && npm run build
```

## Project Structure

```
backend/       Go + Gin backend (API, engine, SQLite)
dashboard/     Vue3 dashboard SPA (embedded in binary)
community/     Serverless community hub
docs/          Documentation
```
