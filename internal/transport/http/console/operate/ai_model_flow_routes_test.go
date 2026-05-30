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

	saveReq := httptest.NewRequest(http.MethodPut, "/merchant/ai-model-flow/add", bytes.NewReader([]byte(`{"name":"流程A","prompt":"请处理呼叫"}`)))
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
