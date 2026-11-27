package service

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"

	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/monitor"
	"github.com/pichub/backend/store"
)

const (
	prefetchInterval            = 2 * time.Second
	unlimitedPrefetchInterval   = time.Second
	prefetchSourceInterval      = 10 * time.Second
	prefetchTagReserve          = 15
	distributionTimeout         = 8 * time.Second
	distributionDownloadTimeout = 4 * time.Second
	// maxPoolPerSource 单源池条目封顶
	maxPoolPerSource = 8
	// minPoolPerSource 单源池条目下限
	minPoolPerSource = 1
)

type Engine struct {
	modeMu       sync.RWMutex
	modeSwitchMu sync.Mutex
	modeInit     sync.Once
	modeCtx      context.Context
	modeCancel   context.CancelCauseFunc
	replenishMu  sync.Mutex
	// Only accessed under replenishMu.
	nextPrefetch             time.Time
	ordinaryPrefetchInFlight bool
	prefetchInFlight         map[int64]bool
	unlimitedPrefetchPace    map[int64]time.Time
	prefetchCooldown         map[string]time.Time
	prefetchPace             map[string]time.Time
	debugSourceID            int64
	debugState               atomic.Pointer[DebugSourceState]
	debugSwitches            atomic.Int32
	store                    *store.Store
	proxyCache               *ProxyCache
	imageStore               *ImageStore
	proxyConfig              *ProxyConfig
	distPool                 *DistributionPool
	demandTracker            *DemandTracker
	sourceDemand             *SourceDemandTracker
	httpClient               *http.Client
	monitor                  *monitor.SourceMonitor
	healthChecker            *HealthChecker
}

func NewEngine(st *store.Store, pc *ProxyCache, imgStore *ImageStore, proxyCfg *ProxyConfig, mon *monitor.SourceMonitor, healthChecker ...*HealthChecker) *Engine {
	transport := &http.Transport{}
	if proxyCfg != nil {
		transport.Proxy = proxyCfg.Proxy
	}
	eng := &Engine{
		store:         st,
		proxyCache:    pc,
		imageStore:    imgStore,
		proxyConfig:   proxyCfg,
		distPool:      NewDistributionPool(),
		demandTracker: NewDemandTracker(),
		sourceDemand:  NewSourceDemandTracker(),
		monitor:       mon,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	if len(healthChecker) > 0 {
		eng.healthChecker = healthChecker[0]
	}
	if err := eng.restorePrefetchPool(); err != nil {
		logger.Error("restore prefetch pool: %v", err)
	}
	go eng.StartDistributionWorker()
	return eng
}

type Result struct {
	Debug           bool     `json:"debug,omitempty"`
	ImageData       []byte   `json:"-"`
	ContentType     string   `json:"-"`
	URL             string   `json:"url"`
	LocalURL        string   `json:"local_url,omitempty"`
	SourceName      string   `json:"source"`
	SourceID        int64    `json:"source_id,omitempty"`
	Categories      []string `json:"categories"`
	Orientation     string   `json:"orientation,omitempty"`
	FileID          string   `json:"file_id,omitempty"`
	Width           int      `json:"width,omitempty"`
	Height          int      `json:"height,omitempty"`
	Format          string   `json:"format,omitempty"`
	ImageID         int64    `json:"image_id,omitempty"`
	DistributionRef string   `json:"distribution_ref,omitempty"`
}

func detectOrientationFromUA(ua string) string {
	if ua == "" {
		return ""
	}
	uaLower := strings.ToLower(ua)
	isMobile := strings.Contains(uaLower, "mobile") ||
		strings.Contains(uaLower, "iphone") ||
		strings.Contains(uaLower, "ipad") ||
		strings.Contains(uaLower, "ipod") ||
		strings.Contains(uaLower, "android")
	isDesktop := strings.Contains(uaLower, "windows") ||
		strings.Contains(uaLower, "macintosh") ||
		strings.Contains(uaLower, "linux") ||
		strings.Contains(uaLower, "cros") ||
		strings.Contains(uaLower, "x11")

	if isMobile {
		return "vertical"
	}
	if isDesktop {
		return "horizontal"
	}
	return ""
}

func (e *Engine) RandomImage(category string, format string, orientation string, clientUA string) (*Result, int, error) {
	return e.RandomImageContext(context.Background(), category, format, orientation, clientUA)
}

func (e *Engine) RandomImageContext(ctx context.Context, category, format, orientation, clientUA string) (*Result, int, error) {
	e.modeMu.RLock()
	defer e.modeMu.RUnlock()

	mode := e.modeContext()
	ctx, cancel := context.WithCancelCause(ctx)
	stop := context.AfterFunc(mode, func() { cancel(context.Cause(mode)) })
	defer stop()
	defer cancel(nil)
	if cause := context.Cause(mode); cause != nil {
		return nil, 0, cause
	}

	var result *Result
	var status int
	var err error
	if e.debugSourceID != 0 {
		result, status, err = e.debugImage(ctx, category, format, orientation, clientUA)
	} else {
		result, status, err = e.randomImage(ctx, category, format, orientation, clientUA)
	}
	if err != nil && errors.Is(context.Cause(mode), ErrDistributionModeChanged) {
		return nil, 0, ErrDistributionModeChanged
	}
	return result, status, err
}

func (e *Engine) randomImage(parent context.Context, category, format, orientation, clientUA string) (result *Result, code int, err error) {
	budget, stop := context.WithTimeout(parent, distributionTimeout)
	defer stop()
	if err := budget.Err(); err != nil {
		return nil, 0, err
	}

	defer func() {
		if result != nil && result.FileID != "" && result.SourceID > 0 && e.imageStore != nil {
			e.imageStore.MoveToRoot(result.FileID, result.SourceID)
		}
	}()
	settings, err := e.store.GetSettings()
	if err != nil {
		return nil, 0, fmt.Errorf("get settings: %w", err)
	}

	queryCats := splitCategory(category)
	orientation, explicitOrientation := resolveRequestOrientation(queryCats, orientation, clientUA)

	tags, _ := e.store.GetTags()
	exclSet := exclusiveTagSet(tags)

	if settings.ProxyMode && e.distPool != nil {
		var poolResult *Result
		single := len(queryCats) == 1
		hasExcl := hasExclusiveTag(tags, queryCats)
		for {
			switch {
			case orientation != "" && len(queryCats) > 0:
				poolResult = e.distPool.PopMatching(func(entry *PoolEntry) bool {
					return matchRequestCategories(entry.Categories, queryCats, tags, single, hasExcl) &&
						entry.Orientation == orientation
				})
			case orientation != "":
				poolResult = e.distPool.PopByOrientation(orientation, exclSet)
			case len(queryCats) > 0:
				poolResult = e.distPool.PopMatching(func(entry *PoolEntry) bool {
					return matchRequestCategories(entry.Categories, queryCats, tags, single, hasExcl)
				})
			default:
				poolResult = e.distPool.PopAny(exclSet)
			}

			if poolResult == nil || e.imageStore == nil {
				break
			}
			if _, _, err := e.imageStore.GetImage(poolResult.FileID); err == nil {
				break
			}
			logger.System("pool entry %s no longer exists on disk, skip", poolResult.FileID)
			if err := budget.Err(); err != nil {
				return nil, 0, err
			}
		}

		if poolResult != nil {
			e.demandTracker.RecordRequest(queryCats, true)
			e.sourceDemand.RecordSelection(poolResult.SourceID)
			src := model.Source{
				ID:   poolResult.SourceID,
				Name: poolResult.SourceName,
			}
			imgID := poolResult.ImageID
			if err := e.store.SetImagePooled(imgID, false); err != nil {
				e.distPool.Push(poolEntryFromResult(poolResult))
				return nil, 0, fmt.Errorf("mark prefetched image distributed: %w", err)
			}
			poolResult.DistributionRef = e.recordDistribution(queryCats, src, poolResult.LocalURL, &imgID, poolResult.FileID, format)
			logger.System("pool hit for category %q, instant response", category)

			if format == "json" {
				return poolResult, http.StatusOK, nil
			}
			return poolResult, http.StatusFound, nil
		}
	}

	if e.demandTracker != nil {
		e.demandTracker.RecordRequest(queryCats, false)
	}
	sources, err := e.store.ListSources()
	if err != nil {
		return nil, 0, fmt.Errorf("list sources: %w", err)
	}

	candidates := filterRequestSources(sources, queryCats, tags, orientation)
	if len(candidates) == 0 {
		return nil, 0, fmt.Errorf("no available sources")
	}

	timeout := time.Duration(settings.Timeout) * time.Millisecond

	maxAttempts := 3
	if orientation != "" && (hasCategory(queryCats, "adaptive") || (e.imageStore != nil && settings.ProxyMode)) {
		maxAttempts = 8
	}

	for attempt := 0; attempt < maxAttempts && len(candidates) > 0; attempt++ {
		if err := budget.Err(); err != nil {
			return nil, 0, err
		}
		selectedIndex := weightedPickIndex(candidates)
		selected := candidates[selectedIndex]
		candidates = append(candidates[:selectedIndex], candidates[selectedIndex+1:]...)

		attemptDeadline, _ := budget.Deadline()
		if len(candidates) > 0 {
			// Reserve time for another branch if the API or image download stalls.
			now := time.Now()
			attemptDeadline = now.Add(attemptDeadline.Sub(now) / 2)
		}
		checkOrientation := requestOrientation(selected, queryCats, orientation, explicitOrientation)

		ctx, cancel := context.WithTimeout(budget, min(timeout, time.Until(attemptDeadline)))
		req, err := e.newFetchRequest(ctx, selected, clientUA)
		if err != nil {
			cancel()
			continue
		}

		resp, err := e.httpClient.Do(req)
		if err != nil {
			cancel()
			if budget.Err() != nil {
				return nil, 0, budget.Err()
			}
			logger.Error("source %q fetch failed: %v", selected.Name, err)
			if e.monitor != nil {
				e.monitor.Emit(selected.ID, false)
			}
			continue
		}

		hasFailed := false
		var cachedInfo *CachedImageInfo

		if resp.StatusCode >= 500 {
			logger.Error("source %q returned %d", selected.Name, resp.StatusCode)
			if e.monitor != nil {
				e.monitor.Emit(selected.ID, false)
			}
			resp.Body.Close()
			cancel()
			continue
		}

		if e.monitor != nil {
			e.monitor.Emit(selected.ID, true)
		}

		imageURL, err := extractImageURL(resp, selected)
		resp.Body.Close()
		cancel()
		if err != nil {
			continue
		}

		e.store.UpdateSourceStatus(selected.ID, "normal")

		origURL := imageURL

		if e.imageStore != nil && settings.ProxyMode {
			downloadCtx, cancelDownload := context.WithTimeout(budget, min(distributionDownloadTimeout, time.Until(attemptDeadline)))
			cached, cacheErr := e.imageStore.downloadAndStore(downloadCtx, origURL, selected.URL, selected.ID, selected.Name, queryCats, selected.Headers, false)
			cancelDownload()
			if cacheErr != nil {
				if budget.Err() != nil {
					return nil, 0, budget.Err()
				}
				logger.Error("cache download failed for %s: %v", selected.Name, cacheErr)
				if e.monitor != nil {
					e.monitor.Emit(selected.ID, false)
				}
				continue
			}
			cachedInfo = cached

			if checkOrientation != "" {
				imgOrient := GetOrientation(cached.Width, cached.Height)
				if imgOrient != checkOrientation {
					logger.System("orientation mismatch for %s: got %s, wanted %s", selected.Name, imgOrient, orientation)
					continue
				}
			}

			imageURL = "/images/" + cached.FileID
		} else if e.proxyCache != nil && settings.ProxyMode {
			proxyCtx, cancelProxy := context.WithDeadline(budget, attemptDeadline)
			_, _, proxyErr := e.proxyCache.getOrFetch(proxyCtx, imageURL)
			cancelProxy()
			if proxyErr != nil {
				hasFailed = true
			}
		}

		if cachedInfo == nil && hasCategory(queryCats, "adaptive") && checkOrientation != "" && !hasDirectionTag(selected.Categories) {
			if err := checkRemoteOrientationContext(budget, selected, imageURL, checkOrientation, clientUA, min(timeout, time.Until(attemptDeadline)), e.httpClient); err != nil {
				logger.System("adaptive image rejected for %s: %v", selected.Name, err)
				continue
			}
		}

		if hasFailed {
			continue
		}

		var imgID *int64
		var fileID string
		if cachedInfo != nil {
			imgID = &cachedInfo.ID
			fileID = cachedInfo.FileID
		}
		e.sourceDemand.RecordSelection(selected.ID)
		ref := e.recordDistribution(queryCats, selected, imageURL, imgID, fileID, format)

		res := &Result{
			URL:             origURL,
			SourceName:      selected.Name,
			Categories:      selected.Categories,
			DistributionRef: ref,
		}
		if cachedInfo != nil {
			res.LocalURL = imageURL
			res.SourceID = selected.ID
			res.FileID = cachedInfo.FileID
			res.Width = cachedInfo.Width
			res.Height = cachedInfo.Height
			res.Format = cachedInfo.Format
			res.ImageID = cachedInfo.ID
			res.Orientation = cachedInfo.Orientation
		}

		if format == "json" {
			return res, http.StatusOK, nil
		}
		return res, http.StatusFound, nil
	}

	if err := budget.Err(); err != nil {
		return nil, 0, err
	}
	return nil, 0, fmt.Errorf("all sources failed")
}

func (e *Engine) recordDistribution(categories []string, source model.Source, url string, imageID *int64, fileID string, format string) string {
	ref := ""
	if imageID != nil && *imageID > 0 && format == "json" {
		ref = cryptorand.Text()
	}
	if err := e.store.RecordDistribution(categories, source, url, imageID, fileID, ref); err != nil {
		logger.Error("record distribution stats: %v", err)
		return ""
	}
	return ref
}

func (e *Engine) newFetchRequest(ctx context.Context, selected model.Source, clientUA string) (*http.Request, error) {
	return newSourceRequest(ctx, selected, clientUA)
}

// newSourceRequest keeps health checks and distribution request headers consistent.
func newSourceRequest(ctx context.Context, selected model.Source, clientUA string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", selected.URL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range selected.Headers {
		req.Header.Set(k, v)
	}
	if clientUA != "" {
		hasUA := false
		for k := range selected.Headers {
			if strings.EqualFold(k, "User-Agent") {
				hasUA = true
				break
			}
		}
		if !hasUA || hasCategory(selected.Categories, "adaptive") {
			req.Header.Set("User-Agent", clientUA)
		}
	} else if !hasHeader(selected.Headers, "User-Agent") {
		req.Header.Set("User-Agent", defaultBrowserUA)
	}
	return req, nil
}

func hasExclusiveTag(tags []model.Tag, ids []string) bool {
	for _, id := range ids {
		for _, t := range tags {
			if t.ID == id && t.Exclusive {
				return true
			}
		}
	}
	return false
}

// exclusiveTagSet 收集所有 exclusive 标签 ID, 用于池消费隔离
func exclusiveTagSet(tags []model.Tag) map[string]bool {
	set := make(map[string]bool)
	for _, t := range tags {
		if t.Exclusive {
			set[t.ID] = true
		}
	}
	return set
}

// excludeExclusiveSources 过滤掉 Categories 含 exclusive 标签的源 (通用预取不得拉取 exclusive 内容)
func excludeExclusiveSources(sources []model.Source, tags []model.Tag) []model.Source {
	var result []model.Source
	for _, src := range sources {
		excl := false
		for _, cat := range src.Categories {
			if isExclusiveTag(cat, tags) {
				excl = true
				break
			}
		}
		if !excl {
			result = append(result, src)
		}
	}
	return result
}

func isExclusiveTag(id string, tags []model.Tag) bool {
	for _, t := range tags {
		if t.ID == id && t.Exclusive {
			return true
		}
	}
	return false
}

func isUncategorized(cats []string) bool {
	return len(cats) == 1 && cats[0] == "__uncategorized__"
}

func matchCategories(srcCats []string, queryCats []string, tags []model.Tag, single, hasExcl bool) bool {
	if len(queryCats) == 0 || queryCats[0] == "" {
		return true
	}

	if isUncategorized(srcCats) {
		if single {
			return false
		}
		return !hasExcl
	}

	// 源含有未选中的 exclusive tag → 排除
	for _, cat := range srcCats {
		if isExclusiveTag(cat, tags) {
			found := false
			for _, qc := range queryCats {
				if qc == cat {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return hasAnyCategory(srcCats, queryCats)
}

func filterSources(sources []model.Source, queryCats []string, tags []model.Tag) []model.Source {
	var result []model.Source
	hasQueryCat := len(queryCats) > 0 && queryCats[0] != ""
	single := hasQueryCat && len(queryCats) == 1
	hasExcl := hasExclusiveTag(tags, queryCats)

	for _, src := range sources {
		if !src.Enabled || src.Status == "error" {
			continue
		}
		if len(src.Categories) == 0 {
			src.Categories = []string{"__uncategorized__"}
		}

		if len(src.Params) > 0 {
			for _, param := range src.Params {
				key := strings.TrimSpace(param.Key)
				val := strings.TrimSpace(param.Value)
				if key == "" {
					continue
				}
				paramURL, subLabel := resolveSubEndpoint(src.URL, key, val)
				paramURL = appendDefaultQuery(paramURL, src.DefaultQuery)
				paramWeight := param.Weight
				if paramWeight <= 0 {
					paramWeight = src.Weight
				}
				paramCats := param.Categories
				if len(paramCats) == 0 {
					paramCats = src.Categories
				}

				if hasQueryCat {
					if !matchCategories(paramCats, queryCats, tags, single, hasExcl) {
						continue
					}
				}

				variant := src
				variant.Name = src.Name + " › " + subLabel
				variant.URL = paramURL
				variant.Weight = paramWeight
				variant.Categories = paramCats
				result = append(result, variant)
			}

			baseURL := appendDefaultQuery(src.URL, src.DefaultQuery)
			if len(src.Categories) > 0 && hasQueryCat {
				if matchCategories(src.Categories, queryCats, tags, single, hasExcl) {
					variant := src
					variant.URL = baseURL
					result = append(result, variant)
				}
			} else {
				variant := src
				variant.URL = baseURL
				result = append(result, variant)
			}
		} else {
			if len(src.Categories) > 0 && hasQueryCat {
				if !matchCategories(src.Categories, queryCats, tags, single, hasExcl) {
					continue
				}
			}
			variant := src
			variant.URL = appendDefaultQuery(src.URL, src.DefaultQuery)
			result = append(result, variant)
		}
	}
	return result
}

func appendDefaultQuery(baseURL, defaultQuery string) string {
	defaultQuery = strings.TrimSpace(defaultQuery)
	if defaultQuery == "" {
		return baseURL
	}
	if strings.Contains(baseURL, "?") {
		return baseURL + "&" + defaultQuery
	}
	return baseURL + "?" + defaultQuery
}

func buildURL(baseURL, key, value string) string {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" && value == "" {
		return baseURL
	}

	if key == "/" || strings.HasPrefix(value, "/") {
		path := value
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return strings.TrimRight(baseURL, "/") + path
	}

	if key == "" || strings.Contains(key, "=") {
		paramStr := key
		if paramStr == "" {
			paramStr = value
		}
		if strings.Contains(baseURL, "?") {
			return fmt.Sprintf("%s&%s", baseURL, paramStr)
		}
		return fmt.Sprintf("%s?%s", baseURL, paramStr)
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		if strings.Contains(baseURL, "?") {
			return fmt.Sprintf("%s&%s=%s", baseURL, url.QueryEscape(key), url.QueryEscape(value))
		}
		return fmt.Sprintf("%s?%s=%s", baseURL, url.QueryEscape(key), url.QueryEscape(value))
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

func resolveSubEndpoint(mainURL, key, val string) (string, string) {
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)

	// Case 1: Full Sub-API URL
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		label := key
		if val != "" {
			label = fmt.Sprintf("%s (%s)", val, getRelativePathOrHost(key))
		}
		return key, label
	}

	// Case 2: Sub-API Path / Relative Link
	if strings.HasPrefix(key, "/") || (strings.Contains(key, ".") && !strings.Contains(key, "=") && !strings.Contains(key, "?")) {
		path := key
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		var targetURL string
		if u, err := url.Parse(mainURL); err == nil {
			relURL, err := url.Parse(path)
			if err == nil {
				targetURL = u.ResolveReference(relURL).String()
			} else {
				targetURL = strings.TrimRight(mainURL, "/") + path
			}
		} else {
			targetURL = strings.TrimRight(mainURL, "/") + path
		}
		label := path
		if val != "" {
			label = fmt.Sprintf("%s (%s)", val, path)
		}
		return targetURL, label
	}

	// Case 3: Query Parameter
	targetURL := buildURL(mainURL, key, val)
	label := key
	if val != "" {
		label = fmt.Sprintf("%s=%s", key, val)
	}
	return targetURL, label
}

func getRelativePathOrHost(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		if u.Path != "" && u.Path != "/" {
			return u.Path
		}
		return u.Host
	}
	return rawURL
}

func hasCategory(cats []string, target string) bool {
	for _, c := range cats {
		if strings.EqualFold(c, target) {
			return true
		}
	}
	return false
}

func hasAnyCategory(srcCats, queryCats []string) bool {
	for _, qc := range queryCats {
		qc = strings.TrimSpace(qc)
		for _, sc := range srcCats {
			if strings.EqualFold(sc, qc) {
				return true
			}
		}
	}
	return false
}

func weightedPick(sources []model.Source) model.Source {
	return sources[weightedPickIndex(sources)]
}

func weightedPickIndex(sources []model.Source) int {
	totalWeight := 0
	for _, src := range sources {
		totalWeight += src.Weight
	}
	if totalWeight == 0 {
		return rand.Intn(len(sources))
	}
	r := rand.Intn(totalWeight)
	for i, src := range sources {
		r -= src.Weight
		if r < 0 {
			return i
		}
	}
	return len(sources) - 1
}

func extractImageURL(resp *http.Response, src model.Source) (string, error) {
	reqURL := src.URL
	if resp.Request != nil && resp.Request.URL != nil {
		reqURL = resp.Request.URL.String()
	}

	if resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 || resp.StatusCode == 307 || resp.StatusCode == 308 {
		loc := resp.Header.Get("Location")
		if loc != "" {
			loc = resolveURL(reqURL, loc)
			return resolveBaiduURL(loc), nil
		}
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			text := strings.TrimSpace(string(body))
			if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
				return resolveBaiduURL(text), nil
			}
		}
	}

	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "image/") {
		return reqURL, nil
	}

	if strings.HasPrefix(ct, "application/json") || src.RespType == "json" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read body: %w", err)
		}
		path := src.JsonPath
		if path == "" {
			path = "url"
		}
		result := gjson.Get(string(body), path)
		if result.Exists() {
			return resolveURL(reqURL, result.String()), nil
		}
		return "", fmt.Errorf("json path %s not found", path)
	}

	body, err := io.ReadAll(resp.Body)
	if err == nil {
		text := strings.TrimSpace(string(body))
		if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
			return resolveBaiduURL(text), nil
		}
	}

	return reqURL, nil
}

func resolveBaiduURL(rawURL string) string {
	if !strings.Contains(rawURL, "image.baidu.com/search/down") && !strings.Contains(rawURL, "search/down") {
		return rawURL
	}
	if u, err := url.Parse(rawURL); err == nil {
		if realURL := u.Query().Get("url"); realURL != "" &&
			(strings.HasPrefix(realURL, "http://") || strings.HasPrefix(realURL, "https://")) {
			return realURL
		}
	}
	return rawURL
}

func resolveURL(baseURLStr, targetURLStr string) string {
	targetURLStr = strings.TrimSpace(targetURLStr)
	if targetURLStr == "" {
		return baseURLStr
	}
	if strings.HasPrefix(targetURLStr, "http://") || strings.HasPrefix(targetURLStr, "https://") {
		return targetURLStr
	}
	base, err := url.Parse(baseURLStr)
	if err != nil {
		return targetURLStr
	}
	rel, err := url.Parse(targetURLStr)
	if err != nil {
		return targetURLStr
	}
	resolved := base.ResolveReference(rel)
	return resolved.String()
}

func (e *Engine) StartDistributionWorker() {
	time.Sleep(3 * time.Second)

	ticker := time.NewTicker(unlimitedPrefetchInterval)
	defer ticker.Stop()
	for tick := range ticker.C {
		go e.replenishPoolAt(tick)
	}
}

func (e *Engine) ReplenishPool() {
	e.replenishPool()
}

func (e *Engine) recordCacheHealth(result HealthResult) {
	if e.healthChecker != nil {
		e.healthChecker.RecordCacheResult(result)
		return
	}
	if err := e.store.RecordHealthResult(result.ID, result.Available, result.LatencyMs); err != nil {
		logger.Error("record cache health result for source %d: %v", result.ID, err)
	}
}

type prefetchChoice struct {
	candidates []model.Source
	tag        string
}

type prefetchBatch struct {
	mode    context.Context
	jobs    []model.Source
	timeout time.Duration
}

func (e *Engine) replenishPool() {
	e.replenishPoolAt(time.Now())
}

func (e *Engine) replenishPoolAt(now time.Time) {
	batch := e.planPrefetch(now)
	if batch == nil {
		return
	}
	var inFlight sync.WaitGroup
	for _, job := range batch.jobs {
		inFlight.Go(func() { e.runPrefetch(batch, job) })
	}
	inFlight.Wait()
}

func (e *Engine) planPrefetch(now time.Time) *prefetchBatch {
	// Select and reserve work under one lock. Independent network requests run outside it.
	e.modeMu.RLock()
	defer e.modeMu.RUnlock()
	if e.debugSourceID != 0 || e.debugSwitches.Load() > 0 || !e.replenishMu.TryLock() {
		return nil
	}
	defer e.replenishMu.Unlock()

	if e.distPool == nil || e.imageStore == nil {
		return nil
	}
	settings, err := e.store.GetSettings()
	if err != nil || !settings.ProxyMode || settings.PoolSize <= 0 {
		return nil
	}

	slots := settings.PoolSize - e.distPool.Size() - len(e.prefetchInFlight)
	if slots <= 0 {
		return nil
	}

	sources, err := e.store.ListSources()
	if err != nil {
		return nil
	}
	tags, _ := e.store.GetTags()
	sourceCounts := e.distPool.SourceCounts()
	caps := e.sourceCaps(settings.PoolSize)
	activeSources := 0
	for _, source := range sources {
		if source.Enabled && source.Status != "error" {
			activeSources++
		}
	}
	if activeSources > 0 {
		baseCap := min(maxPoolPerSource, (settings.PoolSize+activeSources-1)/activeSources)
		for _, source := range sources {
			caps[source.ID] = max(baseCap, caps[source.ID])
		}
	}
	choices, reservations := e.prefetchChoices(sources, tags, settings)
	var jobs []model.Source
	addJob := func(eligible func(model.Source) bool) {
		if len(jobs) >= slots {
			return
		}
		for _, choice := range choices {
			selected, ok := e.pickPrefetchSource(choice.candidates, sourceCounts, caps, reservations[choice.tag], now, eligible)
			if !ok {
				continue
			}
			e.reservePrefetchSource(selected, now)
			jobs = append(jobs, selected)
			return
		}
	}
	if !e.ordinaryPrefetchInFlight && !now.Before(e.nextPrefetch) {
		addJob(func(src model.Source) bool { return !src.PrefetchUnlimited })
	}
	for _, source := range sources {
		if source.Enabled && source.Status != "error" && source.PrefetchUnlimited && !e.prefetchInFlight[source.ID] && !now.Before(e.unlimitedPrefetchPace[source.ID]) {
			addJob(func(src model.Source) bool { return src.ID == source.ID })
		}
	}
	if len(jobs) == 0 {
		return nil
	}
	return &prefetchBatch{
		mode: e.modeContext(), jobs: jobs, timeout: time.Duration(settings.Timeout) * time.Millisecond,
	}
}

func (e *Engine) prefetchChoices(sources []model.Source, tags []model.Tag, settings *model.Settings) ([]prefetchChoice, map[string]int) {
	stock := e.distPool.CategorySnapshot(tags)
	type tagStock struct {
		id         string
		count      int
		candidates []model.Source
	}
	var shelves []tagStock
	for _, tag := range tags {
		candidates := filterSources(sources, []string{tag.ID}, tags)
		if len(candidates) > 0 {
			shelves = append(shelves, tagStock{tag.ID, stock[tag.ID], candidates})
		}
	}
	var choices []prefetchChoice
	reservations := make(map[string]int)
	missing := 0
	if len(shelves) > 0 {
		reserve := min(prefetchTagReserve, settings.PoolSize/len(shelves))
		for _, shelf := range shelves {
			reservations[shelf.id] = reserve
		}
		sort.Slice(shelves, func(i, j int) bool {
			if shelves[i].count != shelves[j].count {
				return shelves[i].count < shelves[j].count
			}
			return shelves[i].id < shelves[j].id
		})
		for _, shelf := range shelves {
			if shelf.count >= reserve {
				break
			}
			missing += reserve - shelf.count
			choices = append(choices, prefetchChoice{shelf.candidates, shelf.id})
		}
	}
	if e.distPool.Size()+missing >= settings.PoolSize {
		return choices, reservations
	}
	plan := ensureBoundTagCoverage(e.demandTracker.GetAllocationPlan(settings.PoolSize, stock), settings.BoundTags, stock)
	for _, shelf := range shelves {
		if plan[shelf.id] > 0 {
			choices = append(choices, prefetchChoice{candidates: shelf.candidates})
		}
	}
	choices = append(choices, prefetchChoice{candidates: excludeExclusiveSources(filterSources(sources, nil, tags), tags)})
	return choices, reservations
}

func (e *Engine) runPrefetch(batch *prefetchBatch, source model.Source) {
	defer func() {
		e.replenishMu.Lock()
		defer e.replenishMu.Unlock()
		delete(e.prefetchInFlight, source.ID)
		if !source.PrefetchUnlimited {
			e.ordinaryPrefetchInFlight = false
		}
	}()

	e.modeMu.RLock()
	defer e.modeMu.RUnlock()
	if batch.mode.Err() != nil || e.debugSourceID != 0 || e.debugSwitches.Load() > 0 {
		return
	}
	res, sample := e.fetchPrefetch(batch.mode, source, batch.timeout)
	if sample == nil {
		return
	}

	// Serialize admission when multiple fetches finish together.
	e.replenishMu.Lock()
	defer e.replenishMu.Unlock()
	settings, settingsErr := e.store.GetSettings()
	current, sourceErr := e.store.GetSource(source.ID)
	if batch.mode.Err() != nil || settingsErr != nil || !settings.ProxyMode || settings.PoolSize <= 0 ||
		sourceErr != nil || !prefetchSourceMatches(*current, source) {
		if res != nil {
			e.clearPrefetchFlag(res.ImageID)
		}
		return
	}
	if res == nil {
		e.recordCacheHealth(*sample)
		return
	}
	if e.distPool.Size() >= settings.PoolSize {
		e.clearPrefetchFlag(res.ImageID)
		return
	}
	e.distPool.Push(poolEntryFromResult(res))
	e.recordCacheHealth(*sample)
}

func prefetchSourceMatches(current, selected model.Source) bool {
	for _, choice := range filterSources([]model.Source{current}, nil, nil) {
		if choice.URL == selected.URL && choice.Name == selected.Name && choice.RespType == selected.RespType &&
			choice.JsonPath == selected.JsonPath && maps.Equal(choice.Headers, selected.Headers) &&
			slices.Equal(choice.Categories, selected.Categories) {
			return true
		}
	}
	return false
}

func (e *Engine) clearPrefetchFlag(id int64) {
	if err := e.store.SetImagePooled(id, false); err != nil {
		logger.Error("clear prefetch flag for image %d: %v", id, err)
	}
}

// sourceCaps 按源近期被选中热度分配池额度, 并叠加单源封顶 (防源风控)
func (e *Engine) sourceCaps(poolSize int) map[int64]int {
	snap := e.sourceDemand.Snapshot()
	caps := make(map[int64]int)
	if len(snap) == 0 {
		return caps
	}
	total := 0
	for _, n := range snap {
		total += n
	}
	if total == 0 {
		for id := range snap {
			caps[id] = minPoolPerSource
		}
		return caps
	}
	for id, n := range snap {
		share := int(math.Ceil(float64(n) / float64(total) * float64(poolSize)))
		if share < minPoolPerSource {
			share = minPoolPerSource
		}
		if share > maxPoolPerSource {
			share = maxPoolPerSource
		}
		caps[id] = share
	}
	return caps
}

// ensureBoundTagCoverage 为已绑定但当前无池库存的 tag 分配最小额度,
// 使勾选绑定的标签(如 exclusive)能实际进入池并影响输出。
// plan 可能为 nil (无需求窗口), 必须初始化, 否则赋值 panic。
func ensureBoundTagCoverage(plan map[string]int, boundTags []string, stock map[string]int) map[string]int {
	if plan == nil {
		plan = make(map[string]int)
	}
	for _, bt := range boundTags {
		if bt == "" {
			continue
		}
		if _, ok := plan[bt]; ok {
			continue
		}
		if stock[bt] > 0 {
			continue
		}
		plan[bt] = 1
	}
	return plan
}

func prefetchKeys(src model.Source) []string {
	keys := []string{"source:" + strconv.FormatInt(src.ID, 10)}
	if parsed, err := url.Parse(src.URL); err == nil && parsed.Hostname() != "" {
		keys = append(keys, "host:"+strings.ToLower(strings.TrimSuffix(parsed.Hostname(), ".")))
	}
	return keys
}

// prefetchWait respects both Retry-After formats. HTTP 403 pauses background
// requests to a source that has refused them.
func prefetchWait(now time.Time, status int, retryAfter string) time.Time {
	if status == http.StatusForbidden {
		return now.Add(10 * time.Minute)
	}
	until := now.Add(time.Minute)
	if seconds, err := strconv.ParseInt(strings.TrimSpace(retryAfter), 10, 64); err == nil && seconds >= 0 {
		return maxTime(until, now.Add(time.Duration(min(seconds, int64((1<<63-1)/time.Second)))*time.Second))
	}
	if date, err := http.ParseTime(retryAfter); err == nil {
		return maxTime(until, date)
	}
	return until
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func (e *Engine) pausePrefetch(src model.Source, until time.Time) {
	e.replenishMu.Lock()
	defer e.replenishMu.Unlock()
	if e.prefetchCooldown == nil {
		e.prefetchCooldown = make(map[string]time.Time)
	}
	for _, key := range prefetchKeys(src) {
		e.prefetchCooldown[key] = maxTime(e.prefetchCooldown[key], until)
	}
}

// pickPrefetchSource and reservePrefetchSource run under replenishMu.
func (e *Engine) pickPrefetchSource(candidates []model.Source, sourceCounts, caps map[int64]int, reserve int, now time.Time, eligible func(model.Source) bool) (model.Source, bool) {
	var filtered []model.Source
	for _, src := range candidates {
		if !eligible(src) || e.prefetchInFlight[src.ID] {
			continue
		}
		if reserve == 0 && sourceCounts[src.ID] >= max(caps[src.ID], minPoolPerSource) {
			continue
		}
		if src.PrefetchUnlimited && now.Before(e.unlimitedPrefetchPace[src.ID]) {
			continue
		}
		cooling := false
		for _, key := range prefetchKeys(src) {
			cooling = cooling || now.Before(e.prefetchCooldown[key]) || (!src.PrefetchUnlimited && now.Before(e.prefetchPace[key]))
		}
		if !cooling {
			filtered = append(filtered, src)
		}
	}
	if len(filtered) == 0 {
		return model.Source{}, false
	}
	return weightedPick(filtered), true
}

func (e *Engine) reservePrefetchSource(selected model.Source, now time.Time) {
	if e.prefetchInFlight == nil {
		e.prefetchInFlight = make(map[int64]bool)
	}
	e.prefetchInFlight[selected.ID] = true
	if selected.PrefetchUnlimited {
		if e.unlimitedPrefetchPace == nil {
			e.unlimitedPrefetchPace = make(map[int64]time.Time)
		}
		e.unlimitedPrefetchPace[selected.ID] = now.Add(unlimitedPrefetchInterval)
		return
	}

	e.ordinaryPrefetchInFlight = true
	e.nextPrefetch = now.Add(prefetchInterval)
	if e.prefetchPace == nil {
		e.prefetchPace = make(map[string]time.Time)
	}
	for _, key := range prefetchKeys(selected) {
		e.prefetchPace[key] = now.Add(prefetchSourceInterval)
	}
}

func (e *Engine) fetchPrefetch(parent context.Context, selected model.Source, timeout time.Duration) (*Result, *HealthResult) {
	started := time.Now()
	failed := func(target string, statusCode int, reason string) (*Result, *HealthResult) {
		if parent.Err() != nil {
			return nil, nil
		}
		return nil, &HealthResult{ID: selected.ID, Name: selected.Name, URL: target, StatusCode: statusCode, LatencyMs: time.Since(started).Milliseconds(), Available: false, Error: "缓存预取失败：" + reason, Reason: "分发池预取", Origin: "cache", CheckedAt: time.Now()}
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	req, err := e.newFetchRequest(ctx, selected, "")
	if err != nil {
		return failed(selected.URL, 0, "接口地址无效："+err.Error())
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return failed(selected.URL, 0, "接口请求失败："+err.Error())
	}
	if resp.StatusCode >= 400 {
		status := resp.StatusCode
		if status == http.StatusTooManyRequests || status == http.StatusForbidden {
			e.pausePrefetch(selected, prefetchWait(time.Now(), status, resp.Header.Get("Retry-After")))
		}
		resp.Body.Close()
		return failed(selected.URL, status, fmt.Sprintf("接口返回 HTTP %d", status))
	}

	imageURL, err := extractImageURL(resp, selected)
	resp.Body.Close()
	if err != nil {
		return failed(selected.URL, resp.StatusCode, "解析图片地址失败："+err.Error())
	}

	if e.imageStore != nil {
		srcCats := selected.Categories
		if srcCats == nil {
			srcCats = []string{}
		}
		origURL := imageURL
		cached, cacheErr := e.imageStore.downloadAndStore(parent, origURL, selected.URL, selected.ID, selected.Name, srcCats, selected.Headers, true)
		if cacheErr != nil {
			if errors.Is(cacheErr, errImageBelowMinimum) || parent.Err() != nil {
				return nil, nil
			}
			if errors.Is(cacheErr, errImageCacheStorage) {
				logger.Error("cache prefetch could not store image from source %d: %v", selected.ID, cacheErr)
				return nil, nil
			}
			status := resp.StatusCode
			var upstreamErr *imageHTTPError
			if errors.As(cacheErr, &upstreamErr) {
				status = upstreamErr.status
				if status == http.StatusTooManyRequests || status == http.StatusForbidden {
					e.pausePrefetch(selected, prefetchWait(time.Now(), status, upstreamErr.retryAfter))
				}
			}
			return failed(origURL, status, "下载或校验图片失败："+cacheErr.Error())
		}
		return &Result{
			URL:         origURL,
			LocalURL:    "/images/" + cached.FileID,
			SourceName:  selected.Name,
			SourceID:    selected.ID,
			Categories:  selected.Categories,
			Orientation: cached.Orientation,
			FileID:      cached.FileID,
			Width:       cached.Width,
			Height:      cached.Height,
			Format:      cached.Format,
			ImageID:     cached.ID,
		}, &HealthResult{ID: selected.ID, Name: selected.Name, URL: origURL, StatusCode: resp.StatusCode, LatencyMs: time.Since(started).Milliseconds(), Available: true, CheckedAt: time.Now(), Origin: "cache", Reason: "分发池预取成功"}
	}
	return failed(imageURL, resp.StatusCode, "图片缓存服务未启用")
}

func poolEntryFromResult(res *Result) *PoolEntry {
	if res == nil {
		return nil
	}
	return &PoolEntry{
		FileID:      res.FileID,
		SourceName:  res.SourceName,
		SourceID:    res.SourceID,
		Categories:  res.Categories,
		Orientation: res.Orientation,
		Width:       res.Width,
		Height:      res.Height,
		Format:      res.Format,
		ImageID:     res.ImageID,
	}
}
