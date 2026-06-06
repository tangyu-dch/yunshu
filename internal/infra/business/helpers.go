package business

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func hmacSign(raw []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		var parsed int
		_, _ = fmt.Sscanf(typed, "%d", &parsed)
		return parsed
	default:
		return 0
	}
}

func timeValue(value any) time.Time {
	if value == nil {
		return time.Time{}
	}
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC()
	case *time.Time:
		if typed == nil {
			return time.Time{}
		}
		return typed.UTC()
	case string:
		parsed, _ := time.Parse(time.RFC3339Nano, typed)
		return parsed.UTC()
	default:
		return time.Time{}
	}
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func firstString(candidates ...any) string {
	for _, val := range candidates {
		if val != nil {
			str := fmt.Sprint(val)
			if str != "" {
				return str
			}
		}
	}
	return ""
}

// floatValue 将任意类型值转换为 float64。
func floatValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		var parsed float64
		_, _ = fmt.Sscanf(typed, "%f", &parsed)
		return parsed
	default:
		return 0
	}
}
