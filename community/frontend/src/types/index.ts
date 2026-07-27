export interface Rule {
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

export interface Comment {
  id?: string
  rule_id: string
  author: string
  content: string
  created_at?: string
}

export interface VoteData {
  type: 'up' | 'down'
  turnstile_token: string
}

export interface CommentData {
  author: string
  content: string
  turnstile_token: string
}
