package http

import (
	"net/http"
	"time"
)

// DefaultHTTPClient 是对标准 http.Client 的包装，满足 domain 层的 HTTPClient 接口。
type DefaultHTTPClient struct {
	client *http.Client
}

// NewDefaultHTTPClient 创建一个指定超时的默认 HTTP 客户端。
func NewDefaultHTTPClient(timeout time.Duration) *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{Timeout: timeout},
	}
}

// Do 执行 HTTP 请求。
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}
