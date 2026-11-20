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
	settings, err := e.store.GetSettings()
	if err != nil {
		return err
	}
	if !settings.ProxyMode || settings.PoolSize <= 0 {
		return nil
	}
	sources, err := e.store.ListSources()
	if err != nil {
		return err
	}
	active := make(map[int64]bool, len(sources))
	for _, src := range sources {
		active[src.ID] = src.Enabled && src.Status != "error"
	}
	images, err := e.store.ListPooledImages()
	if err != nil {
		return err
	}
	var restored []*PoolEntry
	var retired int
	for _, img := range images {
		entry := &PoolEntry{
			FileID: img.FileID, ImageID: img.ID, SourceID: img.SourceID, SourceName: img.SourceName,
			Width: img.Width, Height: img.Height, Format: img.Format, Orientation: GetOrientation(img.Width, img.Height),
		}
		available := len(restored) < settings.PoolSize && active[img.SourceID] && !img.IsSaved &&
			img.FileID != "" && img.Width > 0 && img.Height > 0 && img.Format != "" && img.Format != "unknown" &&
			json.Unmarshal([]byte(img.Categories), &entry.Categories) == nil
		if available && e.hasRestorableFile(img.FileID) {
			restored = append(restored, entry)
			continue
		}
		if err := e.store.SetImagePooled(img.ID, false); err != nil {
			logger.Error("clear stale prefetch image %d: %v", img.ID, err)
		} else {
			retired++
		}
	}
	// Keep the most recent images within capacity, then resume FIFO delivery.
	for i := len(restored) - 1; i >= 0; i-- {
		e.distPool.Push(restored[i])
	}
	logger.System("prefetch pool restored: %d/%d images, %d stale entries retired", len(restored), settings.PoolSize, retired)
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
