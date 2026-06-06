package storage

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	StatusPending   = "pending"
	StatusSkipped = "skipped"
	StatusFailed  = "failed"
	StatusUploaded  = "uploaded"
)

// OSSUploader 封装 S3 兼容对象存储的录音文件上传能力。
// 支持 MinIO、RustFS、阿里云 OSS 等 S3 兼容接口。
type OSSUploader struct {
	Client     *minio.Client
	Bucket     string
	BaseDir    string
	CDNBaseURL string
	Logger     *slog.Logger
}

// NewOSSUploader 创建 OSS 上传器。
func NewOSSUploader(endpoint, accessKey, secretKey, bucket, baseDir, cdnBaseURL string, logger *slog.Logger) (*OSSUploader, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("OSS endpoint is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	useSSL := strings.HasPrefix(endpoint, "https://")
	cleanEndpoint := strings.TrimPrefix(endpoint, "http://")
	cleanEndpoint = strings.TrimPrefix(cleanEndpoint, "https://")

	minioClient, err := minio.New(cleanEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	if bucket == "" {
		bucket = "recordings"
	}

	return &OSSUploader{
		Client:     minioClient,
		Bucket:     bucket,
		BaseDir:    baseDir,
		CDNBaseURL: cdnBaseURL,
		Logger:     logger,
	}, nil
}

// Upload 读取本地录音文件并上传至 OSS，返回 CDN 访问 URL。
func (u *OSSUploader) Upload(ctx context.Context, localPath, objectKey string) (string, error) {
	fullPath := u.buildFullPath(localPath)

	if u.Logger != nil {
		u.Logger.Info("准备上传录音文件", "localPath", localPath, "fullPath", fullPath, "objectKey", objectKey)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	contentType := u.detectContentType(localPath)

	uploadInfo, err := u.Client.PutObject(ctx, u.Bucket, objectKey, file, fileInfo.Size(), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to OSS: %w", err)
	}

	if u.Logger != nil {
		u.Logger.Info("录音文件上传成功", "objectKey", objectKey, "size", uploadInfo.Size, "etag", uploadInfo.ETag)
	}

	cdnURL := u.buildCDNURL(objectKey)
	return cdnURL, nil
}

// buildFullPath 构建完整的本地文件路径。
func (u *OSSUploader) buildFullPath(localPath string) string {
	if filepath.IsAbs(localPath) {
		return localPath
	}
	return filepath.Join(u.BaseDir, localPath)
}

// detectContentType 根据文件扩展名检测 Content-Type。
func (u *OSSUploader) detectContentType(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	default:
		return mime.TypeByExtension(ext)
	}
}

// buildCDNURL 构建 CDN 访问 URL。
func (u *OSSUploader) buildCDNURL(objectKey string) string {
	if u.CDNBaseURL != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(u.CDNBaseURL, "/"), strings.TrimLeft(objectKey, "/"))
	}
	return fmt.Sprintf("s3://%s/%s", u.Bucket, objectKey)
}

// GenerateObjectKey 生成录音文件的 objectKey。
// 使用 {merchantId}/{date}/{callId}.{ext} 格式。
func GenerateObjectKey(merchantID int, dateStr, callID, recordFilePath string) string {
	ext := filepath.Ext(recordFilePath)
	if ext == "" {
		ext = ".mp3"
	}
	return fmt.Sprintf("%d/%s/%s%s", merchantID, dateStr, callID, ext)
}
