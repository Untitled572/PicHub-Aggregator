package handler

import "github.com/pichub/backend/model"

// DetectURLRequest POST /random/detect 请求体
type DetectURLRequest struct {
	URL string `json:"url"`
}

// EndpointCreateRequest 创建自定义端点请求体
type EndpointCreateRequest struct {
	Name      string   `json:"name"`
	BoundTags []string `json:"bound_tags"`
}

// EndpointUpdateRequest 更新自定义端点请求体
type EndpointUpdateRequest struct {
	Name      *string  `json:"name"`
	BoundTags []string `json:"bound_tags"`
	Enabled   *bool    `json:"enabled"`
}

// ImportSourceRequest 导入规则中的单个源
type ImportSourceRequest struct {
	Name         string             `json:"name"`
	URL          string             `json:"url"`
	RespType     string             `json:"resp_type"`
	JsonPath     string             `json:"json_path"`
	Weight       int                `json:"weight"`
	Categories   []string           `json:"categories"`
	Headers      map[string]string  `json:"headers"`
	Enabled      bool               `json:"enabled"`
	DefaultQuery string             `json:"default_query"`
	Params       []model.QueryParam `json:"params"`
}

// ImportRequest POST /api/import 请求体
type ImportRequest struct {
	Sources []ImportSourceRequest `json:"sources"`
}
