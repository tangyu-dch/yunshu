package operate

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/callflow"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterAIModelFlowRoutes 注册商户 AI 流程管理接口。
func RegisterAIModelFlowRoutes(r gin.IRoutes, service *operatedomain.AIModelFlowManagementService) {
	r.GET("/merchant/ai-model-flow/providers", func(c *gin.Context) {
		// 厂商基础显示预设 (静态 UI 表现层)
		rawProviders := []struct {
			Value string `json:"value"`
			Label string `json:"label"`
			Emoji string `json:"emoji"`
			Color string `json:"color"`
		}{
			{"deepseek", "DeepSeek API", "🐳", "cyan"},
			{"openai", "OpenAI 兼容接口", "🌐", "purple"},
			{"ali", "阿里通义千问 Qwen", "☁️", "geekblue"},
			{"tencent", "腾讯混元 Hunyuan", "🐧", "blue"},
			{"volc", "火山引擎“豆包”大模型", "🌋", "orange"},
			{"mock", "云枢自研仿真大模型 (MOCK)", "🤖", "gold"},
			{"baidu", "百度文心千帆 ERNIE", "🐻", "red"},
		}

		// 100% 动态自省：读取呼叫中心底层引擎注册表 (Registry) 判定其实际物理就绪状态
		list := make([]map[string]any, 0, len(rawProviders))
		for _, rp := range rawProviders {
			list = append(list, map[string]any{
				"value":      rp.Value,
				"label":      rp.Label,
				"emoji":      rp.Emoji,
				"color":      rp.Color,
				"supportAsr": callflow.IsAsrImplemented(rp.Value),
				"supportTts": callflow.IsTtsImplemented(rp.Value),
				"supportLlm": callflow.IsLlmImplemented(rp.Value),
			})
		}
		c.JSON(http.StatusOK, contracts.OK(list))
	})

	r.POST("/merchant/ai-model-flow/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		var req operatedomain.AIModelFlowPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询 AI 流程失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/merchant/ai-model-flow/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		flow, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeAIModelFlowError(c, err, "AI 流程不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(flow))
	})

	r.PUT("/merchant/ai-model-flow/add", saveAIModelFlow(service))
	r.POST("/merchant/ai-model-flow/update", saveAIModelFlow(service))
	r.POST("/merchant/ai-model-flow/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		ids, ok := parseIDs(c)
		if !ok {
			return
		}
		if err := service.Delete(c.Request.Context(), ids); err != nil {
			writeAIModelFlowError(c, err, "删除 AI 流程失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(ids)}))
	})

	r.POST("/merchant/ai-model-flow/precheck", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		var req operatedomain.AIModelFlow
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		flow, err := service.Precheck(c.Request.Context(), req)
		if err != nil {
			writeAIModelFlowError(c, err, "AI 流程预检查失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(flow))
	})

	r.POST("/merchant/ai-model-flow/publish/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		flow, err := service.Publish(c.Request.Context(), id)
		if err != nil {
			writeAIModelFlowError(c, err, "AI 流程发布失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(flow))
	})

	r.POST("/merchant/ai-model-flow/demo-voice", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}

		// 1. 接收音频上传
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "未接收到有效音频文件"))
			return
		}

		f, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "无法读取音频内容"))
			return
		}
		defer f.Close()

		audioData, err := io.ReadAll(f)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "读取音频内容失败"))
			return
		}

		// 2. 接收多厂商所有透传参数
		volcAppId := c.PostForm("volcAppId")
		volcToken := c.PostForm("volcToken")
		volcCluster := c.PostForm("volcCluster")
		volcVoiceType := c.PostForm("volcVoiceType")
		volcSpeedRatioStr := c.PostForm("volcSpeedRatio")

		aliAppKey := c.PostForm("aliAppKey")
		aliToken := c.PostForm("aliToken")
		aliVoice := c.PostForm("aliVoice")

		tencentSecretId := c.PostForm("tencentSecretId")
		tencentSecretKey := c.PostForm("tencentSecretKey")
		tencentVoice := c.PostForm("tencentVoice")

		openaiVoice := c.PostForm("openaiVoice")

		systemPrompt := c.PostForm("systemPrompt")
		modelName := c.PostForm("model")
		endpoint := c.PostForm("endpoint")
		apiKey := c.PostForm("apiKey") // 通用/OpenAI/DeepSeek 的 API Key

		asrProvider := c.PostForm("asrProvider")
		ttsProvider := c.PostForm("ttsProvider")
		llmProvider := c.PostForm("llmProvider")

		// 默认兜底
		if systemPrompt == "" {
			systemPrompt = "您是云枢呼叫中心的智能大模型助手。"
		}
		if asrProvider == "" {
			asrProvider = "volc"
		}
		if ttsProvider == "" {
			ttsProvider = "volc"
		}
		if llmProvider == "" {
			llmProvider = "volc"
		}

		var volcSpeedRatio float64 = 1.0
		if volcSpeedRatioStr != "" {
			if parsed, err := strconv.ParseFloat(volcSpeedRatioStr, 64); err == nil && parsed > 0 {
				volcSpeedRatio = parsed
			}
		}

		// 3. 执行物理/仿真 ASR (通过通用 ASREngine 多态路由)
		var transcribedText string
		var asrDuration int64
		if asrProvider != "mock" {
			startTime := time.Now()
			format := "wav"
			if strings.HasSuffix(file.Filename, ".webm") {
				format = "webm"
			}

			// 组装当前 ASR 厂商专属参数
			asrConfig := map[string]any{
				"volcAppId":        volcAppId,
				"volcToken":        volcToken,
				"volcCluster":      volcCluster,
				"aliAppKey":        aliAppKey,
				"aliToken":         aliToken,
				"tencentSecretId":  tencentSecretId,
				"tencentSecretKey": tencentSecretKey,
				"llmApiKey":        apiKey, // OpenAI Whisper 使用该 Key
			}

			asrEng := callflow.GetASREngine(asrProvider)
			text, err := asrEng.Transcribe(c.Request.Context(), audioData, format, asrConfig)
			asrDuration = time.Since(startTime).Milliseconds()
			if err == nil && text != "" {
				transcribedText = text
			} else {
				transcribedText = "[ASR 语音转写失败，使用兜底] 您好，请帮我转接一下云枢人工坐席。"
			}
		} else {
			transcribedText = "[仿真模拟 ASR 识别结果] 您好，我想查询一下我的账单信息。"
		}

		// 4. 调用大语言模型进行智能对话 (通过通用 LLMEngine 多态路由)
		var llmResponse string
		var llmDuration int64
		if llmProvider != "mock" {
			startTime := time.Now()

			// 组装当前 LLM 厂商专属参数，并决定大模型 ApiKey
			llmApiKey := apiKey
			if llmProvider == "volc" && llmApiKey == "" {
				llmApiKey = volcToken // 火山大模型兼容 Token
			}
			if llmProvider == "ali" && llmApiKey == "" {
				llmApiKey = aliToken // 阿里 Qwen 兼容 Token
			}

			llmConfig := map[string]any{
				"llmApiKey":        llmApiKey,
				"llmModel":         modelName,
				"llmEndpoint":      endpoint,
				"tencentSecretId":  tencentSecretId,
				"tencentSecretKey": tencentSecretKey,
			}

			llmEng := callflow.GetLLMEngine(llmProvider)
			respText, err := llmEng.GenerateReply(c.Request.Context(), systemPrompt, transcribedText, llmConfig)
			llmDuration = time.Since(startTime).Milliseconds()
			if err == nil && respText != "" {
				llmResponse = respText
			} else {
				llmResponse = "【大模型物理调用报错】您好，云枢已感知到引擎异常，将在 1 秒内为您划拨物理坐席队列。"
			}
		} else {
			llmResponse = "【云枢流程仿真】您好！已成功感知到您的 ASR 输入为：“" + transcribedText + "”。当前正通过云枢仿真引擎为您播报解答。"
		}

		// 5. 调用语音合成 TTS 将文本合成为音频 (通过通用 TTSEngine 多态路由)
		var audioBase64 string
		var ttsDuration int64
		if ttsProvider != "mock" {
			startTime := time.Now()

			// 组装当前 TTS 厂商专属参数
			ttsConfig := map[string]any{
				"volcAppId":        volcAppId,
				"volcToken":        volcToken,
				"volcCluster":      volcCluster,
				"volcVoiceType":    volcVoiceType,
				"volcSpeedRatio":   volcSpeedRatio,
				"aliAppKey":        aliAppKey,
				"aliToken":         aliToken,
				"aliVoice":         aliVoice,
				"tencentSecretId":  tencentSecretId,
				"tencentSecretKey": tencentSecretKey,
				"tencentVoice":     tencentVoice,
				"openaiVoice":      openaiVoice,
				"llmApiKey":        apiKey, // OpenAI TTS 使用该 Key
			}

			ttsEng := callflow.GetTTSEngine(ttsProvider)
			audioBytes, err := ttsEng.Synthesize(c.Request.Context(), llmResponse, ttsConfig)
			ttsDuration = time.Since(startTime).Milliseconds()
			if err == nil && len(audioBytes) > 0 {
				audioBase64 = base64.StdEncoding.EncodeToString(audioBytes)
			}
		}

		// 6. 返回 JSON 跟踪数据
		c.JSON(http.StatusOK, contracts.OK(map[string]any{
			"asrText":     transcribedText,
			"llmResponse": llmResponse,
			"audioBase64": audioBase64,
			"durations": map[string]int64{
				"asr": asrDuration,
				"llm": llmDuration,
				"tts": ttsDuration,
			},
		}))
	})
}

func saveAIModelFlow(service *operatedomain.AIModelFlowManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		var req operatedomain.AIModelFlow
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeAIModelFlowError(c, err, "保存 AI 流程失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	}
}

func writeAIModelFlowError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrAIModelFlowNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "AI 流程不存在"))
	case errors.Is(err, operatedomain.ErrInvalidAIModelFlow):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "AI 流程参数错误"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
