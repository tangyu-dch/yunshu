package operate

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	operatedomain "yunshu/internal/domain/operate"
	"yunshu/internal/infra/telephony"
)

func TestDispatcherRoutesAddPageAndDelete(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	service := &operatedomain.DispatcherManagementService{Repository: telephony.NewMemoryDispatcherRepository()}
	RegisterDispatcherRoutes(router, service)

	// 1. 测试新增 Dispatcher 节点
	body, _ := json.Marshal(operatedomain.Dispatcher{
		SetID:       1,
		Destination: "sip:127.0.0.1:5060",
		Description: "Test Dispatcher Node",
		Enable:      true,
	})
	addReq := httptest.NewRequest(http.MethodPut, "/operate/kamailio/dispatcher/add", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addResp := httptest.NewRecorder()

	router.ServeHTTP(addResp, addReq)
	if addResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", addResp.Code, addResp.Body.String())
	}

	var addResult struct {
		Code int `json:"code"`
		Data struct {
			Dispatcher operatedomain.Dispatcher `json:"dispatcher"`
		} `json:"data"`
	}
	_ = json.Unmarshal(addResp.Body.Bytes(), &addResult)
	if addResult.Data.Dispatcher.ID == 0 {
		t.Fatalf("expected dispatcher id")
	}

	// 2. 测试分页查询
	pageReq := httptest.NewRequest(http.MethodPost, "/operate/kamailio/dispatcher/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageResp := httptest.NewRecorder()

	router.ServeHTTP(pageResp, pageReq)
	if pageResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", pageResp.Code)
	}

	var pageResult struct {
		Code int `json:"code"`
		Data struct {
			Records []operatedomain.Dispatcher `json:"records"`
			Total   int64                      `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(pageResp.Body.Bytes(), &pageResult)
	if pageResult.Data.Total != 1 {
		t.Fatalf("expected 1 record, got %d", pageResult.Data.Total)
	}

	// 3. 测试批量逻辑删除
	deleteBody, _ := json.Marshal([]operatedomain.Dispatcher{
		{ID: addResult.Data.Dispatcher.ID, Destination: "sip:127.0.0.1:5060"},
	})
	deleteReq := httptest.NewRequest(http.MethodPost, "/operate/kamailio/dispatcher/delete", bytes.NewReader(deleteBody))
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteResp := httptest.NewRecorder()

	router.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", deleteResp.Code)
	}
}

func TestDispatcherRoutesRejectConflict(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	service := &operatedomain.DispatcherManagementService{Repository: telephony.NewMemoryDispatcherRepository()}
	RegisterDispatcherRoutes(router, service)

	// 先添加一个节点
	body, _ := json.Marshal(operatedomain.Dispatcher{
		SetID:       1,
		Destination: "sip:127.0.0.1:5060",
		Description: "Test Dispatcher Node",
		Enable:      true,
	})
	req := httptest.NewRequest(http.MethodPut, "/operate/kamailio/dispatcher/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// 重复添加相同的 destination，应当返回 409 Conflict
	resp2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPut, "/operate/kamailio/dispatcher/add", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp2.Code)
	}
}
