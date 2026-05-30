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

func TestBillingRoutesOverviewRechargeAndRecords(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.BillingManagementService{Repository: merchant.NewMemoryBillingRepository()}
	RegisterBillingRoutes(router, service)

	saveBody := []byte(`{"merchantId":1,"paymentModeCode":1,"creditLimit":1000}`)
	saveReq := httptest.NewRequest(http.MethodPost, "/operate/billing/overview/save", bytes.NewReader(saveBody))
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	router.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save status got %d body %s", saveRec.Code, saveRec.Body.String())
	}

	rechargeBody := []byte(`{"merchantId":1,"amount":200,"remark":"首次充值","operator":9}`)
	rechargeReq := httptest.NewRequest(http.MethodPost, "/operate/billing/recharge", bytes.NewReader(rechargeBody))
	rechargeReq.Header.Set("Content-Type", "application/json")
	rechargeRec := httptest.NewRecorder()
	router.ServeHTTP(rechargeRec, rechargeReq)
	if rechargeRec.Code != http.StatusOK {
		t.Fatalf("recharge status got %d body %s", rechargeRec.Code, rechargeRec.Body.String())
	}

	pageReq := httptest.NewRequest(http.MethodPost, "/operate/billing/overview/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("overview page status got %d body %s", pageRec.Code, pageRec.Body.String())
	}

	recordReq := httptest.NewRequest(http.MethodPost, "/operate/billing/recharge-records", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10}`)))
	recordReq.Header.Set("Content-Type", "application/json")
	recordRec := httptest.NewRecorder()
	router.ServeHTTP(recordRec, recordReq)
	if recordRec.Code != http.StatusOK {
		t.Fatalf("record page status got %d body %s", recordRec.Code, recordRec.Body.String())
	}
	var result contracts.Result
	if err := json.NewDecoder(recordRec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeOK {
		t.Fatalf("unexpected result: %+v", result)
	}
}
