package operate

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

type callRecordStubRepository struct {
	page   operatedomain.CallRecordPageResult
	record operatedomain.CallRecord
	err    error
}

func (r *callRecordStubRepository) Page(_ context.Context, req operatedomain.CallRecordPageRequest) (operatedomain.CallRecordPageResult, error) {
	if r.err != nil {
		return operatedomain.CallRecordPageResult{}, r.err
	}
	r.page.PageNumber = req.PageNumber
	r.page.PageSize = req.PageSize
	return r.page, nil
}

func (r *callRecordStubRepository) GetByCallID(_ context.Context, _ string) (operatedomain.CallRecord, error) {
	if r.err != nil {
		return operatedomain.CallRecord{}, r.err
	}
	return r.record, nil
}

func TestCallRecordRoutes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.CallRecordManagementService{
		Repository: &callRecordStubRepository{
			page: operatedomain.CallRecordPageResult{
				Total: 1,
				Records: []operatedomain.CallRecord{{
					CallID:      "call-1",
					MerchantID:  8,
					UserID:      9,
					BatchTaskID: 10,
					CompletedAt: time.Now().UTC(),
				}},
			},
			record: operatedomain.CallRecord{CallID: "call-1", MerchantID: 8, UserID: 9},
		},
	}
	RegisterCallRecordRoutes(router, service)

	pageReq := httptest.NewRequest(http.MethodPost, "/merchant/call-record/page", bytes.NewReader([]byte(`{"pageNumber":1,"pageSize":10,"callId":"call"}`)))
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status got %d body %s", pageRec.Code, pageRec.Body.String())
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/merchant/call-record/detail/call-1", nil)
	detailRec := httptest.NewRecorder()
	router.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status got %d body %s", detailRec.Code, detailRec.Body.String())
	}

	var result contracts.Result
	if err := json.NewDecoder(detailRec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeOK {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCallRecordRoutesNotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.CallRecordManagementService{
		Repository: &callRecordStubRepository{err: operatedomain.ErrCallRecordNotFound},
	}
	RegisterCallRecordRoutes(router, service)

	req := httptest.NewRequest(http.MethodGet, "/merchant/call-record/detail/missing", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("detail status got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestCallRecordRoutesDetailRejectsBlankCallID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &operatedomain.CallRecordManagementService{Repository: &callRecordStubRepository{}}
	RegisterCallRecordRoutes(router, service)

	req := httptest.NewRequest(http.MethodGet, "/merchant/call-record/detail/%20", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("detail status got %d body %s", rec.Code, rec.Body.String())
	}
}

var _ operatedomain.CallRecordRepository = (*callRecordStubRepository)(nil)
