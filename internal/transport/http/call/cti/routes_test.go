package httpcti

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
	"yunshu/internal/domain/cti"
	"yunshu/internal/domain/esl"
	"yunshu/pkg/idempotency"
)

func TestSelectNumberRoute(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	RegisterRoutes(router, testAPICallService(), nil, nil, nil, nil, nil, nil, nil, nil, nil)

	body := []byte(`{"callId":"call-1","merchantId":"m1","candidates":[{"phone":"1001","gatewayId":"gw-1","available":true,"riskAllowed":true,"concurrency":1}]}`)
	req := httptest.NewRequest(http.MethodPost, "/cti/select-number", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d", rec.Code)
	}
	var result contracts.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeOK {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSelectNumberRouteBusinessFailure(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	RegisterRoutes(router, testAPICallService(), nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/cti/select-number", bytes.NewReader([]byte(`{"callId":"call-1","merchantId":"m1"}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d", rec.Code)
	}
	var result contracts.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeSelectionFailed {
		t.Fatalf("unexpected code: %+v", result)
	}
}

func TestCompatibleAPICallRoute(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	RegisterRoutes(router, testAPICallService(), nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/cti/callTask/call?callId=call-1", bytes.NewReader([]byte(`{"userId":7,"callee":"13800000000","extra":"{}"}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d", rec.Code)
	}
	var result contracts.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeOK {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCompatibleSelectRuleRoute(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	RegisterRoutes(router, testAPICallService(), nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/cti/select/number/rule?callId=call-1", bytes.NewReader([]byte(`{"merchantId":1,"callee":"13800000000","userId":7}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d", rec.Code)
	}
	var result contracts.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeOK {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCompatibleSelectRuleRouteUsesCandidateSource(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	RegisterRoutes(router, testAPICallService(), nil, nil, testCandidateSource{}, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/cti/select/number/rule?callId=call-1", bytes.NewReader([]byte(`{"merchantId":1,"callee":"13800000000","userId":7}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d", rec.Code)
	}
	var result struct {
		Code int                       `json:"code"`
		Data contracts.SelectPhoneResp `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Phone != "1001" || result.Data.GatewayName != "gw-test" || result.Data.GatewayRegion != "10.0.0.1:5060" {
		t.Fatalf("unexpected select response: %+v", result.Data)
	}
}

func TestSelectNumberRouteUsesCandidateSource(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	RegisterRoutes(router, testAPICallService(), nil, nil, testCandidateSource{}, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/cti/select-number", bytes.NewReader([]byte(`{"callId":"call-1","merchantId":"m1","userId":7}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d", rec.Code)
	}
	var result struct {
		Code int                 `json:"code"`
		Data cti.SelectionResult `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.Data.Success || result.Data.Caller == nil || result.Data.Caller.Phone != "1001" || result.Data.Caller.GatewayID != "9" {
		t.Fatalf("unexpected select response: %+v", result.Data)
	}
}

func TestBatchDispatchRoute(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	eslClient := &testESLClient{originate: &esl.OriginateService{CommandService: esl.NewCommandService(idempotency.NewMemoryStore(), &esl.MemoryCommandExecutor{}, nil)}}
	scheduler := &cti.BatchSchedulerService{
		Repository: testBatchRepository{},
		ESL:        eslClient,
		NewCallID:  func(cti.BatchTaskSnapshot, cti.BatchTelSnapshot) string { return "batch-call-1" },
	}
	RegisterRoutes(router, testAPICallService(), nil, scheduler, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/cti/batch-call-task/dispatch", bytes.NewReader([]byte(`{"taskId":10}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d body=%s", rec.Code, rec.Body.String())
	}
	var result contracts.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeOK {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestWebSocketRouteDisabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	RegisterRoutes(router, testAPICallService(), nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/cti/ws", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status got %d", rec.Code)
	}
}

func TestKamailioRegisterStatusWebhook(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	RegisterRoutes(router, testAPICallService(), nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// 1. 测试参数缺失
	body1 := []byte(`{"extension":"100001"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/cti/kamailio/subscriber/register-status", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec1.Code)
	}

	// 2. 测试 Redis 服务不可用
	body2 := []byte(`{"extension":"100001","event":"register"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/cti/kamailio/subscriber/register-status", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec2.Code)
	}
}

func TestKamailioCompatibilityAuthWebhooks(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	RegisterRoutes(router, testAPICallService(), nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// 1. Test /cti/kamailio/auth
	bodyAuth := []byte(`{"username":"1001","ip":"10.0.0.5"}`)
	reqAuth := httptest.NewRequest(http.MethodPost, "/cti/kamailio/auth", bytes.NewReader(bodyAuth))
	reqAuth.Header.Set("Content-Type", "application/json")
	recAuth := httptest.NewRecorder()
	router.ServeHTTP(recAuth, reqAuth)
	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recAuth.Code)
	}

	// 2. Test Redis unavailable for register
	bodyReg := []byte(`{"username":"1001","ip":"10.0.0.5","port":"5060","domain":"sip.yunshu.local"}`)
	reqReg := httptest.NewRequest(http.MethodPost, "/cti/kamailio/auth/register", bytes.NewReader(bodyReg))
	reqReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	router.ServeHTTP(recReg, reqReg)
	if recReg.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", recReg.Code)
	}

	// 3. Test Redis unavailable for unregister
	bodyUnreg := []byte(`{"username":"1001","ip":"10.0.0.5","port":"5060","domain":"sip.yunshu.local"}`)
	reqUnreg := httptest.NewRequest(http.MethodPost, "/cti/kamailio/auth/unregister", bytes.NewReader(bodyUnreg))
	reqUnreg.Header.Set("Content-Type", "application/json")
	recUnreg := httptest.NewRecorder()
	router.ServeHTTP(recUnreg, reqUnreg)
	if recUnreg.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", recUnreg.Code)
	}
}

func testAPICallService() *cti.APICallService {
	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	originate := &esl.OriginateService{CommandService: command}
	return &cti.APICallService{ESL: testESLClient{originate: originate}}
}

type testESLClient struct {
	originate *esl.OriginateService
}

func (c testESLClient) StartAPIOutbound(ctx context.Context, version, callID string, req contracts.ApiCallReq) error {
	return c.originate.StartAPIOutbound(ctx, esl.OriginateRequest{Version: version, CallID: callID, Request: req})
}

func (c testESLClient) StartBatchOutbound(ctx context.Context, version, callID string, req contracts.BatchCallReq) error {
	return c.originate.StartBatchOutbound(ctx, esl.BatchOriginateRequest{Version: version, CallID: callID, Request: req})
}

type testBatchRepository struct{}

func (testBatchRepository) GetRunnableBatchTask(context.Context, int) (cti.BatchTaskSnapshot, error) {
	return cti.BatchTaskSnapshot{ID: 10, MerchantID: 88, UserID: 7, ExtensionNumber: "1001"}, nil
}

func (testBatchRepository) ClaimNextPendingBatchTel(context.Context, int, time.Time) (cti.BatchTelSnapshot, error) {
	return cti.BatchTelSnapshot{ID: 20, TaskID: 10, MerchantID: 88, UserID: 7, Tel: "13800138000"}, nil
}

func (testBatchRepository) CompleteBatchTel(context.Context, int, int, bool, time.Time) error {
	return nil
}

func (testBatchRepository) ReleaseBatchTel(context.Context, int, int, time.Time) error {
	return nil
}

func (testBatchRepository) CompleteBatchTaskIfDrained(context.Context, int, time.Time) (bool, error) {
	return false, nil
}

func (testBatchRepository) GetIdleAgentFromSkillGroup(context.Context, int, int) (int, string, error) {
	return 0, "", nil
}

func (testBatchRepository) GetOnlineAgents(context.Context, int) ([]int, error) {
	return []int{1}, nil
}

func (testBatchRepository) GetActiveCallCount(context.Context, int) (int, error) {
	return 0, nil
}

func (testBatchRepository) GetAgentSkillGroups(context.Context, int) ([]int, error) {
	return []int{1}, nil
}

type testCandidateSource struct{}

func (testCandidateSource) CandidatesForUser(context.Context, int) ([]cti.NumberCandidate, error) {
	return []cti.NumberCandidate{{
		Phone:         "1001",
		GatewayID:     "9",
		GatewayName:   "gw-test",
		GatewayRegion: "10.0.0.1:5060",
		Concurrency:   1,
		Available:     true,
		RiskAllowed:   true,
	}}, nil
}
