// Package logging 提供结构化日志创建和统一日志字段定义能力。
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format 定义日志输出格式枚举。
type Format string

// 日志格式常量：JSON 格式（生产环境推荐）或 Text 格式（本地调试）。
const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

// Config 定义日志器配置，包含服务名称、日志级别、输出格式和输出目标。
type Config struct {
	Service string
	Level   string
	Format  Format
	Writer  io.Writer
}

// New 创建服务级结构化日志器。
// 默认使用 JSON，便于生产环境由日志平台采集；本地调试可以切换为 text。
func New(cfg Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Format == FormatText {
		handler = slog.NewTextHandler(writer, opts)
	} else {
		handler = slog.NewJSONHandler(writer, opts)
	}
	return slog.New(handler).With(slog.String("service", cfg.Service))
}

// SetDefault 安装全局日志器，供尚未显式注入 logger 的旧代码或基础库使用。
func SetDefault(logger *slog.Logger) {
	if logger != nil {
		slog.SetDefault(logger)
	}
}
