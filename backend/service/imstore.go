package service

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pichub/backend/store"
)

type ImageStore struct {
	store       *store.Store
	cacheDir    string
	proxyConfig *ProxyConfig
	httpClient  *http.Client
	mu          sync.Mutex
}

func NewImageStore(st *store.Store, cacheDir string, proxyCfg *ProxyConfig) *ImageStore {
	os.MkdirAll(cacheDir, 0755)
	transport := &http.Transport{}
	if proxyCfg != nil {
		transport.Proxy = proxyCfg.Proxy
	}
	return &ImageStore{
		store:       st,
		cacheDir:    cacheDir,
		proxyConfig: proxyCfg,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

type CachedImageInfo struct {
	ID          int64
	FileID      string
	Width       int
	Height      int
	Format      string
	SourceID    int64
	SourceName  string
	Orientation string
}

func generateFileID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func (is *ImageStore) DownloadAndStore(imageURL, sourceURL string, sourceID int64, sourceName string, categories []string, headers map[string]string, pooled bool) (*CachedImageInfo, error) {
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, vs := range buildImageRequestHeaders(sourceURL, headers) {
		if len(vs) > 0 {
			req.Header.Set(k, vs[0])
		}
	}
	resp, err := is.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch image: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty response body")
	}

	settings, _ := is.store.GetSettings()
	if settings != nil && settings.MinResolution != "" && settings.MinResolution != "0" {
		ok, _ := checkResolution(data, settings.MinResolution)
		if !ok {
			return nil, fmt.Errorf("resolution below minimum: %s", settings.MinResolution)
		}
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid image: %w", err)
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		return nil, fmt.Errorf("invalid dimensions: %dx%d", cfg.Width, cfg.Height)
	}
	if format == "" || format == "unknown" {
		return nil, fmt.Errorf("unknown image format")
	}

	fileID := generateFileID()
	ext := getExtension(format, resp.Header.Get("Content-Type"))
	filename := fileID + ext
	subDir := filepath.Join(is.cacheDir, fmt.Sprintf("%d", sourceID))
	os.MkdirAll(subDir, 0755)
	filePath := filepath.Join(subDir, filename)

	is.mu.Lock()
	os.WriteFile(filePath, data, 0644)
	protectedLimit := 60
	if settings != nil && settings.MaxHistoryRecords > 0 {
		protectedLimit = settings.MaxHistoryRecords
	}
	protected, _ := is.store.GetProtectedFileIDs(protectedLimit)
	if settings != nil && settings.CacheMaxImages > 0 {
		is.evictByCount(settings.CacheMaxImages, protected)
	}
	totalSize := dirSize(is.cacheDir)
	if settings != nil && totalSize > int64(settings.CacheMaxMB)*1024*1024 {
		maxIter := 100
		for i := 0; i < maxIter; i++ {
			if dirSize(is.cacheDir) <= int64(settings.CacheMaxMB)*1024*1024 {
				break
			}
			if !evictLRU(is.cacheDir, is.store, protected) {
				break
			}
		}
	}
	is.mu.Unlock()

	orientation := GetOrientation(cfg.Width, cfg.Height)
	catsJSON := encodeStringSlice(categories)
	imgID, err := is.store.InsertImage(fileID, imageURL, sourceID, sourceName, cfg.Width, cfg.Height, format, int64(len(data)), catsJSON, orientation, pooled)
	if err != nil {
		return nil, fmt.Errorf("store metadata: %w", err)
	}

	return &CachedImageInfo{
		ID:          imgID,
		FileID:      fileID,
		Width:       cfg.Width,
		Height:      cfg.Height,
		Format:      format,
		SourceID:    sourceID,
		SourceName:  sourceName,
		Orientation: orientation,
	}, nil
}

func (is *ImageStore) GetImage(fileID string) (string, string, error) {
	img, err := is.store.GetImageByFileID(fileID)
	if err != nil {
		return "", "", err
	}

	// 优先新结构：cacheDir/{sourceID}/{fileID}.*
	subDir := filepath.Join(is.cacheDir, fmt.Sprintf("%d", img.SourceID))
	pattern := filepath.Join(subDir, img.FileID+".*")
	matches, _ := filepath.Glob(pattern)
	if len(matches) > 0 {
		contentType := detectContentType(img.Format, matches[0])
		return matches[0], contentType, nil
	}

	// 回退旧结构：cacheDir/{fileID}.*
	pattern = filepath.Join(is.cacheDir, img.FileID+".*")
	matches, _ = filepath.Glob(pattern)
	if len(matches) > 0 {
		contentType := detectContentType(img.Format, matches[0])
		return matches[0], contentType, nil
	}

	settings, _ := is.store.GetSettings()
	var savedDir string
	if settings != nil {
		savedDir = settings.SavedImagesDir
	}
	if savedDir == "" {
		savedDir = filepath.Join(is.cacheDir, "../saved")
	}
	savedPattern := filepath.Join(savedDir, img.FileID+".*")
	savedMatches, err := filepath.Glob(savedPattern)
	if err == nil && len(savedMatches) > 0 {
		contentType := detectContentType(img.Format, savedMatches[0])
		return savedMatches[0], contentType, nil
	}

	return "", "", fmt.Errorf("file not found on disk")
}

func (is *ImageStore) SaveImage(imageID int64) error {
	img, err := is.store.GetImageByID(imageID)
	if err != nil {
		return err
	}

	settings, err := is.store.GetSettings()
	if err != nil {
		return err
	}

	var savedDir string
	if settings != nil {
		savedDir = settings.SavedImagesDir
	}

	if savedDir == "" {
		savedDir = filepath.Join(is.cacheDir, "../saved")
	}
	os.MkdirAll(savedDir, 0755)

	srcPattern := filepath.Join(is.cacheDir, fmt.Sprintf("%d", img.SourceID), img.FileID+".*")
	matches, _ := filepath.Glob(srcPattern)
	if len(matches) == 0 {
		srcPattern = filepath.Join(is.cacheDir, img.FileID+".*")
		matches, _ = filepath.Glob(srcPattern)
	}
	if len(matches) == 0 {
		return fmt.Errorf("source file not found")
	}

	dstPath := filepath.Join(savedDir, filepath.Base(matches[0]))
	if err := copyFile(matches[0], dstPath); err != nil {
		return err
	}

	if err := is.store.UpdateImageSaved(imageID, true); err != nil {
		return err
	}
	return nil
}

func (is *ImageStore) UnsaveImage(imageID int64) error {
	img, err := is.store.GetImageByID(imageID)
	if err != nil {
		return err
	}

	settings, _ := is.store.GetSettings()
	var savedDir string
	if settings != nil {
		savedDir = settings.SavedImagesDir
	}
	if savedDir == "" {
		savedDir = filepath.Join(is.cacheDir, "../saved")
	}

	pattern := filepath.Join(savedDir, img.FileID+".*")
	matches, _ := filepath.Glob(pattern)
	for _, m := range matches {
		os.Remove(m)
	}

	// 同时清理旧结构中的缓存文件（如果存在）
	oldPattern := filepath.Join(is.cacheDir, img.FileID+".*")
	oldMatches, _ := filepath.Glob(oldPattern)
	for _, m := range oldMatches {
		os.Remove(m)
	}

	return is.store.UpdateImageSaved(imageID, false)
}

func getExtension(format, contentType string) string {
	switch format {
	case "jpeg":
		return ".jpg"
	case "png":
		return ".png"
	case "gif":
		return ".gif"
	case "webp":
		return ".webp"
	}
	if strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg") {
		return ".jpg"
	}
	if strings.Contains(contentType, "png") {
		return ".png"
	}
	if strings.Contains(contentType, "gif") {
		return ".gif"
	}
	return ".jpg"
}

func detectContentType(format, path string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return "image/jpeg"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func fileIDFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func evictLRU(dir string, st *store.Store, protected map[string]bool) bool {
	var pooledSet map[string]bool
	if st != nil {
		pooledSet, _ = st.GetPooledFileIDSet()
	}
	var candidates []string
	for _, e := range listCacheFiles(dir) {
		if protected != nil && protected[fileIDFromPath(e)] {
			continue
		}
		candidates = append(candidates, e)
	}
	if len(candidates) == 0 {
		return false
	}
	// 优先淘汰池中未分发 (pooled=1) 的最旧文件, 再淘汰未保护的已分发文件
	oldest := oldestFile(candidates)
	if pooledSet != nil {
		var pooled []string
		for _, e := range candidates {
			if pooledSet[fileIDFromPath(e)] {
				pooled = append(pooled, e)
			}
		}
		if len(pooled) > 0 {
			if p := oldestFile(pooled); p != "" {
				oldest = p
			}
		}
	}
	if oldest == "" {
		return false
	}
	if st != nil {
		st.DeleteImageByFileID(fileIDFromPath(oldest))
	}
	os.Remove(oldest)
	return true
}

func (is *ImageStore) evictByCount(maxImages int, protected map[string]bool) {
	entries := listCacheFiles(is.cacheDir)
	if len(entries) <= maxImages {
		return
	}
	over := len(entries) - maxImages

	pooledSet, _ := is.store.GetPooledFileIDSet()

	type entryInfo struct {
		path string
		mod  time.Time
	}
	var infos []entryInfo
	for _, e := range entries {
		info, err := os.Stat(e)
		if err != nil {
			continue
		}
		if protected != nil && protected[fileIDFromPath(e)] {
			continue
		}
		infos = append(infos, entryInfo{e, info.ModTime()})
	}

	sort.Slice(infos, func(i, j int) bool {
		iPooled := pooledSet != nil && pooledSet[fileIDFromPath(infos[i].path)]
		jPooled := pooledSet != nil && pooledSet[fileIDFromPath(infos[j].path)]
		if iPooled != jPooled {
			return iPooled
		}
		return infos[i].mod.Before(infos[j].mod)
	})

	for i := 0; i < over && i < len(infos); i++ {
		fileID := fileIDFromPath(infos[i].path)
		is.store.DeleteImageByFileID(fileID)
		os.Remove(infos[i].path)
	}
}

// CleanupOrphanPooled 清理 pooled=1 但磁盘文件已缺失的孤儿记录 (启动时调用)
func (is *ImageStore) CleanupOrphanPooled() {
	ids, err := is.store.QueryPooledFileIDs()
	if err != nil {
		return
	}
	for _, fid := range ids {
		found := false
		for _, pattern := range []string{
			filepath.Join(is.cacheDir, "*", fid+".*"),
			filepath.Join(is.cacheDir, fid+".*"),
		} {
			if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
				found = true
				break
			}
		}
		if !found {
			is.store.DeleteImageByFileID(fid)
		}
	}
}

func encodeStringSlice(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(s)
	return string(b)
}

func (is *ImageStore) ClearSourceImages(sourceID int64) error {
	dir := filepath.Join(is.cacheDir, fmt.Sprintf("%d", sourceID))
	fileIDs, err := is.store.GetImageFileIDsBySourceID(sourceID)
	if err != nil {
		return err
	}
	for _, fileID := range fileIDs {
		pattern := filepath.Join(dir, fileID+".*")
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			os.Remove(m)
		}
		// 已被 MoveToRoot 移到根目录的文件一并清理
		rootPattern := filepath.Join(is.cacheDir, fileID+".*")
		rootMatches, _ := filepath.Glob(rootPattern)
		for _, m := range rootMatches {
			os.Remove(m)
		}
	}
	return is.store.DeleteImagesBySourceID(sourceID)
}

func (is *ImageStore) DeleteSourceFolder(sourceID int64) error {
	dir := filepath.Join(is.cacheDir, fmt.Sprintf("%d", sourceID))
	os.RemoveAll(dir)
	fileIDs, err := is.store.GetImageFileIDsBySourceIDAll(sourceID)
	if err != nil {
		return err
	}
	for _, fileID := range fileIDs {
		rootMatches, _ := filepath.Glob(filepath.Join(is.cacheDir, fileID+".*"))
		for _, m := range rootMatches {
			os.Remove(m)
		}
	}
	return is.store.DeleteImagesBySourceIDAll(sourceID)
}

func (is *ImageStore) MoveToRoot(fileID string, sourceID int64) error {
	srcDir := filepath.Join(is.cacheDir, fmt.Sprintf("%d", sourceID))
	pattern := filepath.Join(srcDir, fileID+".*")
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		return nil
	}
	dstPath := filepath.Join(is.cacheDir, filepath.Base(matches[0]))
	return os.Rename(matches[0], dstPath)
}

func (is *ImageStore) CountSourceCachedFiles(sourceID int64) int {
	count, _ := filepath.Glob(filepath.Join(is.cacheDir, fmt.Sprintf("%d", sourceID), "*"))
	total := len(count)
	// 统计已被 MoveToRoot 移到根目录的文件
	if ids, err := is.store.GetImageFileIDsBySourceID(sourceID); err == nil {
		for _, fid := range ids {
			if matches, _ := filepath.Glob(filepath.Join(is.cacheDir, fid+".*")); len(matches) > 0 {
				total += len(matches)
			}
		}
	}
	return total
}

func GetOrientation(width, height int) string {
	if width > height {
		return "horizontal"
	}
	if height > width {
		return "vertical"
	}
	return "square"
}
