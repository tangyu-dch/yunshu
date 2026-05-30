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

func TestRtpengineRoutesAddPageAndDelete(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	service := &operatedomain.RtpengineManagementService{Repository: telephony.NewMemoryRtpengineRepository()}
	RegisterRtpengineRoutes(router, service)

	// 1. 测试新增 RTPEngine 节点
	body, _ := json.Marshal(operatedomain.Rtpengine{
		SetID:         1,
		RtpengineSock: "udp:127.0.0.1:2223",
		Description:   "Test RTPEngine Proxy",
	})
	addReq := httptest.NewRequest(http.MethodPut, "/operate/kamailio/rtpengine/add", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addResp := httptest.NewRecorder()

	router.ServeHTTP(addResp, addReq)
	if addResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", addResp.Code)
	}

	var addResult struct {
		Code int `json:"code"`
		Data struct {
			Rtpengine operatedomain.Rtpengine `json:"rtpengine"`
		} `json:"data"`
	}
	_ = json.Unmarshal(addResp.Body.Bytes(), &addResult)
	if addResult.Data.Rtpengine.ID == 0 {
		t.Fatalf("expected rtpengine id")
	}

	// 2. 测试分页查询
	pageReq := httptest.NewRequest(http.MethodPost, "/operate/kamailio/rtpengine/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10,"rtpengineSock":"127.0.0.1"}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageResp := httptest.NewRecorder()

	router.ServeHTTP(pageResp, pageReq)
	if pageResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", pageResp.Code)
	}

	var pageResult struct {
		Code int `json:"code"`
		Data struct {
			Records []operatedomain.Rtpengine `json:"records"`
			Total   int64                     `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(pageResp.Body.Bytes(), &pageResult)
	if pageResult.Data.Total != 1 {
		t.Fatalf("expected 1 record, got %d", pageResult.Data.Total)
	}

	// 3. 测试批量逻辑删除
	deleteBody, _ := json.Marshal([]operatedomain.Rtpengine{
		{ID: addResult.Data.Rtpengine.ID, RtpengineSock: "udp:127.0.0.1:2223"},
	})
	deleteReq := httptest.NewRequest(http.MethodPost, "/operate/kamailio/rtpengine/delete", bytes.NewReader(deleteBody))
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteResp := httptest.NewRecorder()

	router.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", deleteResp.Code)
	}
}

func TestRtpengineRoutesRejectConflict(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	service := &operatedomain.RtpengineManagementService{Repository: telephony.NewMemoryRtpengineRepository()}
	RegisterRtpengineRoutes(router, service)

	// 先添加一个节点
	body, _ := json.Marshal(operatedomain.Rtpengine{
		SetID:         1,
		RtpengineSock: "udp:127.0.0.1:2223",
		Description:   "Test RTPEngine Proxy",
	})
	req := httptest.NewRequest(http.MethodPut, "/operate/kamailio/rtpengine/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// 重复添加相同的 socket，应当返回 409 Conflict
	resp2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPut, "/operate/kamailio/rtpengine/add", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp2.Code)
	}
}
