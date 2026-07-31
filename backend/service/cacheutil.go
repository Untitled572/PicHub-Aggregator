package service

import (
	"os"
	"path/filepath"
	"time"
)

// dirSize 递归计算目录总字节数
func dirSize(dir string) int64 {
	var total int64
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// listCacheFiles 收集缓存目录下所有文件(含按源分子目录), 供淘汰/清理使用
func listCacheFiles(cacheDir string) []string {
	var files []string
	root, _ := filepath.Glob(filepath.Join(cacheDir, "*"))
	for _, e := range root {
		info, err := os.Stat(e)
		if err != nil {
			continue
		}
		if info.IsDir() {
			sub, _ := filepath.Glob(filepath.Join(e, "*"))
			files = append(files, sub...)
		} else {
			files = append(files, e)
		}
	}
	return files
}

// oldestFile 返回候选路径中最旧的文件; 无有效候选返回空串
func oldestFile(paths []string) string {
	var oldest string
	var oldestTime time.Time
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if oldest == "" || info.ModTime().Before(oldestTime) {
			oldest = p
			oldestTime = info.ModTime()
		}
	}
	return oldest
}
