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

func TestExtensionRoutesAddPageAndToggle(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.ExtensionManagementService{Repository: resource.NewMemoryExtensionManagementRepository()}
	RegisterExtensionRoutes(router, service)

	body := []byte(`{"extensionNumber":"2002","password":"123456","merchantId":88,"userId":7,"enable":true}`)
	addReq := httptest.NewRequest(http.MethodPut, "/operate/extension/add", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()
	router.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status got %d body %s", addRec.Code, addRec.Body.String())
	}
	var addResult struct {
		Data operatedomain.Extension `json:"data"`
	}
	if err := json.NewDecoder(addRec.Body).Decode(&addResult); err != nil {
		t.Fatal(err)
	}
	if addResult.Data.ID == 0 {
		t.Fatalf("expected extension id")
	}

	pageReq := httptest.NewRequest(http.MethodPost, "/operate/extension/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10,"extensionNumber":"2002"}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d body %s", pageRec.Code, pageRec.Body.String())
	}
	var pageResult struct {
		Data struct {
			Total   int64                     `json:"total"`
			Records []operatedomain.Extension `json:"records"`
		} `json:"data"`
	}
	if err := json.NewDecoder(pageRec.Body).Decode(&pageResult); err != nil {
		t.Fatal(err)
	}
	if pageResult.Data.Total != 1 || len(pageResult.Data.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", pageResult.Data)
	}

	disableReq := httptest.NewRequest(http.MethodPost, "/operate/extension/disable/1", nil)
	disableRec := httptest.NewRecorder()
	router.ServeHTTP(disableRec, disableReq)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status got %d body %s", disableRec.Code, disableRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/operate/extension/delete", bytes.NewReader([]byte(`[{"id":1,"extensionNumber":"2002"}]`)))
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
