package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// LocalTTSCacheStore 实现了 callflow.TTSCacheStore 接口，使用本地文件系统进行缓存物理落盘。
type LocalTTSCacheStore struct {
	cacheDir string
}

// NewLocalTTSCacheStore 创建一个本地 TTS 缓存管理器。
func NewLocalTTSCacheStore(dir string) *LocalTTSCacheStore {
	if dir == "" {
		dir = filepath.Join("data", "tts_cache")
	}
	return &LocalTTSCacheStore{cacheDir: dir}
}

// Exists 检查物理缓存是否存在，若存在返回其绝对/相对路径。
func (s *LocalTTSCacheStore) Exists(ctx context.Context, filename string) (string, bool) {
	filePath := filepath.Join(s.cacheDir, filename)
	if _, err := os.Stat(filePath); err == nil {
		return filePath, true
	}
	return "", false
}

// Save 将音频二进制数据物理落盘到指定路径。
func (s *LocalTTSCacheStore) Save(ctx context.Context, filename string, data []byte) (string, error) {
	if err := os.MkdirAll(s.cacheDir, 0755); err != nil {
		return "", fmt.Errorf("创建 TTS 缓存目录失败: %w", err)
	}
	filePath := filepath.Join(s.cacheDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("写入 TTS 缓存文件失败: %w", err)
	}
	return filePath, nil
}
