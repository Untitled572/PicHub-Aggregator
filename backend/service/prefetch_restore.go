package service

import (
	"encoding/json"
	"os"

	"github.com/pichub/backend/logger"
)

func (e *Engine) restorePrefetchPool() error {
	if e.imageStore == nil {
		return nil
	}
	images, err := e.store.ListPooledImages()
	if err != nil {
		return err
	}
	var restored []*PoolEntry
	for _, img := range images {
		entry := &PoolEntry{
			FileID: img.FileID, ImageID: img.ID, SourceID: img.SourceID, SourceName: img.SourceName,
			Width: img.Width, Height: img.Height, Format: img.Format, Orientation: GetOrientation(img.Width, img.Height),
		}
		available := img.FileID != "" && img.Width > 0 && img.Height > 0 && img.Format != "" && img.Format != "unknown" &&
			json.Unmarshal([]byte(img.Categories), &entry.Categories) == nil
		if available && e.hasRestorableFile(img.FileID) {
			restored = append(restored, entry)
		}
	}
	// ListPooledImages returns newest first; put the oldest at the front.
	for i := len(restored) - 1; i >= 0; i-- {
		e.distPool.Push(restored[i])
	}
	logger.System("prefetch pool restored: %d images", len(restored))
	return nil
}

func (e *Engine) hasRestorableFile(fileID string) bool {
	path, _, err := e.imageStore.GetImage(fileID)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}
