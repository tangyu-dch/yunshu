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
	"yunshu/internal/infra/business"
)

func TestBatchTaskRoutesAddPageDeleteAndToggle(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.BatchTaskManagementService{Repository: business.NewMemoryBatchTaskRepository()}
	RegisterBatchTaskRoutes(router, service)
	RegisterBatchDialpadRoutes(router, service)

	addReq := httptest.NewRequest(http.MethodPut, "/merchant/batch-call-task/add", bytes.NewReader([]byte(`{"merchantId":88,"userId":7,"name":"任务A","enable":true}`)))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()
	router.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status got %d body %s", addRec.Code, addRec.Body.String())
	}
	var addResult struct {
		Code int                     `json:"code"`
		Data operatedomain.BatchTask `json:"data"`
	}
	if err := json.NewDecoder(addRec.Body).Decode(&addResult); err != nil {
		t.Fatal(err)
	}
	if addResult.Data.ID == 0 {
		t.Fatalf("expected batch task id")
	}

	pageReq := httptest.NewRequest(http.MethodPost, "/merchant/batch-call-task/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10,"name":"任务"}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d body %s", pageRec.Code, pageRec.Body.String())
	}

	pauseReq := httptest.NewRequest(http.MethodPost, "/merchant/batch-call-dialpad/pause/1?reason=测试暂停", nil)
	pauseRec := httptest.NewRecorder()
	router.ServeHTTP(pauseRec, pauseReq)
	if pauseRec.Code != http.StatusOK {
		t.Fatalf("pause status got %d body %s", pauseRec.Code, pauseRec.Body.String())
	}

	enableReq := httptest.NewRequest(http.MethodPost, "/merchant/batch-call-task/enable/1", nil)
	enableRec := httptest.NewRecorder()
	router.ServeHTTP(enableRec, enableReq)
	if enableRec.Code != http.StatusOK {
		t.Fatalf("enable status got %d body %s", enableRec.Code, enableRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/merchant/batch-call-task/delete", bytes.NewReader([]byte(`[{"id":1}]`)))
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status got %d body %s", deleteRec.Code, deleteRec.Body.String())
	}
	var deleteResult contracts.Result
	if err := json.NewDecoder(deleteRec.Body).Decode(&deleteResult); err != nil {
		t.Fatal(err)
	}
	if deleteResult.Code != contracts.CodeOK {
		t.Fatalf("unexpected delete result: %+v", deleteResult)
	}
}
