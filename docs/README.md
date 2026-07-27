# PicHub-Aggregator

A unified image API aggregator with embedded Dashboard, distribution engine, and community hub.

## Quick Start (Docker)

```bash
docker compose -f backend/docker-compose.yml up --build
```

Open http://localhost:5721

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
