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
	"yunshu/internal/infra/resource"
)

func TestPhoneGroupRoutesAddPageAndBind(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.PhoneGroupManagementService{Repository: resource.NewMemoryPhoneGroupRepository()}
	RegisterPhoneGroupRoutes(router, service)

	body := []byte(`{"name":"号码组A","remark":"r","desc":"d","merchantId":88,"enable":true}`)
	addReq := httptest.NewRequest(http.MethodPut, "/merchant/phone-group/add", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()
	router.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status got %d body %s", addRec.Code, addRec.Body.String())
	}
	var addResult struct {
		Data operatedomain.PhoneGroup `json:"data"`
	}
	if err := json.NewDecoder(addRec.Body).Decode(&addResult); err != nil {
		t.Fatal(err)
	}
	if addResult.Data.ID == 0 {
		t.Fatalf("expected phone group id")
	}

	pageReq := httptest.NewRequest(http.MethodPost, "/merchant/phone-group/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10,"merchantId":88,"name":"号码组"}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d body %s", pageRec.Code, pageRec.Body.String())
	}
	var pageResult struct {
		Data struct {
			Total   int64                      `json:"total"`
			Records []operatedomain.PhoneGroup `json:"records"`
		} `json:"data"`
	}
	if err := json.NewDecoder(pageRec.Body).Decode(&pageResult); err != nil {
		t.Fatal(err)
	}
	if pageResult.Data.Total != 1 || len(pageResult.Data.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", pageResult.Data)
	}

	bindReq := httptest.NewRequest(http.MethodPost, "/merchant/phone-group/phones/1", bytes.NewReader([]byte(`{"merchantId":88,"phoneIds":[10,11]}`)))
	bindReq.Header.Set("Content-Type", "application/json")
	bindRec := httptest.NewRecorder()
	router.ServeHTTP(bindRec, bindReq)
	if bindRec.Code != http.StatusOK {
		t.Fatalf("bind status got %d body %s", bindRec.Code, bindRec.Body.String())
	}

	queryReq := httptest.NewRequest(http.MethodGet, "/merchant/phone-group/phones/1", nil)
	queryRec := httptest.NewRecorder()
	router.ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusOK {
		t.Fatalf("query bind status got %d body %s", queryRec.Code, queryRec.Body.String())
	}
	var queryResult struct {
		Data struct {
			PhoneIDs []int `json:"phoneIds"`
		} `json:"data"`
	}
	if err := json.NewDecoder(queryRec.Body).Decode(&queryResult); err != nil {
		t.Fatal(err)
	}
	if len(queryResult.Data.PhoneIDs) != 2 {
		t.Fatalf("unexpected phone ids: %+v", queryResult.Data.PhoneIDs)
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/merchant/phone-group/delete", bytes.NewReader([]byte(`[{"id":1,"name":"号码组A"}]`)))
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
