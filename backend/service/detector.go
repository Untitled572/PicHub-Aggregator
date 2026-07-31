package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type DetectResult struct {
	RespType string            `json:"resp_type"`
	Headers  map[string]string `json:"headers"`
	BodyTree interface{}       `json:"body_tree,omitempty"`
	URLHints []string          `json:"url_hints,omitempty"`
	FinalURL string            `json:"final_url,omitempty"`
	Error    string            `json:"error,omitempty"`
}

const maxDetectRedirects = 5

func DetectURL(targetURL string) (*DetectResult, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	result := &DetectResult{
		RespType: "unknown",
		Headers:  make(map[string]string),
	}

	current := targetURL
	redirects := 0

	for {
		req, err := http.NewRequest("GET", current, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("User-Agent", "PicHub-Aggregator/1.0")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		// 跟随重定向, 以最终响应判定类型
		loc := resp.Header.Get("Location")
		if isRedirectStatus(resp.StatusCode) && loc != "" {
			resp.Body.Close()
			redirects++
			if redirects > maxDetectRedirects {
				result.RespType = "redirect"
				result.URLHints = []string{resolveURL(current, loc)}
				result.FinalURL = current
				return result, nil
			}
			current = resolveURL(current, loc)
			continue
		}

		defer resp.Body.Close()

		for k, v := range resp.Header {
			if len(v) > 0 {
				result.Headers[k] = v[0]
			}
		}
		result.FinalURL = current

		ct := resp.Header.Get("Content-Type")

		// 3xx 但无 Location: 保留现有行为, 直接读 body
		if isRedirectStatus(resp.StatusCode) {
			result.RespType = "redirect"
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				text := strings.TrimSpace(string(body))
				if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
					result.URLHints = []string{text}
				}
			}
			return result, nil
		}

		// 最终为图片: 若经历重定向则归类 redirect (直链源), 否则 image
		if strings.HasPrefix(ct, "image/") {
			if redirects > 0 {
				result.RespType = "redirect"
			} else {
				result.RespType = "image"
			}
			result.URLHints = []string{current}
			return result, nil
		}

		if strings.HasPrefix(ct, "application/json") {
			result.RespType = "json"
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				result.Error = fmt.Sprintf("read body: %v", err)
				return result, nil
			}
			if json.Valid(body) {
				var bodyTree interface{}
				if err := json.Unmarshal(body, &bodyTree); err != nil {
					result.Error = fmt.Sprintf("json parse: %v", err)
					return result, nil
				}
				result.BodyTree = bodyTree
				result.URLHints = findURLFields("", body, 0)
			}
			return result, nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			result.Error = fmt.Sprintf("read body: %v", err)
			return result, nil
		}
		if len(body) > 0 && json.Valid(body) {
			result.RespType = "json"
			var bodyTree interface{}
			if err := json.Unmarshal(body, &bodyTree); err != nil {
				result.Error = fmt.Sprintf("json parse: %v", err)
				return result, nil
			}
			result.BodyTree = bodyTree
			result.URLHints = findURLFields("", body, 0)
			return result, nil
		}

		return result, nil
	}
}

func isRedirectStatus(code int) bool {
	return code == 301 || code == 302 || code == 303 || code == 307 || code == 308
}

func findURLFields(prefix string, data []byte, depth int) []string {
	if depth > 10 {
		return nil
	}
	var hints []string
	result := gjson.GetBytes(data, "@this")
	result.ForEach(func(key, value gjson.Result) bool {
		var fullPath string
		if prefix == "" {
			fullPath = key.String()
		} else {
			fullPath = prefix + "." + key.String()
		}
		if value.IsObject() || value.IsArray() {
			subHints := findURLFields(fullPath, []byte(value.Raw), depth+1)
			hints = append(hints, subHints...)
		} else if value.Type == gjson.String {
			str := value.String()
			if isURLString(str) {
				hints = append(hints, fullPath)
			}
		}
		return true
	})
	sort.SliceStable(hints, func(i, j int) bool {
		return hintPriority(hints[i]) > hintPriority(hints[j])
	})
	return hints
}

func isURLString(s string) bool {
	s = strings.ToLower(s)
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func hintPriority(path string) int {
	lower := strings.ToLower(path)
	parts := strings.Split(lower, ".")
	key := ""
	if len(parts) > 0 {
		key = parts[len(parts)-1]
	}
	// 去除引号: 0.['url'] → url
	key = strings.Trim(key, "'\"[]")

	// 图片相关字段优先
	switch {
	case key == "url" || key == "img_url" || key == "image_url":
		return 10
	case key == "src" || key == "image" || key == "file" || key == "file_url":
		return 9
	case strings.Contains(key, "img") || strings.Contains(key, "image") || strings.Contains(key, "src"):
		return 7
	case key == "download" || key == "pic" || key == "thumbnail":
		return 6
	}

	// 页面/链接类字段降权
	switch {
	case key == "page" || key == "link" || key == "href":
		return 2
	case key == "detail" || key == "origin" || key == "original":
		return 3
	}

	return 5
}
