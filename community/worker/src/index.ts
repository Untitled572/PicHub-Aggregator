interface Env {
  RULES_KV: KVNamespace
  VOTES_KV: KVNamespace
  COMMENTS_KV: KVNamespace
  RATELIMIT_KV: KVNamespace
  TURNSTILE_SECRET?: string
}

interface Rule {
  id?: string
  name: string
  url: string
  description?: string
  resp_type: string
  json_path: string
  categories: string[]
  author?: string
  public: boolean
  upvotes: number
  downvotes: number
  comment_count: number
  created_at?: string
}

interface Comment {
  id?: string
  rule_id: string
  author: string
  content: string
  created_at?: string
}

interface VoteData {
  type: 'up' | 'down'
  turnstile_token: string
}

interface CommentData {
  author: string
  content: string
  turnstile_token: string
}

const CORS_HEADERS = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type',
}

function jsonResponse(data: any, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { ...CORS_HEADERS, 'Content-Type': 'application/json' },
  })
}

function errorResponse(message: string, status = 400): Response {
  return jsonResponse({ error: message }, status)
}

async function verifyTurnstile(token: string, secret: string): Promise<boolean> {
  try {
    const res = await fetch('https://challenges.cloudflare.com/turnstile/v0/siteverify', {
      method: 'POST',
      body: new URLSearchParams({ secret, response: token }),
    })
    const data: any = await res.json()
    return data.success === true
  } catch {
    return false
  }
}

async function checkRateLimit(ip: string, kv: KVNamespace): Promise<boolean> {
  const key = `ratelimit:${ip}`
  const count = await kv.get(key)
  const current = count ? parseInt(count) : 0
  if (current >= 10) return false
  await kv.put(key, String(current + 1), { expirationTtl: 60 })
  return true
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url)
    const path = url.pathname
    const method = request.method

    if (method === 'OPTIONS') {
      return new Response(null, { headers: CORS_HEADERS })
    }

    const ip = request.headers.get('CF-Connecting-IP') || 'unknown'

    try {
      if (path === '/api/rules' && method === 'GET') {
        const category = url.searchParams.get('category')
        const sort = url.searchParams.get('sort') || 'popular'
        return handleGetRules(env, category, sort)
      }

      if (path === '/api/rules' && method === 'POST') {
        const body: any = await request.json()
        if (!await checkRateLimit(ip, env.RATELIMIT_KV)) {
          return errorResponse('Rate limit exceeded', 429)
        }
        if (env.TURNSTILE_SECRET && !await verifyTurnstile(body.turnstile_token, env.TURNSTILE_SECRET)) {
          return errorResponse('Invalid captcha', 403)
        }
        return handleCreateRule(env, body)
      }

      const voteMatch = path.match(/^\/api\/rules\/([^/]+)\/vote$/)
      if (voteMatch && method === 'POST') {
        const body: VoteData = await request.json()
        if (!await checkRateLimit(ip, env.RATELIMIT_KV)) {
          return errorResponse('Rate limit exceeded', 429)
        }
        if (env.TURNSTILE_SECRET && !await verifyTurnstile(body.turnstile_token, env.TURNSTILE_SECRET)) {
          return errorResponse('Invalid captcha', 403)
        }
        return handleVote(env, voteMatch[1], body)
      }

      const commentMatch = path.match(/^\/api\/rules\/([^/]+)\/comments$/)
      if (commentMatch) {
        if (method === 'GET') {
          return handleGetComments(env, commentMatch[1])
        }
        if (method === 'POST') {
          const body: CommentData = await request.json()
          if (!await checkRateLimit(ip, env.RATELIMIT_KV)) {
            return errorResponse('Rate limit exceeded', 429)
          }
          if (env.TURNSTILE_SECRET && !await verifyTurnstile(body.turnstile_token, env.TURNSTILE_SECRET)) {
            return errorResponse('Invalid captcha', 403)
          }
          return handleAddComment(env, commentMatch[1], body)
        }
      }

      return errorResponse('Not found', 404)
    } catch (e: any) {
      return errorResponse(e.message, 500)
    }
  },
}

async function handleGetRules(env: Env, category: string | null, sort: string): Promise<Response> {
  const list = await env.RULES_KV.list({ prefix: 'rule:' })
  const rules: Rule[] = []

  for (const key of list.keys) {
    const data = await env.RULES_KV.get(key.name)
    if (!data) continue
    const rule: Rule = JSON.parse(data)
    if (!rule.public) continue
    if (category && !rule.categories.includes(category)) continue
    rules.push(rule)
  }

  if (sort === 'newest') {
    rules.sort((a, b) => new Date(b.created_at || 0).getTime() - new Date(a.created_at || 0).getTime())
  } else if (sort === 'oldest') {
    rules.sort((a, b) => new Date(a.created_at || 0).getTime() - new Date(b.created_at || 0).getTime())
  } else {
    rules.sort((a, b) => (b.upvotes || 0) - (a.upvotes || 0))
  }

  return jsonResponse(rules)
}

async function handleCreateRule(env: Env, data: any): Promise<Response> {
  const id = crypto.randomUUID()
  const rule: Rule = {
    id,
    name: data.name,
    url: data.url,
    description: data.description || '',
    resp_type: data.resp_type || 'json',
    json_path: data.json_path || '',
    categories: data.categories || [],
    author: data.author || '',
    public: data.public !== false,
    upvotes: 0,
    downvotes: 0,
    comment_count: 0,
    created_at: new Date().toISOString(),
  }

  if (!rule.name || !rule.url) {
    return errorResponse('Name and URL are required')
  }

  await env.RULES_KV.put(`rule:${id}`, JSON.stringify(rule))
  return jsonResponse(rule, 201)
}

async function handleVote(env: Env, id: string, data: VoteData): Promise<Response> {
  const ruleData = await env.RULES_KV.get(`rule:${id}`)
  if (!ruleData) return errorResponse('Rule not found', 404)

  const rule: Rule = JSON.parse(ruleData)
  if (data.type === 'up') {
    rule.upvotes = (rule.upvotes || 0) + 1
  } else {
    rule.downvotes = (rule.downvotes || 0) + 1
  }

  await env.RULES_KV.put(`rule:${id}`, JSON.stringify(rule))
  return jsonResponse({ upvotes: rule.upvotes, downvotes: rule.downvotes })
}

async function handleGetComments(env: Env, ruleId: string): Promise<Response> {
  const list = await env.COMMENTS_KV.list({ prefix: `comment:${ruleId}:` })
  const comments: Comment[] = []

  for (const key of list.keys) {
    const data = await env.COMMENTS_KV.get(key.name)
    if (!data) continue
    comments.push(JSON.parse(data))
  }

  comments.sort((a, b) => new Date(a.created_at || 0).getTime() - new Date(b.created_at || 0).getTime())
  return jsonResponse(comments)
}

async function handleAddComment(env: Env, ruleId: string, data: CommentData): Promise<Response> {
  const id = crypto.randomUUID()
  const ts = Date.now()
  const comment: Comment = {
    id,
    rule_id: ruleId,
    author: data.author,
    content: data.content,
    created_at: new Date().toISOString(),
  }

  if (!comment.author || !comment.content) {
    return errorResponse('Author and content are required')
  }

  await env.COMMENTS_KV.put(`comment:${ruleId}:${ts}:${id}`, JSON.stringify(comment))

  const ruleData = await env.RULES_KV.get(`rule:${ruleId}`)
  if (ruleData) {
    const rule: Rule = JSON.parse(ruleData)
    rule.comment_count = (rule.comment_count || 0) + 1
    await env.RULES_KV.put(`rule:${ruleId}`, JSON.stringify(rule))
  }

  return jsonResponse(comment, 201)
}
