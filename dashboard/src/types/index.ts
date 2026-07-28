export interface QueryParam {
  key: string
  value: string
  weight?: number
  categories?: string[]
}

export interface Source {
  id: number
  name: string
  url: string
  resp_type: string
  json_path: string
  weight: number
  categories: string[]
  headers: Record<string, string>
  params?: QueryParam[]
  default_query: string
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
  health_check_interval?: number
  bound_tags?: string[]
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
    default_query: string
    enabled: boolean
  }>
}

export interface ImportResult {
  imported: number
}
