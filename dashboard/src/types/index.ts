export interface Source {
  id: number
  name: string
  url: string
  resp_type: string
  json_path: string
  weight: number
  categories: string[]
  headers: Record<string, string>
  enabled: boolean
  fail_count: number
  success_rate: number
  avg_latency: number
  status: string
  created_at: string
  updated_at: string
}

export interface Settings {
  proxy_mode: boolean
  cache_max_mb: number
  cache_ttl: number
  min_resolution: string
  rate_limit: number
  timeout: number
}

export interface DetectResult {
  resp_type: string
  headers: Record<string, string>
  body_tree: unknown
  url_hints: string[]
  error?: string
}

export interface HealthResult {
  id: number
  name: string
  url: string
  status_code: number
  latency_ms: number
  available: boolean
  error?: string
}

export interface ExportData {
  sources: Array<{
    name: string
    url: string
    resp_type: string
    json_path: string
    weight: number
    categories: string[]
    headers: Record<string, string>
    enabled: boolean
  }>
}

export interface ImportResult {
  imported: number
}
