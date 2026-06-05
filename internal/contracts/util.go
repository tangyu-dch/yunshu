package contracts

import (
	"fmt"
	"strconv"
)

// StringFromMap 从 map[string]any 中安全提取字符串值
func StringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// BoolFromMap 从 map[string]any 中安全提取布尔值
func BoolFromMap(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		switch b := v.(type) {
		case bool:
			return b
		case string:
			return b == "true" || b == "1"
		case float64:
			return b != 0
		case int:
			return b != 0
		}
	}
	return false
}

// IntFromMap 从 map[string]any 中安全提取整数值
func IntFromMap(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		case string:
			if i, err := strconv.Atoi(n); err == nil {
				return i
			}
		}
	}
	return 0
}

// FirstNonEmpty 返回第一个非空字符串
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
