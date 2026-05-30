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
	"yunshu/internal/infra/merchant"
)

func TestMerchantRoutesAddPageAndDelete(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.MerchantManagementService{Repository: merchant.NewMemoryMerchantRepository()}
	RegisterMerchantRoutes(router, service)

	body := []byte(`{"name":"商户A","account":"merchant-a","rateId":0,"whitelistDomains":"example.com,192.168.1.1","enable":true}`)
	addReq := httptest.NewRequest(http.MethodPut, "/operate/merchant/add", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()
	router.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status got %d body %s", addRec.Code, addRec.Body.String())
	}
	var addResult struct {
		Code int `json:"code"`
		Data struct {
			Merchant operatedomain.Merchant `json:"merchant"`
		} `json:"data"`
	}
	if err := json.NewDecoder(addRec.Body).Decode(&addResult); err != nil {
		t.Fatal(err)
	}
	if addResult.Data.Merchant.ID == 0 {
		t.Fatalf("expected merchant id")
	}
	if addResult.Data.Merchant.WhitelistDomains != "example.com,192.168.1.1" {
		t.Fatalf("unexpected whitelist domains: %s", addResult.Data.Merchant.WhitelistDomains)
	}

	pageReq := httptest.NewRequest(http.MethodPost, "/operate/merchant/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10,"name":"商户"}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d", pageRec.Code)
	}
	var pageResult struct {
		Code int `json:"code"`
		Data struct {
			Total   int64                    `json:"total"`
			Records []operatedomain.Merchant `json:"records"`
		} `json:"data"`
	}
	if err := json.NewDecoder(pageRec.Body).Decode(&pageResult); err != nil {
		t.Fatal(err)
	}
	if pageResult.Data.Total != 1 || len(pageResult.Data.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", pageResult.Data)
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/operate/merchant/delete", bytes.NewReader([]byte(`[{"id":1,"name":"商户A"}]`)))
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

func TestMerchantRoutesRejectConflict(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.MerchantManagementService{Repository: merchant.NewMemoryMerchantRepository()}
	RegisterMerchantRoutes(router, service)

	body := []byte(`{"name":"商户A","account":"merchant-a"}`)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPut, "/operate/merchant/add", bytes.NewReader(body))
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
