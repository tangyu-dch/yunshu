package callflow

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// 1. 通用 AI 服务接口定义 (ASR / TTS / LLM)
// ============================================================================

// ASREngine 语音识别转写引擎接口，用于对接不同厂商的语音转文字服务。
type ASREngine interface {
	// Transcribe 输入音频二进制裸数据，输出转写后的文本内容。
	Transcribe(ctx context.Context, audioData []byte, format string, config map[string]any) (string, error)
}

// TTSEngine 语音合成引擎接口，用于对接不同厂商的文字转语音服务，并支持缓存。
type TTSEngine interface {
	// Synthesize 输入需要播报的文本，输出合成后的音频二进制数据。
	Synthesize(ctx context.Context, text string, config map[string]any) ([]byte, error)
}

// LLMEngine 大语言模型引擎接口，用于支持不同厂商的智能决策与话术决策服务。
type LLMEngine interface {
	// GenerateReply 输入角色设定与用户的提问，输出模型决策的回复。
	GenerateReply(ctx context.Context, systemPrompt, userMessage string, config map[string]any) (string, error)
}

// ============================================================================
// 2. 全局多引擎注册与获取机制 (解耦硬编码，方便未来扩展阿里/百度/DeepSeek)
// ============================================================================

var (
	asrRegistry = make(map[string]ASREngine)
	ttsRegistry = make(map[string]TTSEngine)
	llmRegistry = make(map[string]LLMEngine)
)

// RegisterASREngine 注册 ASR 引擎。
func RegisterASREngine(provider string, engine ASREngine) {
	asrRegistry[strings.ToLower(provider)] = engine
}

// GetASREngine 根据厂商标识获取 ASR 引擎，若未找到则默认返回火山引擎。
func GetASREngine(provider string) ASREngine {
	if eng, ok := asrRegistry[strings.ToLower(provider)]; ok {
		return eng
	}
	return asrRegistry["volc"]
}

// RegisterTTSEngine 注册 TTS 引擎。
func RegisterTTSEngine(provider string, engine TTSEngine) {
	ttsRegistry[strings.ToLower(provider)] = engine
}

// GetTTSEngine 根据厂商标识获取 TTS 引擎，若未找到则默认返回火山引擎。
func GetTTSEngine(provider string) TTSEngine {
	if eng, ok := ttsRegistry[strings.ToLower(provider)]; ok {
		return eng
	}
	return ttsRegistry["volc"]
}

// RegisterLLMEngine 注册 LLM 大模型引擎。
func RegisterLLMEngine(provider string, engine LLMEngine) {
	llmRegistry[strings.ToLower(provider)] = engine
}

// GetLLMEngine 根据厂商标识获取 LLM 引擎，若未找到则默认返回火山引擎。
func GetLLMEngine(provider string) LLMEngine {
	if eng, ok := llmRegistry[strings.ToLower(provider)]; ok {
		return eng
	}
	return llmRegistry["volc"]
}

// ============================================================================
// 4. 便捷工具函数：支持以文本/配置哈希做物理 TTS 缓存的封装播发
// ============================================================================

// SynthesizeAndCacheTTS 统一控制不同 TTS 接口，并完成本地 MP3 文件级 MD5 缓存物理落盘。
func SynthesizeAndCacheTTS(ctx context.Context, text string, provider string, config map[string]any) (string, error) {
	// 以文字与核心音色参数做 MD5 hash 防止冲突
	voiceType, _ := config["volcVoiceType"].(string)
	speedRatio, _ := config["volcSpeedRatio"].(float64)

	key := fmt.Sprintf("%s:%s:%s:%.2f", text, provider, voiceType, speedRatio)
	hash := md5.Sum([]byte(key))
	fileName := hex.EncodeToString(hash[:]) + ".mp3"

	// 统一创建静态目录
	cacheDir := filepath.Join("data", "tts_cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	filePath := filepath.Join(cacheDir, fileName)

	// 检查物理缓存是否存在
	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil
	}

	// 命中对应的 TTS 物理提供商
	ttsEng := GetTTSEngine(provider)
	audioBytes, err := ttsEng.Synthesize(ctx, text, config)
	if err != nil {
		return "", err
	}

	if len(audioBytes) == 0 {
		return "", fmt.Errorf("empty audio data synthesized")
	}

	// 物理落盘
	if err := os.WriteFile(filePath, audioBytes, 0644); err != nil {
		return "", err
	}

	return filePath, nil
}

// IsProviderImplemented 检查某个服务商是否在 runtime 至少注册了 ASR、TTS 或 LLM 引擎中的任意一个驱动。
func IsProviderImplemented(provider string) bool {
	return IsAsrImplemented(provider) || IsTtsImplemented(provider) || IsLlmImplemented(provider)
}

// IsAsrImplemented 检查某个服务商是否在 runtime 注册了 ASR 引擎驱动。
func IsAsrImplemented(provider string) bool {
	p := strings.ToLower(provider)
	_, ok := asrRegistry[p]
	return ok
}

// IsTtsImplemented 检查某个服务商是否在 runtime 注册了 TTS 引擎驱动。
func IsTtsImplemented(provider string) bool {
	p := strings.ToLower(provider)
	_, ok := ttsRegistry[p]
	return ok
}

// IsLlmImplemented 检查某个服务商是否在 runtime 注册了 LLM 引擎驱动。
func IsLlmImplemented(provider string) bool {
	p := strings.ToLower(provider)
	_, ok := llmRegistry[p]
	return ok
}
