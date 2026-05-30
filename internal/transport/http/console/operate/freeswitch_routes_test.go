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
	fsregistry "yunshu/internal/infra/telephony"
)

func TestFreeSwitchRoutesSaveListAndDisable(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.FreeSwitchManagementService{Registry: fsregistry.NewMemoryRegistry()}
	RegisterFreeSwitchRoutes(router, service)

	body := []byte(`{"id":1,"fsAddr":"10.0.0.1:8021","password":"ClueCon","enable":true}`)
	saveReq := httptest.NewRequest(http.MethodPost, "/operate/freeswitch", bytes.NewReader(body))
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	router.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save status got %d body %s", saveRec.Code, saveRec.Body.String())
	}
	var saveResult contracts.Result
	if err := json.NewDecoder(saveRec.Body).Decode(&saveResult); err != nil {
		t.Fatal(err)
	}
	if saveResult.Code != contracts.CodeOK {
		t.Fatalf("unexpected save result: %+v", saveResult)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/operate/freeswitch/list", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status got %d", listRec.Code)
	}
	var listResult struct {
		Code int               `json:"code"`
		Data []fsregistry.Node `json:"data"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listResult); err != nil {
		t.Fatal(err)
	}
	if len(listResult.Data) != 1 || listResult.Data[0].FSAddr != "10.0.0.1:8021" {
		t.Fatalf("unexpected list result: %+v", listResult.Data)
	}

	disableReq := httptest.NewRequest(http.MethodPost, "/operate/freeswitch/1/disable", nil)
	disableRec := httptest.NewRecorder()
	router.ServeHTTP(disableRec, disableReq)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status got %d body %s", disableRec.Code, disableRec.Body.String())
	}
	node, err := service.Registry.GetByID(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if node.Enable {
		t.Fatalf("expected node disabled")
	}
}

func TestFreeSwitchRoutesRejectInvalidNode(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.FreeSwitchManagementService{Registry: fsregistry.NewMemoryRegistry()}
	RegisterFreeSwitchRoutes(router, service)

	req := httptest.NewRequest(http.MethodPost, "/operate/freeswitch", bytes.NewReader([]byte(`{"address":"10.0.0.1"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status got %d body %s", rec.Code, rec.Body.String())
	}
	var result contracts.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeBadRequest {
		t.Fatalf("unexpected result: %+v", result)
	}
}
