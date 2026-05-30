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
	"yunshu/internal/infra/security"
)

func TestBlacklistRoutesAddPageAndDelete(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.BlacklistManagementService{Repository: security.NewMemoryBlacklistRepository()}
	RegisterBlacklistRoutes(router, service)

	body := []byte(`{"name":"投诉黑名单","verificationChannel":1,"gatewayIds":[1,2],"remark":"重点拦截"}`)
	addReq := httptest.NewRequest(http.MethodPut, "/operate/blacklist/add", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()
	router.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status got %d body %s", addRec.Code, addRec.Body.String())
	}
	var addResult struct {
		Code int                     `json:"code"`
		Data operatedomain.Blacklist `json:"data"`
	}
	if err := json.NewDecoder(addRec.Body).Decode(&addResult); err != nil {
		t.Fatal(err)
	}
	if addResult.Data.ID == 0 {
		t.Fatalf("expected blacklist id")
	}

	pageReq := httptest.NewRequest(http.MethodPost, "/operate/blacklist/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10,"name":"投诉"}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d", pageRec.Code)
	}
	var pageResult struct {
		Code int `json:"code"`
		Data struct {
			Total   int64                     `json:"total"`
			Records []operatedomain.Blacklist `json:"records"`
		} `json:"data"`
	}
	if err := json.NewDecoder(pageRec.Body).Decode(&pageResult); err != nil {
		t.Fatal(err)
	}
	if pageResult.Data.Total != 1 || len(pageResult.Data.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", pageResult.Data)
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/operate/blacklist/delete/1", nil)
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

func TestBlacklistRoutesRejectConflict(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.BlacklistManagementService{Repository: security.NewMemoryBlacklistRepository()}
	RegisterBlacklistRoutes(router, service)

	body := []byte(`{"name":"投诉黑名单","verificationChannel":1}`)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPut, "/operate/blacklist/add", bytes.NewReader(body))
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
