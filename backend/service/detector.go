package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type DetectResult struct {
	RespType string                 `json:"resp_type"`
	Headers  map[string]string      `json:"headers"`
	BodyTree interface{}            `json:"body_tree,omitempty"`
	URLHints []string               `json:"url_hints,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

func DetectURL(targetURL string) (*DetectResult, error) {
	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "PicHub-Aggregator/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	result := &DetectResult{
		RespType: "unknown",
		Headers:  make(map[string]string),
	}

	for k, v := range resp.Header {
		if len(v) > 0 {
			result.Headers[k] = v[0]
		}
	}

	ct := resp.Header.Get("Content-Type")

	if resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 || resp.StatusCode == 307 || resp.StatusCode == 308 {
		loc := resp.Header.Get("Location")
		if loc != "" {
			result.RespType = "redirect"
			result.URLHints = []string{loc}
			return result, nil
		}
	}

	if strings.HasPrefix(ct, "image/") {
		result.RespType = "image"
		result.URLHints = []string{targetURL}
		return result, nil
	}

	if strings.HasPrefix(ct, "application/json") {
		result.RespType = "json"
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			result.Error = fmt.Sprintf("read body: %v", err)
			return result, nil
		}
		var bodyTree interface{}
		if json.Valid(body) {
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
	return hints
}

func isURLString(s string) bool {
	s = strings.ToLower(s)
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
