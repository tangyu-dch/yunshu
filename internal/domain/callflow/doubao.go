package callflow

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// ============================================================================
// 🌋 火山引擎“豆包”AI Suite 物理驱动实现 (ASR / TTS / LLM)
// ============================================================================

func init() {
	// 将火山语音/豆包驱动物理注册进全局 AI 引擎注册表中
	RegisterASREngine("volc", &VolcanoASREngine{})
	RegisterTTSEngine("volc", &VolcanoTTSEngine{})
	RegisterLLMEngine("volc", &VolcanoLLMEngine{})

	// 兼容不同的别名，包括 "doubao", "volcano"
	RegisterASREngine("volcano", &VolcanoASREngine{})
	RegisterTTSEngine("volcano", &VolcanoTTSEngine{})
	RegisterLLMEngine("volcano", &VolcanoLLMEngine{})

	RegisterASREngine("doubao", &VolcanoASREngine{})
	RegisterTTSEngine("doubao", &VolcanoTTSEngine{})
	RegisterLLMEngine("doubao", &VolcanoLLMEngine{})
}

// ----------------------------------------------------------------------------
// ASR (语音转文字) 物理驱动：对接火山 OpenSpeech ASR 极速接口
// ----------------------------------------------------------------------------

type VolcanoASREngine struct{}

// Transcribe 物理转写：将 PCM/WAV 裸音频数据上传火山极速语音识别接口。
func (e *VolcanoASREngine) Transcribe(ctx context.Context, audioData []byte, format string, config map[string]any) (string, error) {
	appid, _ := config["volcAppId"].(string)
	token, _ := config["volcToken"].(string)
	cluster, _ := config["volcCluster"].(string)

	if appid == "" || token == "" {
		return "", fmt.Errorf("缺失火山语音识别 ASR 凭证：AppId 或 Token 为空")
	}
	if cluster == "" {
		cluster = "volc_common_asr"
	}

	url := "https://openspeech.bytedance.com/api/v1/asr"

	type VolcASRRequest struct {
		App struct {
			Appid   string `json:"appid"`
			Token   string `json:"token"`
			Cluster string `json:"cluster"`
		} `json:"app"`
		User struct {
			Uid string `json:"uid"`
		} `json:"user"`
		Audio struct {
			Format  string `json:"format"`
			Codec   string `json:"codec"`
			Rate    int    `json:"rate"`
			Channel int    `json:"channel"`
			Bits    int    `json:"bits"`
		} `json:"audio"`
		Request struct {
			Reqid    string `json:"reqid"`
			Workflow string `json:"workflow"`
		} `json:"request"`
		AudioData string `json:"audio_data"`
	}

	reqBody := VolcASRRequest{}
	reqBody.App.Appid = appid
	reqBody.App.Token = token
	reqBody.App.Cluster = cluster
	reqBody.User.Uid = "yunshu_user"
	reqBody.Audio.Format = format
	reqBody.Audio.Codec = "raw"
	reqBody.Audio.Rate = 16000
	reqBody.Audio.Channel = 1
	reqBody.Audio.Bits = 16
	reqBody.Request.Reqid = "req-asr-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	reqBody.Request.Workflow = "audio_in"
	reqBody.AudioData = hex.EncodeToString(audioData)

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer; "+token)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("火山 ASR 物理转译失败：HTTP 状态码 %d", resp.StatusCode)
	}

	var res struct {
		Resp struct {
			Text string `json:"text"`
		} `json:"resp"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	return res.Resp.Text, nil
}

// ----------------------------------------------------------------------------
// TTS (文字转语音) 物理驱动：对接火山 OpenSpeech TTS 情绪合成极速接口
// ----------------------------------------------------------------------------

type VolcanoTTSEngine struct{}

// Synthesize 物理合成：调用火山语音情绪发音人合成并输出 MP3 二进制流。
func (e *VolcanoTTSEngine) Synthesize(ctx context.Context, text string, config map[string]any) ([]byte, error) {
	appid, _ := config["volcAppId"].(string)
	token, _ := config["volcToken"].(string)
	cluster, _ := config["volcCluster"].(string)
	voiceType, _ := config["volcVoiceType"].(string)
	speedRatioVal, _ := config["volcSpeedRatio"].(float64)

	if appid == "" || token == "" {
		return nil, fmt.Errorf("缺失火山情绪合成 TTS 凭证：AppId 或 Token 为空")
	}
	if cluster == "" {
		cluster = "volcano_tts"
	}
	if voiceType == "" {
		voiceType = "bv001_streaming"
	}
	if speedRatioVal <= 0 {
		speedRatioVal = 1.0
	}

	url := "https://openspeech.bytedance.com/api/v1/tts"

	type VolcTTSRequest struct {
		App struct {
			Appid   string `json:"appid"`
			Token   string `json:"token"`
			Cluster string `json:"cluster"`
		} `json:"app"`
		User struct {
			Uid string `json:"uid"`
		} `json:"user"`
		Audio struct {
			VoiceType  string  `json:"voice_type"`
			Encoding   string  `json:"encoding"`
			SpeedRatio float64 `json:"speed_ratio"`
		} `json:"audio"`
		Request struct {
			Reqid     string `json:"reqid"`
			Text      string `json:"text"`
			TextType  string `json:"text_type"`
			Operation string `json:"operation"`
		} `json:"request"`
	}

	reqBody := VolcTTSRequest{}
	reqBody.App.Appid = appid
	reqBody.App.Token = token
	reqBody.App.Cluster = cluster
	reqBody.User.Uid = "yunshu_user"
	reqBody.Audio.VoiceType = voiceType
	reqBody.Audio.Encoding = "mp3"
	reqBody.Audio.SpeedRatio = speedRatioVal
	reqBody.Request.Reqid = "req-tts-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	reqBody.Request.Text = text
	reqBody.Request.TextType = "plain"
	reqBody.Request.Operation = "query"

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer; "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("火山 TTS 物理合成失败：HTTP 状态码 %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// ----------------------------------------------------------------------------
// LLM (大模型智能话务) 物理驱动：对接火山方舟 Ark Endpoint 豆包 API
// ----------------------------------------------------------------------------

type VolcanoLLMEngine struct{}

// GenerateReply 物理对话决策：发起物理请求至豆包 Endpoint 获得智能话务决策。
func (e *VolcanoLLMEngine) GenerateReply(ctx context.Context, systemPrompt, userMessage string, config map[string]any) (string, error) {
	token, _ := config["llmApiKey"].(string)
	model, _ := config["llmModel"].(string)
	endpoint, _ := config["llmEndpoint"].(string)

	if token == "" {
		return "", fmt.Errorf("缺失豆包大模型授权 Token/ApiKey")
	}
	if model == "" {
		model = "doubao-pro-32k"
	}
	if endpoint == "" {
		endpoint = "https://ark.cn-beijing.volces.com/api/v3/chat/completions"
	}

	type VolcChatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type VolcChatRequest struct {
		Model       string            `json:"model"`
		Messages    []VolcChatMessage `json:"messages"`
		Temperature float64           `json:"temperature"`
	}

	reqBody := VolcChatRequest{
		Model: model,
		Messages: []VolcChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("豆包 LLM 大模型物理响应错误：HTTP 状态码 %d", resp.StatusCode)
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if len(res.Choices) > 0 {
		return res.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("豆包大语言模型返回空白 Choice 分支")
}
