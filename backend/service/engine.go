package service

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/monitor"
	"github.com/pichub/backend/store"
)

const (
	// maxFetchPerTick 单轮池补充的最大拉取数, 防止突发打爆图源触发风控
	maxFetchPerTick = 8
	// maxPoolPerSource 单源池条目封顶
	maxPoolPerSource = 8
	// minPoolPerSource 单源池条目下限
	minPoolPerSource = 1
)

type Engine struct {
	store         *store.Store
	proxyCache    *ProxyCache
	imageStore    *ImageStore
	proxyConfig   *ProxyConfig
	distPool      *DistributionPool
	demandTracker *DemandTracker
	sourceDemand  *SourceDemandTracker
	httpClient    *http.Client
	monitor       *monitor.SourceMonitor
}

func NewEngine(st *store.Store, pc *ProxyCache, imgStore *ImageStore, proxyCfg *ProxyConfig, mon *monitor.SourceMonitor) *Engine {
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
	go eng.StartDistributionWorker()
	return eng
}

type Result struct {
	URL         string   `json:"url"`
	LocalURL    string   `json:"local_url,omitempty"`
	SourceName  string   `json:"source"`
	SourceID    int64    `json:"source_id,omitempty"`
	Categories  []string `json:"categories"`
	Orientation string   `json:"orientation,omitempty"`
	FileID      string   `json:"file_id,omitempty"`
	Width       int      `json:"width,omitempty"`
	Height      int      `json:"height,omitempty"`
	Format      string   `json:"format,omitempty"`
	ImageID     int64    `json:"image_id,omitempty"`
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
	settings, err := e.store.GetSettings()
	if err != nil {
		return nil, 0, fmt.Errorf("get settings: %w", err)
	}

	queryCats := splitCategory(category)

	tags, _ := e.store.GetTags()
	exclSet := exclusiveTagSet(tags)

	if orientation == "" {
		uaOri := detectOrientationFromUA(clientUA)
		if uaOri != "" && len(queryCats) == 0 {
			orientation = uaOri
		}
	}
	if len(queryCats) == 1 && queryCats[0] == "adaptive" && orientation == "" {
		if uaOri := detectOrientationFromUA(clientUA); uaOri != "" {
			orientation = uaOri
		}
	}

	if settings.ProxyMode && settings.PoolSize > 0 && e.distPool != nil {
		var poolResult *Result
		single := len(queryCats) == 1
		hasExcl := hasExclusiveTag(tags, queryCats)
		switch {
		case orientation != "" && len(queryCats) > 0:
			poolResult = e.distPool.PopMatching(func(entry *PoolEntry) bool {
				return matchCategories(entry.Categories, queryCats, tags, single, hasExcl) &&
					entry.Orientation == orientation
			})
		case orientation != "":
			poolResult = e.distPool.PopByOrientation(orientation, exclSet)
		case len(queryCats) > 0:
			poolResult = e.distPool.PopMatching(func(entry *PoolEntry) bool {
				return matchCategories(entry.Categories, queryCats, tags, single, hasExcl)
			})
		default:
			poolResult = e.distPool.PopAny(exclSet)
		}

		if poolResult != nil && e.imageStore != nil {
			if _, _, err := e.imageStore.GetImage(poolResult.FileID); err != nil {
				logger.System("pool entry %s no longer exists on disk, skip", poolResult.FileID)
				poolResult = nil
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
			go func() {
				e.store.SetImagePooled(imgID, false)
				e.store.RecordStats(queryCats, src, poolResult.LocalURL, &imgID, poolResult.FileID)
			}()
			logger.System("pool hit for category %q, instant response", category)

			if format == "json" {
				return poolResult, http.StatusOK, nil
			}
			return poolResult, http.StatusFound, nil
		}
	}

	sources, err := e.store.ListSources()
	if err != nil {
		return nil, 0, fmt.Errorf("list sources: %w", err)
	}

	candidates := filterSources(sources, queryCats, tags)
	if len(candidates) == 0 {
		return nil, 0, fmt.Errorf("no available sources")
	}

	timeout := time.Duration(settings.Timeout) * time.Millisecond

	maxAttempts := 3
	if e.imageStore != nil && settings.ProxyMode && orientation != "" {
		maxAttempts = 8
	}

	for attempt := 0; attempt < maxAttempts && len(candidates) > 0; attempt++ {
		selected := weightedPick(candidates)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		req, err := e.newFetchRequest(ctx, selected, clientUA)
		if err != nil {
			cancel()
			candidates = removeSource(candidates, selected.ID)
			continue
		}

		resp, err := e.httpClient.Do(req)
		if err != nil {
			cancel()
			logger.Error("source %q fetch failed: %v", selected.Name, err)
			if e.monitor != nil {
				e.monitor.Emit(selected.ID, false)
			}
			candidates = removeSource(candidates, selected.ID)
			continue
		}

		hasFailed := false
		var cachedInfo *CachedImageInfo

		if resp.StatusCode >= 500 {
			logger.Error("source %q returned %d", selected.Name, resp.StatusCode)
			if e.monitor != nil {
				e.monitor.Emit(selected.ID, false)
			}
			candidates = removeSource(candidates, selected.ID)
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
			candidates = removeSource(candidates, selected.ID)
			continue
		}

		e.store.UpdateSourceStatus(selected.ID, "normal")

		origURL := imageURL

		if e.imageStore != nil && settings.ProxyMode {
			cached, cacheErr := e.imageStore.DownloadAndStore(origURL, selected.URL, selected.ID, selected.Name, queryCats, selected.Headers, false)
			if cacheErr != nil {
				logger.Error("cache download failed for %s: %v", selected.Name, cacheErr)
				if e.monitor != nil {
					e.monitor.Emit(selected.ID, false)
				}
				candidates = removeSource(candidates, selected.ID)
				continue
			}
			cachedInfo = cached

			if orientation != "" {
				imgOrient := GetOrientation(cached.Width, cached.Height)
				if imgOrient != orientation {
					logger.System("orientation mismatch for %s: got %s, wanted %s", selected.Name, imgOrient, orientation)
					candidates = removeSource(candidates, selected.ID)
					continue
				}
			}

			imageURL = "/images/" + cached.FileID
		} else if e.proxyCache != nil && settings.ProxyMode {
			_, _, proxyErr := e.proxyCache.GetOrFetch(imageURL)
			if proxyErr != nil {
				hasFailed = true
			}
		}

		if hasFailed {
			candidates = removeSource(candidates, selected.ID)
			continue
		}

		var imgID *int64
		var fileID string
		if cachedInfo != nil {
			imgID = &cachedInfo.ID
			fileID = cachedInfo.FileID
		}
		e.demandTracker.RecordRequest(queryCats, cachedInfo != nil)
		e.sourceDemand.RecordSelection(selected.ID)
		go e.store.RecordStats(queryCats, selected, imageURL, imgID, fileID)

		res := &Result{
			URL:        origURL,
			SourceName: selected.Name,
			Categories: selected.Categories,
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

	return nil, 0, fmt.Errorf("all sources failed")
}

func (e *Engine) newFetchRequest(ctx context.Context, selected model.Source, clientUA string) (*http.Request, error) {
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
		if !hasUA {
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
	totalWeight := 0
	for _, src := range sources {
		totalWeight += src.Weight
	}
	if totalWeight == 0 {
		return sources[rand.Intn(len(sources))]
	}
	r := rand.Intn(totalWeight)
	for _, src := range sources {
		r -= src.Weight
		if r < 0 {
			return src
		}
	}
	return sources[len(sources)-1]
}

func removeSource(sources []model.Source, id int64) []model.Source {
	for i, src := range sources {
		if src.ID == id {
			return append(sources[:i], sources[i+1:]...)
		}
	}
	return sources
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

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		e.replenishPool()
	}
}

func (e *Engine) ReplenishPool() {
	e.replenishPool()
}

func (e *Engine) RemoveSourceFromPool(sourceID int64) {
	if e.distPool != nil {
		e.distPool.RemoveBySourceID(sourceID)
	}
}

func (e *Engine) replenishPool() {
	settings, err := e.store.GetSettings()
	if err != nil || !settings.ProxyMode || settings.PoolSize <= 0 || e.distPool == nil {
		return
	}

	// 淘汰超轮次项
	maxRounds := settings.PoolSize - 1
	e.distPool.Trim(settings.PoolSize, maxRounds)

	currentSize := e.distPool.Size()
	if currentSize >= settings.PoolSize {
		return
	}

	// 各源磁盘缓存文件数 (池占用) + 按热度/风控封顶计算出的单源额度
	sourceCounts := make(map[int64]int)
	if e.imageStore != nil {
		sources, _ := e.store.ListSources()
		for _, src := range sources {
			sourceCounts[src.ID] = e.imageStore.CountSourceCachedFiles(src.ID)
		}
	}
	caps := e.sourceCaps(settings.PoolSize)

	// 每 tick 拉取数上限, 防止突发打爆图源触发风控
	fetched := 0
	push := func(res *Result) bool {
		if res == nil {
			return false
		}
		e.distPool.Push(poolEntryFromResult(res))
		sourceCounts[res.SourceID]++
		fetched++
		return true
	}

	stock := e.distPool.CategorySnapshot()
	plan := e.demandTracker.GetAllocationPlan(settings.PoolSize, stock)

	if len(plan) == 0 {
		// 窗口无数据：为每个有源的 tag 各拉一张
		tags, _ := e.store.GetTags()
		for _, t := range tags {
			if e.distPool.Size() >= settings.PoolSize || fetched >= maxFetchPerTick {
				break
			}
			if !push(e.fetchSingleForTag(t.ID, sourceCounts, caps)) {
				continue
			}
		}
		// 剩余 slot 填无 tag 源
		for e.distPool.Size() < settings.PoolSize && fetched < maxFetchPerTick {
			if !push(e.fetchSingleForTag("", sourceCounts, caps)) {
				break
			}
		}
		return
	}

	for tag, count := range plan {
		if count <= 0 {
			continue
		}
		for i := 0; i < count && e.distPool.Size() < settings.PoolSize && fetched < maxFetchPerTick; i++ {
			if !push(e.fetchSingleForTag(tag, sourceCounts, caps)) {
				break
			}
		}
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

func (e *Engine) fetchSingleForTag(tag string, sourceCounts map[int64]int, caps map[int64]int) *Result {
	sources, err := e.store.ListSources()
	if err != nil || len(sources) == 0 {
		return nil
	}

	tags, _ := e.store.GetTags()
	var candidates []model.Source
	if tag != "" {
		candidates = filterSources(sources, []string{tag}, tags)
	} else {
		candidates = excludeExclusiveSources(filterSources(sources, nil, tags), tags)
	}
	if len(candidates) == 0 {
		return nil
	}

	// 过滤掉已达该源自适应额度上限的源 (默认上限由热度分配, 封顶 maxPoolPerSource)
	var filtered []model.Source
	for _, src := range candidates {
		limit := caps[src.ID]
		if limit < minPoolPerSource {
			limit = minPoolPerSource
		}
		if sourceCounts[src.ID] < limit {
			filtered = append(filtered, src)
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	selected := weightedPick(filtered)

	settings, err := e.store.GetSettings()
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(settings.Timeout)*time.Millisecond)
	defer cancel()
	req, err := e.newFetchRequest(ctx, selected, "")
	if err != nil {
		return nil
	}
	resp, err := e.httpClient.Do(req)
	if err != nil || resp.StatusCode >= 400 {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}

	imageURL, err := extractImageURL(resp, selected)
	resp.Body.Close()
	if err != nil {
		return nil
	}

	if e.imageStore != nil {
		srcCats := selected.Categories
		if srcCats == nil {
			srcCats = []string{}
		}
		origURL := imageURL
		cached, cacheErr := e.imageStore.DownloadAndStore(origURL, selected.URL, selected.ID, selected.Name, srcCats, selected.Headers, true)
		if cacheErr != nil {
			return nil
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
		}
	}
	return nil
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
		RoundCount:  0,
	}
}
