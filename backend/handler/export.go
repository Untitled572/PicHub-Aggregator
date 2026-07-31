package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pichub/backend/model"
)

func (h *Handler) ExportData(c *gin.Context) {
	scopeStr := c.DefaultQuery("scope", "config,stats,images")
	scopes := strings.Split(scopeStr, ",")
	scopeMap := make(map[string]bool)
	for _, s := range scopes {
		scopeMap[strings.TrimSpace(s)] = true
	}

	manifest := model.ExportManifest{
		Version:    "0.4.0",
		ExportedAt: time.Now().Format(time.RFC3339),
		Scope:      scopes,
	}

	if scopeMap["config"] {
		if settings, err := h.store.GetSettings(); err == nil {
			settings.AdminToken = ""
			manifest.Settings = settings
		}
		if sources, err := h.store.ListSources(); err == nil {
			sanitized := make([]model.Source, 0, len(sources))
			for _, src := range sources {
				if src.Headers != nil {
					hdrs := make(map[string]string, len(src.Headers))
					for k, v := range src.Headers {
						if !isSensitive(k) {
							hdrs[k] = v
						}
					}
					src.Headers = hdrs
				}
				sanitized = append(sanitized, src)
			}
			manifest.Sources = sanitized
		}
		if tags, err := h.store.GetTags(); err == nil {
			manifest.Tags = tags
		}
	}

	if scopeMap["stats"] {
		if stats, err := h.store.ExportStatsData(); err == nil {
			manifest.Stats = stats
		}
	}

	if scopeMap["images"] {
		if savedImgs, err := h.store.ExportSavedImages(); err == nil {
			manifest.SavedImages = savedImgs
		}
	}

	hasImages := scopeMap["images"] && len(manifest.SavedImages) > 0

	if hasImages {
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"pichub_backup_%s.zip\"", time.Now().Format("20060102_150405")))

		zipWriter := zip.NewWriter(c.Writer)
		defer zipWriter.Close()

		// 1. Write manifest.json
		manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
		mfWriter, err := zipWriter.Create("manifest.json")
		if err == nil {
			mfWriter.Write(manifestBytes)
		}

		// 2. Write saved image files
		settings, _ := h.store.GetSettings()
		savedDir := "./data/saved"
		if settings != nil && settings.SavedImagesDir != "" {
			savedDir = settings.SavedImagesDir
		}

		for _, img := range manifest.SavedImages {
			pattern := filepath.Join(savedDir, img.FileID+".*")
			matches, _ := filepath.Glob(pattern)
			if len(matches) > 0 {
				srcFile := matches[0]
				fileData, err := os.ReadFile(srcFile)
				if err == nil {
					entryName := filepath.Join("saved_images", filepath.Base(srcFile))
					imgWriter, err := zipWriter.Create(entryName)
					if err == nil {
						imgWriter.Write(fileData)
					}
				}
			}
		}
		return
	}

	// JSON Export
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"pichub_backup_%s.json\"", time.Now().Format("20060102_150405")))
	c.Data(http.StatusOK, "application/json", manifestBytes)
}

func (h *Handler) ImportData(c *gin.Context) {
	var manifest model.ExportManifest
	var zipFileData map[string][]byte

	contentType := c.GetHeader("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file upload required"})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "open file failed"})
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "read file failed"})
			return
		}

		// Try ZIP parse first
		zr, err := zip.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
		if err == nil {
			zipFileData = make(map[string][]byte)
			for _, f := range zr.File {
				rc, err := f.Open()
				if err == nil {
					b, _ := io.ReadAll(rc)
					rc.Close()
					zipFileData[f.Name] = b
					if f.Name == "manifest.json" {
						json.Unmarshal(b, &manifest)
					}
				}
			}
		} else {
			// Fallback to JSON parse
			if err := json.Unmarshal(fileBytes, &manifest); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON or ZIP backup file"})
				return
			}
		}
	} else {
		// Pure JSON payload
		if err := c.ShouldBindJSON(&manifest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON manifest payload"})
			return
		}
	}

	importedSources := 0
	importedStats := 0
	importedImages := 0

	// Restore Settings (skip admin_token / admin_password_hash)
	if manifest.Settings != nil {
		localSettings, _ := h.store.GetSettings()
		var localToken, localPasswordHash string
		if localSettings != nil {
			localToken = localSettings.AdminToken
			localPasswordHash = localSettings.AdminPasswordHash
		}
		manifest.Settings.AdminToken = ""
		manifest.Settings.AdminPasswordHash = localPasswordHash
		manifest.Settings.AdminPassword = ""
		_ = h.store.UpdateSettings(manifest.Settings)
		manifest.Settings.AdminToken = localToken
	}

	// Restore Tags
	if len(manifest.Tags) > 0 {
		_ = h.store.UpdateTags(manifest.Tags)
	}

	// Restore Sources
	if len(manifest.Sources) > 0 {
		for _, src := range manifest.Sources {
			src.ID = 0
			_, _ = h.store.CreateSource(&src)
			importedSources++
		}
	}

	// Restore Stats
	if manifest.Stats != nil {
		_ = h.store.ImportStatsData(manifest.Stats)
		importedStats = len(manifest.Stats.StatsRequests) + len(manifest.Stats.ImageHistory)
	}

	// Restore Saved Images Records & Extract ZIP Binary Files
	settings, _ := h.store.GetSettings()
	savedDir := "./data/saved"
	if settings != nil && settings.SavedImagesDir != "" {
		savedDir = settings.SavedImagesDir
	}
	os.MkdirAll(savedDir, 0755)

	if len(manifest.SavedImages) > 0 {
		for _, img := range manifest.SavedImages {
			_ = h.store.ImportSavedImage(&img)
			importedImages++
		}
	}

	if zipFileData != nil {
		for fileName, data := range zipFileData {
			if strings.HasPrefix(fileName, "saved_images/") && !strings.HasSuffix(fileName, "/") {
				baseName := filepath.Base(fileName)
				dstPath := filepath.Join(savedDir, baseName)
				os.WriteFile(dstPath, data, 0644)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "Import process completed",
		"imported_sources": importedSources,
		"imported_stats":   importedStats,
		"imported_images":  importedImages,
	})
}
