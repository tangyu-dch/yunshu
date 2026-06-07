package middleware

import (
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"yunshu/internal/contracts"
)

var (
	webhookSecretOnce  sync.Once
	webhookSecretValue string
	webhookSecretSet   bool
)

// loadWebhookSecret reads KAMAILIO_WEBHOOK_SECRET from the environment exactly once.
func loadWebhookSecret() {
	webhookSecretOnce.Do(func() {
		webhookSecretValue = os.Getenv("KAMAILIO_WEBHOOK_SECRET")
		webhookSecretSet = webhookSecretValue != ""
	})
}

// KamailioWebhookAuth validates the X-Webhook-Secret header on Kamailio
// callback endpoints. The expected secret is read from the
// KAMAILIO_WEBHOOK_SECRET environment variable.
//
// If the variable is empty / unset the middleware allows every request through
// (convenient for local development) but emits a warning on the first call so
// that operators know the endpoints are unprotected.
func KamailioWebhookAuth() gin.HandlerFunc {
	loadWebhookSecret()

	warned := false

	return func(c *gin.Context) {
		if !webhookSecretSet {
			if !warned {
				slog.Warn("KAMAILIO_WEBHOOK_SECRET 未设置，Kamailio Webhook 端点处于无鉴权状态，请仅在本地开发环境使用")
				warned = true
			}
			c.Next()
			return
		}

		provided := c.GetHeader("X-Webhook-Secret")
		if provided != webhookSecretValue {
			c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "Webhook 鉴权失败"))
			c.Abort()
			return
		}
		c.Next()
	}
}
