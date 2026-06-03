package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// RustFSConfig 封装 S3 兼容型 RustFS 系统的连接凭证。
type RustFSConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
}

// RustFSStorage 封装面向 RustFS/S3 兼容性存储和本地存储的统一文件管理器。
type RustFSStorage struct {
	cfg RustFSConfig
}

// NewRustFSStorage 实例化文件管理器。
func NewRustFSStorage(cfg RustFSConfig) *RustFSStorage {
	return &RustFSStorage{cfg: cfg}
}

// Store 将桌面端二进制安装包进行持久化存储。
// 优先采用 HTTP PUT 协议直传至 S3 兼容型 RustFS 服务端；若未配置 Endpoint，则自动退避存入本地 data/updates/ 目录。
func (s *RustFSStorage) Store(ctx context.Context, filename string, reader io.Reader) (string, error) {
	// 1. 如果配置了 RustFS 对象存储服务，执行云端 PUT 写入
	if s.cfg.Endpoint != "" {
		url := fmt.Sprintf("%s/%s/%s", s.cfg.Endpoint, s.cfg.Bucket, filename)
		req, err := http.NewRequestWithContext(ctx, "PUT", url, reader)
		if err != nil {
			return "", err
		}

		req.Header.Set("Content-Type", "application/octet-stream")
		// 支持标准 BasicAuth 方式与 RustFS 网关准入交互
		if s.cfg.AccessKey != "" && s.cfg.SecretKey != "" {
			req.SetBasicAuth(s.cfg.AccessKey, s.cfg.SecretKey)
		}

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("直传 RustFS 对象存储失败: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("RustFS 返回错误状态码 %d: %s", resp.StatusCode, string(body))
		}
		return url, nil
	}

	// 2. 本地退避方案：直接写入本地的 data/updates 目录下以备分发
	localDir := filepath.Join("data", "updates")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return "", fmt.Errorf("创建本地更新文件夹失败: %w", err)
	}

	targetPath := filepath.Join(localDir, filename)
	out, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("创建本地更新物理文件失败: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, reader); err != nil {
		return "", fmt.Errorf("写入本地更新物理数据失败: %w", err)
	}

	// 返回本地的相对访问路径，后端静态文件路由可对其进行映射分发
	return "/" + targetPath, nil
}

// Delete 从对象存储或本地文件系统中删除指定的版本二进制包。
func (s *RustFSStorage) Delete(ctx context.Context, filename string) error {
	// 1. 如果配置了 RustFS 对象存储服务，执行云端 DELETE 操作
	if s.cfg.Endpoint != "" {
		url := fmt.Sprintf("%s/%s/%s", s.cfg.Endpoint, s.cfg.Bucket, filename)
		req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
		if err != nil {
			return err
		}

		// 支持标准 BasicAuth 方式与 RustFS 网关准入交互
		if s.cfg.AccessKey != "" && s.cfg.SecretKey != "" {
			req.SetBasicAuth(s.cfg.AccessKey, s.cfg.SecretKey)
		}

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("直连 RustFS 删除对象失败: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("RustFS 删除返回错误状态码 %d: %s", resp.StatusCode, string(body))
		}
		return nil
	}

	// 2. 本地退避方案：直接删除本地的 data/updates 目录下对应的物理文件
	localPath := filepath.Join("data", "updates", filename)
	if _, err := os.Stat(localPath); err == nil {
		if err := os.Remove(localPath); err != nil {
			return fmt.Errorf("删除本地物理更新包失败: %w", err)
		}
	}
	return nil
}

