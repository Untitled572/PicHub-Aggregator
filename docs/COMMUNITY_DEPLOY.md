# Community Hub Deployment Guide

## Architecture

- **Frontend**: Static Vue3 SPA (deploy to GitHub Pages / Vercel)
- **Backend**: Cloudflare Workers + KV (free tier)

## Deploy Frontend

```bash
cd community/frontend
npm install
npm run build
```

Deploy `dist/` to GitHub Pages or Vercel.

## Deploy Worker Backend

1. Install Wrangler:

```bash
npm install -g wrangler
```

2. Create KV namespaces:

```bash
wrangler kv:namespace create RULES_KV
wrangler kv:namespace create VOTES_KV
wrangler kv:namespace create COMMENTS_KV
wrangler kv:namespace create RATELIMIT_KV
```

3. Update `community/worker/wrangler.toml` with the KV namespace IDs.

4. (Optional) Set Turnstile secret:

```bash
wrangler secret put TURNSTILE_SECRET
```

5. Deploy:

```bash
cd community/worker
npx wrangler deploy
```

6. Update frontend `.env` with the Worker URL:

```
VITE_API_BASE=https://your-worker.example.com
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/rules` | List rules (?category=&sort=) |
| POST | `/api/rules` | Submit rule (requires Turnstile) |
| POST | `/api/rules/:id/vote` | Vote up/down |
| GET | `/api/rules/:id/comments` | Get comments |
| POST | `/api/rules/:id/comments` | Add comment |
