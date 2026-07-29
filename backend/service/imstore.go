package service

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
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
	store      *store.Store
	cacheDir   string
	httpClient *http.Client
	mu         sync.Mutex
}

func NewImageStore(st *store.Store, cacheDir string) *ImageStore {
	os.MkdirAll(cacheDir, 0755)
	return &ImageStore{
		store:    st,
		cacheDir: cacheDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
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
	ID         int64
	FileID     string
	Width      int
	Height     int
	Format     string
	SourceID   int64
	SourceName string
}

func generateFileID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func (is *ImageStore) DownloadAndStore(imageURL string, sourceID int64, sourceName string, categories []string) (*CachedImageInfo, error) {
	resp, err := is.httpClient.Get(imageURL)
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
		cfg = image.Config{}
		format = "unknown"
	}

	fileID := generateFileID()
	ext := getExtension(format, resp.Header.Get("Content-Type"))
	filename := fileID + ext
	filePath := filepath.Join(is.cacheDir, filename)

	is.mu.Lock()
	os.WriteFile(filePath, data, 0644)
	if settings != nil && settings.CacheMaxImages > 0 {
		is.evictByCount(settings.CacheMaxImages)
	}
	totalSize := getDirSize(is.cacheDir)
	if settings != nil && totalSize > int64(settings.CacheMaxMB)*1024*1024 {
		maxIter := 100
		for i := 0; i < maxIter; i++ {
			if getDirSize(is.cacheDir) <= int64(settings.CacheMaxMB)*1024*1024 {
				break
			}
			evictLRU(is.cacheDir, is.store)
		}
	}
	is.mu.Unlock()

	catsJSON := encodeStringSlice(categories)
	imgID, err := is.store.InsertImage(fileID, imageURL, sourceID, sourceName, cfg.Width, cfg.Height, format, int64(len(data)), catsJSON)
	if err != nil {
		return nil, fmt.Errorf("store metadata: %w", err)
	}

	return &CachedImageInfo{
		ID:         imgID,
		FileID:     fileID,
		Width:      cfg.Width,
		Height:     cfg.Height,
		Format:     format,
		SourceID:   sourceID,
		SourceName: sourceName,
	}, nil
}

func (is *ImageStore) GetImage(fileID string) (string, string, error) {
	img, err := is.store.GetImageByFileID(fileID)
	if err != nil {
		return "", "", err
	}

	pattern := filepath.Join(is.cacheDir, img.FileID+".*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", "", fmt.Errorf("file not found on disk")
	}

	contentType := detectContentType(img.Format, matches[0])
	return matches[0], contentType, nil
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

	srcPattern := filepath.Join(is.cacheDir, img.FileID+".*")
	matches, _ := filepath.Glob(srcPattern)
	if len(matches) == 0 {
		return fmt.Errorf("source file not found")
	}

	dstPath := filepath.Join(savedDir, filepath.Base(matches[0]))
	if err := copyFile(matches[0], dstPath); err != nil {
		return err
	}

	return is.store.UpdateImageSaved(imageID, true)
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

func getDirSize(dir string) int64 {
	var total int64
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func fileIDFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func evictLRU(dir string, st *store.Store) {
	entries, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(entries) == 0 {
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
		infos = append(infos, entryInfo{e, info.ModTime()})
	}
	if len(infos) == 0 {
		return
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].mod.Before(infos[j].mod)
	})

	oldest := infos[0].path
	if st != nil {
		st.DeleteImageByFileID(fileIDFromPath(oldest))
	}
	os.Remove(oldest)
}

func (is *ImageStore) evictByCount(maxImages int) {
	entries, _ := filepath.Glob(filepath.Join(is.cacheDir, "*"))
	if len(entries) <= maxImages {
		return
	}
	over := len(entries) - maxImages

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

func encodeStringSlice(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(s)
	return string(b)
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
