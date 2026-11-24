package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/model"
	"github.com/tidwall/gjson"
)

type DebugSourceState struct {
	Enabled    bool   `json:"enabled"`
	Switching  bool   `json:"switching"`
	SourceID   int64  `json:"source_id,omitempty"`
	SourceName string `json:"source_name,omitempty"`
}

// Debug state is deliberately absent from settings, imports and the database.
func (e *Engine) DebugSource() DebugSourceState {
	state := DebugSourceState{}
	if current := e.debugState.Load(); current != nil {
		state = *current
	}
	state.Switching = e.debugSwitches.Load() > 0
	return state
}

var ErrDistributionModeChanged = errors.New("图源调试状态已切换，请重试")

// Workers read the current context under modeMu. Mode changes serialize through modeSwitchMu.
func (e *Engine) modeContext() context.Context {
	e.modeInit.Do(func() {
		e.modeCtx, e.modeCancel = context.WithCancelCause(context.Background())
	})
	return e.modeCtx
}

func (e *Engine) lockModeSwitch() func() {
	e.modeSwitchMu.Lock()
	e.debugSwitches.Add(1)
	e.modeContext()
	// Stop upstream I/O first. The lock still lets active cache writes finish
	// before clients are told that the mode changed.
	e.modeCancel(ErrDistributionModeChanged)
	e.modeMu.Lock()
	return func() {
		e.modeCtx, e.modeCancel = context.WithCancelCause(context.Background())
		e.debugSwitches.Add(-1)
		e.modeMu.Unlock()
		e.modeSwitchMu.Unlock()
	}
}

func (e *Engine) SetDebugSource(id int64) (DebugSourceState, error) {
	if id <= 0 {
		return DebugSourceState{}, fmt.Errorf("请选择有效的图源")
	}
	src, err := e.store.GetSource(id)
	if err != nil {
		return DebugSourceState{}, fmt.Errorf("图源不存在")
	}
	unlock := e.lockModeSwitch()
	defer unlock()
	e.debugSourceID = id
	state := DebugSourceState{Enabled: true, SourceID: id, SourceName: src.Name}
	e.debugState.Store(&state)
	return state, nil
}

func (e *Engine) ClearDebugSource() {
	unlock := e.lockModeSwitch()
	defer unlock()
	e.debugSourceID = 0
	e.debugState.Store(&DebugSourceState{})
}

type debugImageResponseKey struct{}

// WithDebugImageResponse opts a client into receiving the exact debug image
// rather than downloading and potentially re-rolling the upstream URL.
func WithDebugImageResponse(ctx context.Context) context.Context {
	return context.WithValue(ctx, debugImageResponseKey{}, true)
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

func (e *Engine) debugImage(parent context.Context, category, format, orientation, ua string) (*Result, int, error) {
	src, err := e.store.GetSource(e.debugSourceID)
	if err != nil {
		return nil, 0, &DebugFetchError{Phase: "图源选择", SourceID: e.debugSourceID, Err: errors.New("调试图源不存在，请退出调试或重新选择图源")}
	}
	settings, err := e.store.GetSettings()
	if err != nil {
		return nil, 0, err
	}
	queryCats := splitCategory(category)
	orientation, explicitOrientation := resolveRequestOrientation(queryCats, orientation, ua)
	tags, err := e.store.GetTags()
	if err != nil {
		return nil, 0, err
	}
	// Allow explicit debugging of stopped sources, while reusing normal tag/branch selection.
	debugSource := *src
	debugSource.Enabled = true
	debugSource.Status = "normal"
	sources := []model.Source{debugSource}
	candidates := filterRequestSources(sources, queryCats, tags, orientation)
	if len(candidates) == 0 && len(queryCats) == 1 && queryCats[0] == orientation &&
		(orientation == "horizontal" || orientation == "vertical") {
		// A direction-only request can still sample an untagged or adaptive
		// branch of this source. Decoded dimensions must match below.
		for _, candidate := range filterSources(sources, nil, tags) {
			if isUncategorized(candidate.Categories) ||
				(len(candidate.Categories) == 1 && candidate.Categories[0] == "adaptive" && !isExclusiveTag("adaptive", tags)) {
				candidates = append(candidates, candidate)
			}
		}
	}
	if len(candidates) == 0 {
		return nil, 0, &DebugFetchError{Phase: "分支匹配", SourceID: src.ID, Err: errors.New("调试图源没有匹配分类的主地址或分支")}
	}

	timeout := time.Duration(settings.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout+debugImageDownloadTimeout)
	defer cancel()
	wantImage, _ := parent.Value(debugImageResponseKey{}).(bool)
	singleBranch := len(candidates) == 1
	var lastErr error
	for attempt := 0; attempt < 3 && len(candidates) > 0; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		index := weightedPickIndex(candidates)
		selected := candidates[index]
		candidates = append(candidates[:index], candidates[index+1:]...)
		direction := requestOrientation(selected, queryCats, orientation, explicitOrientation)
		result, code, err := e.fetchDebugImage(ctx, selected, settings.ProxyMode, format, direction, ua, timeout, wantImage)
		if err == nil {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
			if err := e.store.RecordStats(queryCats, selected, result.URL, nil, ""); err != nil {
				logger.Error("record debug stats: %v", err)
			}
			return result, code, nil
		}
		lastErr = err
		// A mixed-orientation random endpoint can provide the right image on
		// a fresh draw even when this is the only configured matching branch.
		if singleBranch && attempt == 0 && errors.Is(err, errDebugOrientationMismatch) {
			candidates = append(candidates, selected)
		}
	}
	return nil, 0, lastErr
}

var errDebugOrientationMismatch = errors.New("调试图片方向不匹配")

func (e *Engine) fetchDebugImage(parent context.Context, selected model.Source, proxyMode bool, format, orientation, ua string, timeout time.Duration, wantImage bool) (result *Result, code int, err error) {
	phase := "图源接口"
	budget := timeout
	target := selected.URL
	requestCtx, cancelRequest := context.WithCancelCause(parent)
	activeCtx := requestCtx
	timer := time.AfterFunc(timeout, func() { cancelRequest(context.DeadlineExceeded) })
	defer func() { timer.Stop(); cancelRequest(context.Canceled) }()
	defer func() {
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if cause := context.Cause(activeCtx); cause != nil {
					err = cause
				}
			}
			var urlError *url.Error
			if errors.As(err, &urlError) {
				err = urlError.Err
			}
			err = &DebugFetchError{Phase: phase, SourceID: selected.ID, Address: debugRequestAddress(target), Budget: budget, Err: err}
		}
	}()
	client := *e.httpClient
	client.Timeout = 0
	req, err := newSourceRequest(requestCtx, selected, ua)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("调试图源 %s 请求失败：%w", selected.Name, err)
	}
	defer resp.Body.Close()
	if (resp.StatusCode < 200 || resp.StatusCode >= 300) && !isRedirectStatus(resp.StatusCode) {
		var upstream struct{ Error struct{ Code string } }
		body, _ := readDebugBody(resp.Body, 64<<10)
		if resp.StatusCode == http.StatusNotFound && json.Unmarshal(body, &upstream) == nil && upstream.Error.Code == "NO_MATCH" {
			return nil, 0, fmt.Errorf("调试图源 #%d（%s）没有符合当前筛选条件的图片（上游 HTTP 404 / NO_MATCH），请求地址：%s", selected.ID, selected.Name, debugRequestAddress(selected.URL))
		}
		return nil, 0, fmt.Errorf("调试图源 #%d（%s）返回 HTTP %d，请求地址：%s", selected.ID, selected.Name, resp.StatusCode, debugRequestAddress(selected.URL))
	}
	direct := strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "image/") && !isRedirectStatus(resp.StatusCode)
	imageURL := selected.URL
	if direct {
		if !timer.Stop() {
			return nil, 0, context.DeadlineExceeded
		}
		phase = "图片下载"
		budget = debugImageDownloadTimeout
		timer = time.AfterFunc(budget, func() { cancelRequest(context.DeadlineExceeded) })
	}
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
	if !direct {
		if !timer.Stop() {
			return nil, 0, context.DeadlineExceeded
		}
		resp.Body.Close()
		cancelRequest(context.Canceled)
	}
	result = &Result{Debug: true, URL: imageURL, SourceID: selected.ID, SourceName: selected.Name, Categories: selected.Categories}
	if orientation == "" && !wantImage {
		if format == "json" {
			return result, http.StatusOK, nil
		}
		if !proxyMode {
			return result, http.StatusFound, nil
		}
	}
	// Proxy into memory only: no disk cache, image metadata, monitor or pool writes.
	imageResponse := resp
	if !direct {
		phase = "图片下载"
		budget = debugImageDownloadTimeout
		target = imageURL
		downloadCtx, cancelDownload := context.WithTimeout(parent, budget)
		defer cancelDownload()
		activeCtx = downloadCtx
		imageReq, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, imageURL, nil)
		if err != nil {
			return nil, 0, err
		}
		imageReq.Header = buildImageRequestHeaders(selected.URL, selected.Headers)
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
	phase = "图片校验"
	cfg, kind, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 {
		return nil, 0, fmt.Errorf("调试响应不是有效图片")
	}
	result.Orientation = GetOrientation(cfg.Width, cfg.Height)
	if orientation != "" && result.Orientation != orientation {
		return nil, 0, fmt.Errorf("%w：实际 %s，请求 %s", errDebugOrientationMismatch, result.Orientation, orientation)
	}
	result.Width = cfg.Width
	result.Height = cfg.Height
	result.Format = kind
	if wantImage {
		result.ImageData = data
		result.ContentType = http.DetectContentType(data)
		return result, http.StatusOK, nil
	}
	if format == "json" {
		return result, http.StatusOK, nil
	}
	if !proxyMode {
		return result, http.StatusFound, nil
	}
	result.ImageData = data
	result.ContentType = http.DetectContentType(data)
	return result, http.StatusOK, nil
}

// Keep credentials and query values out of errors returned by public distribution APIs.
func debugRequestAddress(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "地址格式不正确"
	}
	u.User = nil
	safe := url.Values{}
	for _, key := range []string{"orientation", "r18", "format", "delivery"} {
		value := u.Query().Get(key)
		switch value {
		case "landscape", "portrait", "square", "horizontal", "vertical", "0", "1", "json", "proxy", "original":
			safe.Set(key, value)
		}
	}
	u.RawQuery = safe.Encode()
	u.Fragment = ""
	return u.String()
}

const debugImageDownloadTimeout = 30 * time.Second

type DebugFetchError struct {
	Phase    string
	SourceID int64
	Address  string
	Budget   time.Duration
	Err      error
}

func (e *DebugFetchError) Error() string {
	if e.Address == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("调试取图%s失败（图源 #%d，阶段时限 %s，请求地址：%s）：%v", e.Phase, e.SourceID, e.Budget, e.Address, e.Err)
}
func (e *DebugFetchError) Unwrap() error { return e.Err }
