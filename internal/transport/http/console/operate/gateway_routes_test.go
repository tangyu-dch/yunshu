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
	"yunshu/internal/infra/telephony"
)

func TestGatewayRoutesAddPageAndDelete(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.GatewayManagementService{Repository: telephony.NewMemoryGatewayRepository()}
	RegisterGatewayRoutes(router, service)

	body := []byte(`{"name":"gw1","description":"网关一","channelId":1,"concurrency":10,"model":2,"realm":"10.0.0.1","port":"5060","priority":1,"supplementRing":false,"rateId":1,"enable":true,"gatewayCode":["PCMU"]}`)
	addReq := httptest.NewRequest(http.MethodPut, "/operate/gateway/add", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()
	router.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status got %d body %s", addRec.Code, addRec.Body.String())
	}
	var addResult struct {
		Code int `json:"code"`
		Data struct {
			Gateway operatedomain.Gateway `json:"gateway"`
		} `json:"data"`
	}
	if err := json.NewDecoder(addRec.Body).Decode(&addResult); err != nil {
		t.Fatal(err)
	}
	if addResult.Data.Gateway.ID == 0 {
		t.Fatalf("expected gateway id")
	}

	pageReq := httptest.NewRequest(http.MethodPost, "/operate/gateway/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10,"name":"gw"}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d", pageRec.Code)
	}
	var pageResult struct {
		Code int `json:"code"`
		Data struct {
			Total   int64                   `json:"total"`
			Records []operatedomain.Gateway `json:"records"`
		} `json:"data"`
	}
	if err := json.NewDecoder(pageRec.Body).Decode(&pageResult); err != nil {
		t.Fatal(err)
	}
	if pageResult.Data.Total != 1 || len(pageResult.Data.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", pageResult.Data)
	}

	syncReq := httptest.NewRequest(http.MethodPost, "/operate/gateway/sync/1", nil)
	syncRec := httptest.NewRecorder()
	router.ServeHTTP(syncRec, syncReq)
	if syncRec.Code != http.StatusOK {
		t.Fatalf("sync status got %d body %s", syncRec.Code, syncRec.Body.String())
	}
	var syncResult struct {
		Code int `json:"code"`
		Data struct {
			SyncAction   string `json:"syncAction"`
			SyncRequired bool   `json:"syncRequired"`
		} `json:"data"`
	}
	if err := json.NewDecoder(syncRec.Body).Decode(&syncResult); err != nil {
		t.Fatal(err)
	}
	if syncResult.Data.SyncAction != "sync" || !syncResult.Data.SyncRequired {
		t.Fatalf("unexpected sync result: %+v", syncResult.Data)
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/operate/gateway/delete", bytes.NewReader([]byte(`[{"id":1,"name":"gw1"}]`)))
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

func TestGatewayRoutesRejectConflict(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.GatewayManagementService{Repository: telephony.NewMemoryGatewayRepository()}
	RegisterGatewayRoutes(router, service)

	body := []byte(`{"name":"gw1","description":"网关一","channelId":1,"concurrency":10,"model":2,"realm":"10.0.0.1","port":"5060","priority":1,"supplementRing":false,"rateId":1,"enable":true}`)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPut, "/operate/gateway/add", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if i == 0 && rec.Code != http.StatusOK {
			t.Fatalf("first add status got %d", rec.Code)
		}
		if i == 1 && rec.Code != http.StatusConflict {
			t.Fatalf("second add status got %d body %s", rec.Code, rec.Body.String())
		}
	}
}
