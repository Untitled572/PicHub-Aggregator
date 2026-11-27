package service

import (
	"bytes"
	"database/sql"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pichub/backend/store"
)

func newTestImageStore(t *testing.T) (*store.Store, *ImageStore, string, *httptest.Server) {
	t.Helper()

	root := t.TempDir()
	dbPath := filepath.Join(root, "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatalf("get test settings: %v", err)
	}
	settings.MinResolution = "0"
	settings.CacheMaxImages = 1000
	settings.CacheMaxMB = 500
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatalf("update test settings: %v", err)
	}

	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageData.Bytes())
	}))
	t.Cleanup(server.Close)

	cacheDir := filepath.Join(root, "cache")
	is := NewImageStore(st, cacheDir, nil)
	return st, is, dbPath, server
}

func TestDownloadAndStoreWritesImageBeforeReturning(t *testing.T) {
	st, is, _, server := newTestImageStore(t)

	got, err := is.DownloadAndStore(server.URL, server.URL, 42, "test", nil, nil, false)
	if err != nil {
		t.Fatalf("download and store image: %v", err)
	}
	if got == nil || got.FileID == "" {
		t.Fatal("download returned no image file ID")
	}

	path, contentType, err := is.GetImage(got.FileID)
	if err != nil {
		t.Fatalf("get stored image: %v", err)
	}
	if contentType != "image/png" {
		t.Errorf("content type = %q, want image/png", contentType)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stored image is missing: %v", err)
	}
	if _, err := st.GetImageByFileID(got.FileID); err != nil {
		t.Fatalf("stored image metadata is missing: %v", err)
	}
}

func TestDownloadAndStoreSameURLKeepsEarlierFile(t *testing.T) {
	_, is, _, server := newTestImageStore(t)

	first, err := is.DownloadAndStore(server.URL, server.URL, 42, "test", nil, nil, false)
	if err != nil {
		t.Fatalf("store first download: %v", err)
	}
	firstPath, _, err := is.GetImage(first.FileID)
	if err != nil {
		t.Fatalf("get first stored image: %v", err)
	}

	second, err := is.DownloadAndStore(server.URL, server.URL, 42, "test", nil, nil, false)
	if err != nil {
		t.Fatalf("store repeated URL download: %v", err)
	}
	if second.FileID == first.FileID {
		t.Fatalf("repeated URL reused file ID %q", first.FileID)
	}
	if _, err := os.Stat(firstPath); err != nil {
		t.Fatalf("repeated URL removed the earlier file: %v", err)
	}
}

func TestDownloadAndStoreReturnsDirectoryCreationError(t *testing.T) {
	_, is, _, server := newTestImageStore(t)
	cacheDir := is.cacheDir
	if err := os.WriteFile(filepath.Join(cacheDir, "42"), []byte("block directory creation"), 0o644); err != nil {
		t.Fatalf("create blocking file: %v", err)
	}

	if _, err := is.DownloadAndStore(server.URL, server.URL, 42, "test", nil, nil, false); err == nil {
		t.Fatal("DownloadAndStore succeeded although the source directory cannot be created")
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache directory: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "42" {
		t.Fatalf("unexpected cache artifacts after directory failure: %v", entries)
	}
}

func TestDownloadAndStoreRemovesFileWhenMetadataInsertFails(t *testing.T) {
	_, is, dbPath, server := newTestImageStore(t)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open second test connection: %v", err)
	}
	if _, err := db.Exec("DROP TABLE images"); err != nil {
		_ = db.Close()
		t.Fatalf("drop images table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close second test connection: %v", err)
	}

	if _, err := is.DownloadAndStore(server.URL, server.URL, 42, "test", nil, nil, false); err == nil {
		t.Fatal("DownloadAndStore succeeded although metadata insertion must fail")
	}
	sourceDir := filepath.Join(is.cacheDir, "42")
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatalf("read source cache directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("metadata failure left image files behind: %v", entries)
	}
}

func TestCacheEvictionKeepsReadyImagesBeforeOrdinaryCache(t *testing.T) {
	for _, byCount := range []bool{true, false} {
		name := "disk limit"
		if byCount {
			name = "count limit"
		}
		t.Run(name, func(t *testing.T) {
			st, images, _, server := newTestImageStore(t)
			ready, err := images.DownloadAndStore(server.URL, server.URL, 1, "ready", nil, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			ordinary, err := images.DownloadAndStore(server.URL, server.URL, 2, "ordinary", nil, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if byCount {
				images.evictByCount(1, nil)
			} else if !evictLRU(images.cacheDir, st, nil) {
				t.Fatal("no file evicted")
			}
			if _, _, err := images.GetImage(ready.FileID); err != nil {
				t.Fatalf("pool image evicted: %v", err)
			}
			if _, _, err := images.GetImage(ordinary.FileID); err == nil {
				t.Fatal("ordinary image survived")
			}
		})
	}
}

func TestCacheEvictionNeverRemovesPooledImages(t *testing.T) {
	for _, byCount := range []bool{true, false} {
		name := "disk limit"
		if byCount {
			name = "count limit"
		}
		t.Run(name, func(t *testing.T) {
			st, images, _, server := newTestImageStore(t)
			first, err := images.DownloadAndStore(server.URL, server.URL, 1, "ready-1", nil, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			second, err := images.DownloadAndStore(server.URL, server.URL, 2, "ready-2", nil, nil, true)
			if err != nil {
				t.Fatal(err)
			}

			if byCount {
				images.evictByCount(0, nil)
			} else {
				for evictLRU(images.cacheDir, st, nil) {
				}
			}

			for _, ready := range []*CachedImageInfo{first, second} {
				if _, _, err := images.GetImage(ready.FileID); err != nil {
					t.Fatalf("pooled image %s was evicted: %v", ready.FileID, err)
				}
			}
		})
	}
}

func TestCleanupOrphanPooledKeepsImagesInSavedDirectory(t *testing.T) {
	st, images, _, server := newTestImageStore(t)
	ready, err := images.DownloadAndStore(server.URL, server.URL, 1, "ready", nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.SavedImagesDir = filepath.Join(t.TempDir(), "saved")
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := images.SaveImage(ready.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(images.cacheDir, "1", ready.FileID+".png")); err != nil {
		t.Fatal(err)
	}

	images.CleanupOrphanPooled()
	if _, _, err := images.GetImage(ready.FileID); err != nil {
		t.Fatalf("saved pooled image was treated as orphan: %v", err)
	}
}
