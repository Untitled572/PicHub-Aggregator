package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/model"
	"github.com/tidwall/gjson"
)

type DebugSourceState struct {
	Enabled    bool   `json:"enabled"`
	SourceID   int64  `json:"source_id,omitempty"`
	SourceName string `json:"source_name,omitempty"`
}

// Debug state is deliberately absent from settings, imports and the database.
func (e *Engine) DebugSource() DebugSourceState {
	e.modeMu.RLock()
	defer e.modeMu.RUnlock()
	state := DebugSourceState{Enabled: e.debugSourceID != 0, SourceID: e.debugSourceID}
	if state.Enabled {
		if src, err := e.store.GetSource(state.SourceID); err == nil {
			state.SourceName = src.Name
		}
	}
	return state
}

func (e *Engine) SetDebugSource(id int64) (DebugSourceState, error) {
	if id <= 0 {
		return DebugSourceState{}, fmt.Errorf("请选择有效的图源")
	}
	// Wait for existing distribution/cache writes before acknowledging the switch.
	e.modeMu.Lock()
	defer e.modeMu.Unlock()
	src, err := e.store.GetSource(id)
	if err != nil {
		return DebugSourceState{}, fmt.Errorf("图源不存在")
	}
	e.debugSourceID = id
	return DebugSourceState{Enabled: true, SourceID: id, SourceName: src.Name}, nil
}
func (e *Engine) ClearDebugSource() {
	e.modeMu.Lock()
	defer e.modeMu.Unlock()
	e.debugSourceID = 0
}

func readDebugBody(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("调试响应超过大小限制")
	}
	return data, nil
}

func (e *Engine) debugImage(category, format, orientation, ua string) (result *Result, code int, err error) {
	src, err := e.store.GetSource(e.debugSourceID)
	if err != nil {
		return nil, 0, fmt.Errorf("调试图源不存在，请退出调试或重新选择图源")
	}
	settings, err := e.store.GetSettings()
	if err != nil {
		return nil, 0, err
	}
	queryCats := splitCategory(category)
	tags, err := e.store.GetTags()
	if err != nil {
		return nil, 0, err
	}
	// Allow explicit debugging of stopped sources, while reusing normal tag/branch selection.
	debugSource := *src
	debugSource.Enabled = true
	debugSource.Status = "normal"
	candidates := filterSources([]model.Source{debugSource}, queryCats, tags)
	if len(candidates) == 0 {
		return nil, 0, fmt.Errorf("调试图源没有匹配分类的主地址或分支")
	}
	selected := weightedPick(candidates)
	if orientation == "" && (len(queryCats) == 0 || (len(queryCats) == 1 && queryCats[0] == "adaptive")) {
		orientation = detectOrientationFromUA(ua)
	}
	defer func() {
		if err == nil && result != nil {
			if statErr := e.store.RecordStats(queryCats, selected, result.URL, nil, ""); statErr != nil {
				logger.Error("record debug stats: %v", statErr)
			}
		}
	}()
	timeout := time.Duration(settings.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := newSourceRequest(ctx, selected, ua)
	if err != nil {
		return nil, 0, err
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("调试图源 %s 请求失败：%w", src.Name, err)
	}
	defer resp.Body.Close()
	if (resp.StatusCode < 200 || resp.StatusCode >= 300) && !isRedirectStatus(resp.StatusCode) {
		return nil, 0, fmt.Errorf("调试图源返回 HTTP %d", resp.StatusCode)
	}
	direct := strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "image/") && !isRedirectStatus(resp.StatusCode)
	imageURL := selected.URL
	if !direct {
		body, err := readDebugBody(resp.Body, 1<<20)
		if err != nil {
			return nil, 0, err
		}
		// Reuse the health validator so HTML and empty JSON URLs cannot pass as images.
		copyResponse := *resp
		copyResponse.Body = io.NopCloser(bytes.NewReader(body))
		if err := validateHealthResponse(&copyResponse, selected); err != nil {
			return nil, 0, err
		}
		if isRedirectStatus(resp.StatusCode) && resp.Header.Get("Location") != "" {
			imageURL = resolveURL(selected.URL, resp.Header.Get("Location"))
		} else if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "json") || selected.RespType == "json" {
			path := selected.JsonPath
			if path == "" {
				path = "url"
			}
			imageURL = resolveURL(selected.URL, gjson.GetBytes(body, path).String())
		} else {
			imageURL = resolveURL(selected.URL, strings.TrimSpace(string(body)))
		}
	}
	result = &Result{Debug: true, URL: imageURL, SourceID: src.ID, SourceName: selected.Name, Categories: selected.Categories}
	if orientation == "" {
		if format == "json" {
			return result, http.StatusOK, nil
		}
		if !settings.ProxyMode {
			return result, http.StatusFound, nil
		}
	}
	// Proxy into memory only: no disk cache, image metadata, monitor or pool writes.
	imageResponse := resp
	if !direct {
		imageReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
		if err != nil {
			return nil, 0, err
		}
		imageReq.Header = buildImageRequestHeaders(selected.URL, selected.Headers)
		client := *e.httpClient
		client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("图片重定向过多")
			}
			return nil
		}
		imageResponse, err = client.Do(imageReq)
		if err != nil {
			return nil, 0, err
		}
		defer imageResponse.Body.Close()
	}
	if imageResponse.StatusCode < 200 || imageResponse.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("图片下载返回 HTTP %d", imageResponse.StatusCode)
	}
	data, err := readDebugBody(imageResponse.Body, 32<<20)
	if err != nil {
		return nil, 0, err
	}
	cfg, kind, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 {
		return nil, 0, fmt.Errorf("调试响应不是有效图片")
	}
	result.Orientation = GetOrientation(cfg.Width, cfg.Height)
	if orientation != "" && result.Orientation != orientation {
		return nil, 0, fmt.Errorf("调试图片方向不匹配：实际 %s，请求 %s", result.Orientation, orientation)
	}
	result.Width = cfg.Width
	result.Height = cfg.Height
	result.Format = kind
	if format == "json" {
		return result, http.StatusOK, nil
	}
	if !settings.ProxyMode {
		return result, http.StatusFound, nil
	}
	result.ImageData = data
	result.ContentType = http.DetectContentType(data)
	return result, http.StatusOK, nil
}
