package operate

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
	"yunshu/internal/infra/merchant"
)

func TestRateRoutesAddPageAndDelete(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.RateManagementService{Repository: merchant.NewMemoryRateRepository()}
	RegisterRateRoutes(router, service)

	body := []byte(`{"rateName":"标准费率","billingPrice":0.3,"billingCycle":60,"remark":"按分钟计费"}`)
	addReq := httptest.NewRequest(http.MethodPut, "/operate/rate/add", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()
	router.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status got %d body %s", addRec.Code, addRec.Body.String())
	}
	var addResult struct {
		Code int                `json:"code"`
		Data operatedomain.Rate `json:"data"`
	}
	if err := json.NewDecoder(addRec.Body).Decode(&addResult); err != nil {
		t.Fatal(err)
	}
	if addResult.Data.ID == 0 {
		t.Fatalf("expected rate id")
	}

	pageReq := httptest.NewRequest(http.MethodPost, "/operate/rate/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10,"name":"标准"}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d", pageRec.Code)
	}
	var pageResult struct {
		Code int `json:"code"`
		Data struct {
			Total   int64                `json:"total"`
			Records []operatedomain.Rate `json:"records"`
		} `json:"data"`
	}
	if err := json.NewDecoder(pageRec.Body).Decode(&pageResult); err != nil {
		t.Fatal(err)
	}
	if pageResult.Data.Total != 1 || len(pageResult.Data.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", pageResult.Data)
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/operate/rate/delete", bytes.NewReader([]byte(`[{"id":1}]`)))
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

func TestRateRoutesRejectReferencedDelete(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	repo := merchant.NewMemoryRateRepository()
	service := &operatedomain.RateManagementService{Repository: repo}
	RegisterRateRoutes(router, service)

	if _, err := repo.Save(context.Background(), operatedomain.Rate{RateName: "标准费率", BillingPrice: 0.3, BillingCycle: 60}); err != nil {
		t.Fatal(err)
	}
	repo.BindingsForTest(1)

	deleteReq := httptest.NewRequest(http.MethodPost, "/operate/rate/delete", bytes.NewReader([]byte(`[{"id":1}]`)))
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusConflict {
		t.Fatalf("delete status got %d body %s", deleteRec.Code, deleteRec.Body.String())
	}
}
