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

func TestChannelRoutesCRUD(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.ChannelManagementService{Repository: resource.NewMemoryChannelRepository()}
	RegisterChannelRoutes(router, service)

	// 1. Add channel (PUT)
	addBody := []byte(`{"name":"测试渠道1","remark":"运营渠道","enable":true}`)
	addReq := httptest.NewRequest(http.MethodPut, "/operate/channel/add", bytes.NewReader(addBody))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()
	router.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status got %d body %s", addRec.Code, addRec.Body.String())
	}

	var addRes contracts.Result
	if err := json.Unmarshal(addRec.Body.Bytes(), &addRes); err != nil {
		t.Fatal(err)
	}
	var savedChannel operatedomain.Channel
	dataBytes, _ := json.Marshal(addRes.Data)
	if err := json.Unmarshal(dataBytes, &savedChannel); err != nil {
		t.Fatal(err)
	}

	// 2. Page channel (POST)
	pageBody := []byte(`{"pageNumber":1,"pageSize":10,"name":"测试"}`)
	pageReq := httptest.NewRequest(http.MethodPost, "/operate/channel/page", bytes.NewReader(pageBody))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d body %s", pageRec.Code, pageRec.Body.String())
	}

	// 3. Detail channel (GET)
	detailReq := httptest.NewRequest(http.MethodGet, "/operate/channel/detail/1", nil)
	detailRec := httptest.NewRecorder()
	router.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status got %d body %s", detailRec.Code, detailRec.Body.String())
	}

	// 4. Update channel (POST)
	updateBody, _ := json.Marshal(operatedomain.Channel{
		ID:     savedChannel.ID,
		Name:   "测试渠道1-已修改",
		Enable: true,
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/operate/channel/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status got %d body %s", updateRec.Code, updateRec.Body.String())
	}

	// 5. Query parameter routing (GET)
	queryReq := httptest.NewRequest(http.MethodGet, "/operate/channel?name=测试&enable=true&pageNumber=1&pageSize=10", nil)
	queryRec := httptest.NewRecorder()
	router.ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusOK {
		t.Fatalf("query status got %d body %s", queryRec.Code, queryRec.Body.String())
	}

	// 6. Delete channel (POST)
	deleteBody, _ := json.Marshal([]operatedomain.Channel{savedChannel})
	deleteReq := httptest.NewRequest(http.MethodPost, "/operate/channel/delete", bytes.NewReader(deleteBody))
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status got %d body %s", deleteRec.Code, deleteRec.Body.String())
	}
}
