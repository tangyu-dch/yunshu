package operate

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ASRConfig ASR配置
type ASRConfig struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Provider string                 `json:"provider"`
	APIKey   string                 `json:"apiKey"`
	Endpoint string                 `json:"endpoint,omitempty"`
	Language string                 `json:"language,omitempty"`
	Enabled  bool                   `json:"enabled"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TTSConfig TTS配置
type TTSConfig struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Provider  string                 `json:"provider"`
	APIKey    string                 `json:"apiKey"`
	Endpoint  string                 `json:"endpoint,omitempty"`
	VoiceType string                 `json:"voiceType,omitempty"`
	Speed     float64                `json:"speed,omitempty"`
	Enabled   bool                   `json:"enabled"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ASRTTSManagementService ASR/TTS管理服务
type ASRTTSManagementService struct {
	asrConfigs map[string]*ASRConfig
	ttsConfigs map[string]*TTSConfig
	mu         sync.RWMutex
}

// NewASRTTSManagementService 创建ASR/TTS管理服务
func NewASRTTSManagementService() *ASRTTSManagementService {
	return &ASRTTSManagementService{
		asrConfigs: make(map[string]*ASRConfig),
		ttsConfigs: make(map[string]*TTSConfig),
	}
}

// ListASRConfigs 列出所有ASR配置
func (s *ASRTTSManagementService) ListASRConfigs(ctx context.Context) ([]*ASRConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ASRConfig, 0, len(s.asrConfigs))
	for _, cfg := range s.asrConfigs {
		result = append(result, cfg)
	}
	return result, nil
}

// SaveASRConfig 保存ASR配置
func (s *ASRTTSManagementService) SaveASRConfig(ctx context.Context, cfg ASRConfig) (*ASRConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("asr-%d", now.Unix())
	}

	s.asrConfigs[cfg.ID] = &cfg
	return &cfg, nil
}

// DeleteASRConfig 删除ASR配置
func (s *ASRTTSManagementService) DeleteASRConfig(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		delete(s.asrConfigs, id)
	}
	return nil
}

// ListTTSConfigs 列出所有TTS配置
func (s *ASRTTSManagementService) ListTTSConfigs(ctx context.Context) ([]*TTSConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*TTSConfig, 0, len(s.ttsConfigs))
	for _, cfg := range s.ttsConfigs {
		result = append(result, cfg)
	}
	return result, nil
}

// SaveTTSConfig 保存TTS配置
func (s *ASRTTSManagementService) SaveTTSConfig(ctx context.Context, cfg TTSConfig) (*TTSConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("tts-%d", now.Unix())
	}

	s.ttsConfigs[cfg.ID] = &cfg
	return &cfg, nil
}

// DeleteTTSConfig 删除TTS配置
func (s *ASRTTSManagementService) DeleteTTSConfig(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		delete(s.ttsConfigs, id)
	}
	return nil
}
