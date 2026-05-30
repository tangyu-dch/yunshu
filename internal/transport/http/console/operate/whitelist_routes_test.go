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

func TestWhitelistRoutesAddPageDetailUpdateDelete(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.WhitelistManagementService{Repository: security.NewMemoryWhitelistRepository()}
	RegisterWhitelistRoutes(router, service)

	addBody := []byte(`{"phones":["13800000000","13800000001"],"numberType":"CALLER","merchantIds":[1,2]}`)
	addReq := httptest.NewRequest(http.MethodPut, "/operate/whitelist/add", bytes.NewReader(addBody))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()
	router.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status got %d body %s", addRec.Code, addRec.Body.String())
	}

	pageReq := httptest.NewRequest(http.MethodPost, "/operate/whitelist/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10,"number":"138"}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d body %s", pageRec.Code, pageRec.Body.String())
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/operate/whitelist/detail/1", nil)
	detailRec := httptest.NewRecorder()
	router.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status got %d body %s", detailRec.Code, detailRec.Body.String())
	}
	var detailResult struct {
		Code int                           `json:"code"`
		Data operatedomain.WhitelistRecord `json:"data"`
	}
	if err := json.NewDecoder(detailRec.Body).Decode(&detailResult); err != nil {
		t.Fatal(err)
	}
	if detailResult.Data.ID != 1 {
		t.Fatalf("unexpected detail: %+v", detailResult.Data)
	}

	updateBody := []byte(`{"id":1,"numberType":"CALLEE","merchantIds":[3]}`)
	updateReq := httptest.NewRequest(http.MethodPost, "/operate/whitelist/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status got %d body %s", updateRec.Code, updateRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/operate/whitelist/delete?whiteIds=1,2", nil)
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
