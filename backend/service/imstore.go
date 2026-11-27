package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
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

type imageHTTPError struct {
	status     int
	retryAfter string
}

func (err *imageHTTPError) Error() string { return fmt.Sprintf("fetch image: HTTP %d", err.status) }

var (
	errImageBelowMinimum = errors.New("resolution below minimum")
	errImageCacheStorage = errors.New("store image in local cache")
)

func (is *ImageStore) DownloadAndStore(imageURL, sourceURL string, sourceID int64, sourceName string, categories []string, headers map[string]string, pooled bool) (*CachedImageInfo, error) {
	return is.downloadAndStore(context.Background(), imageURL, sourceURL, sourceID, sourceName, categories, headers, pooled)
}

func (is *ImageStore) downloadAndStore(ctx context.Context, imageURL, sourceURL string, sourceID int64, sourceName string, categories []string, headers map[string]string, pooled bool) (*CachedImageInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
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
		return nil, &imageHTTPError{status: resp.StatusCode, retryAfter: resp.Header.Get("Retry-After")}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty response body")
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

	settings, _ := is.store.GetSettings()
	if settings != nil && !matchesMinimumResolution(cfg, settings.MinResolution) {
		return nil, fmt.Errorf("%w: %s", errImageBelowMinimum, settings.MinResolution)
	}

	fileID := generateFileID()
	ext := getExtension(format, resp.Header.Get("Content-Type"))
	filename := fileID + ext
	subDir := filepath.Join(is.cacheDir, fmt.Sprintf("%d", sourceID))
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return nil, fmt.Errorf("%w: create image directory: %w", errImageCacheStorage, err)
	}
	filePath := filepath.Join(subDir, filename)

	is.mu.Lock()
	defer is.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tmpFile, err := os.CreateTemp(subDir, ".pichub-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("%w: create temporary image file: %w", errImageCacheStorage, err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	written, err := tmpFile.Write(data)
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("%w: write image file: %w", errImageCacheStorage, err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("%w: sync image file: %w", errImageCacheStorage, err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("%w: close image file: %w", errImageCacheStorage, err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		return nil, fmt.Errorf("%w: publish image file: %w", errImageCacheStorage, err)
	}

	orientation := GetOrientation(cfg.Width, cfg.Height)
	catsJSON := encodeStringSlice(categories)
	imgID, err := is.store.InsertImage(fileID, imageURL, sourceID, sourceName, cfg.Width, cfg.Height, format, int64(len(data)), catsJSON, orientation, pooled)
	if err != nil {
		if removeErr := os.Remove(filePath); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, fmt.Errorf("%w: store metadata: %w (remove image file: %v)", errImageCacheStorage, err, removeErr)
		}
		return nil, fmt.Errorf("%w: store metadata: %w", errImageCacheStorage, err)
	}

	protectedLimit := 60
	if settings != nil && settings.MaxHistoryRecords > 0 {
		protectedLimit = settings.MaxHistoryRecords
	}
	protected, _ := is.store.GetProtectedFileIDs(protectedLimit)
	if protected == nil {
		protected = make(map[string]bool)
	}
	protected[fileID] = true
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
	if st == nil {
		return false
	}
	pooledSet, err := st.GetPooledFileIDSet()
	if err != nil {
		return false
	}
	var candidates []string
	for _, e := range listCacheFiles(dir) {
		fileID := fileIDFromPath(e)
		if (protected != nil && protected[fileID]) || (pooledSet != nil && pooledSet[fileID]) {
			continue
		}
		candidates = append(candidates, e)
	}
	if len(candidates) == 0 {
		return false
	}
	oldest := oldestFile(candidates)
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
	if is.store == nil {
		return
	}
	entries := listCacheFiles(is.cacheDir)
	if len(entries) <= maxImages {
		return
	}
	over := len(entries) - maxImages

	pooledSet, err := is.store.GetPooledFileIDSet()
	if err != nil {
		return
	}

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
		fileID := fileIDFromPath(e)
		if (protected != nil && protected[fileID]) || (pooledSet != nil && pooledSet[fileID]) {
			continue
		}
		infos = append(infos, entryInfo{e, info.ModTime()})
	}

	sort.Slice(infos, func(i, j int) bool {
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
		if _, _, err := is.GetImage(fid); err != nil {
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
