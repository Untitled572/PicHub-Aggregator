export interface Tag {
  id: string
  name: string
  system?: boolean
  exclusive?: boolean
}

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
  proxy_enabled?: boolean
  proxy_url?: string
  cache_max_mb: number
  cache_max_images: number
  cache_ttl: number
  precache_count?: number
  pool_size?: number
  min_resolution: string
  rate_limit: number
  rate_limit_window?: number
  timeout: number
  health_check_interval?: number
  max_history_records?: number
  bound_tags?: string[]
  admin_token?: string
  saved_images_dir?: string
  login_enabled?: boolean
  admin_username?: string
  admin_password?: string
  session_hours?: number
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

export interface TagStat {
  tag_id: string
  count: number
}

export interface SourceStat {
  source_id: number
  source_name: string
  hit_count: number
}

export interface DailyTrend {
  date: string
  total: number
}

export interface SourceDailyTrend {
  date: string
  source_id: number
  source_name: string
  hit_count: number
}

export interface StatsOverview {
  total: number
  tags: TagStat[]
  sources: SourceStat[]
  daily_trends?: DailyTrend[]
  source_trends?: SourceDailyTrend[]
}


export interface StatsResponse {
  today: StatsOverview
  stats: StatsOverview
  start_date: string
  end_date: string
  range?: string
  total: {
    total_requests: number
  }
}


export interface ImageHistoryRecord {
  id: number
  image_url: string
  source_id: number
  source_name: string
  categories: string
  created_at: string
  image_id?: number
  file_id?: string
  is_saved?: boolean
}


export interface SavedImage {
  id: number
  file_id: string
  source_name: string
  width: number
  height: number
  format: string
  file_size: number
  original_url: string
  saved_at: string
}

export interface Endpoint {
  id: number
  name: string
  bound_tags: string[]
  enabled: boolean
  created_at?: string
  updated_at?: string
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
    params?: QueryParam[]
    enabled: boolean
  }>
}

export interface ImportResult {
  imported: number
}
