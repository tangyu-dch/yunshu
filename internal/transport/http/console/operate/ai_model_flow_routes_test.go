package operate

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

func TestAIModelFlowRoutes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.AIModelFlowManagementService{
		Repository: operatedomain.NewMemoryAIModelFlowRepository(),
	}
	RegisterAIModelFlowRoutes(router, service)

	saveReq := httptest.NewRequest(http.MethodPut, "/merchant/ai-model-flow/add", bytes.NewReader([]byte(`{
		"name": "流程A",
		"prompt": "请处理呼叫",
		"customReplies": [
			{
				"id": "rule-1",
				"name": "查话费意图",
				"matchMode": "semantic",
				"intent": "intent_billing",
				"replyText": "您的当前账户余额为 58 元",
				"action": "continue"
			},
			{
				"id": "rule-2",
				"name": "转人工按键",
				"matchMode": "dtmf",
				"intent": "9",
				"replyText": "正在为您引导至人工队列，请稍候",
				"action": "transfer",
				"actionParam": "8002"
			}
		],
		"flowGraph": {
			"nodes": [
				{
					"id": "node-100",
					"type": "start",
					"label": "开始",
					"x": 120.5,
					"y": 150.0,
					"metadata": {
						"asrEnabled": true,
						"wsUrl": "ws://127.0.0.1:9002/stream"
					}
				}
			],
			"edges": []
		}
	}`)))
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	router.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save status got %d body %s", saveRec.Code, saveRec.Body.String())
	}

	pageReq := httptest.NewRequest(http.MethodPost, "/merchant/ai-model-flow/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d body %s", pageRec.Code, pageRec.Body.String())
	}

	publishReq := httptest.NewRequest(http.MethodPost, "/merchant/ai-model-flow/publish/1", nil)
	publishRec := httptest.NewRecorder()
	router.ServeHTTP(publishRec, publishReq)
	if publishRec.Code != http.StatusOK {
		t.Fatalf("publish status got %d body %s", publishRec.Code, publishRec.Body.String())
	}

	var result contracts.Result
	if err := json.NewDecoder(publishRec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeOK {
		t.Fatalf("unexpected result: %+v", result)
	}
}
