package httpesl

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	"yunshu/internal/infra/business"
	"yunshu/pkg/idempotency"
)

func TestValidateControlRoute(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	originate, command, session, _ := testESLRuntime()
	RegisterRoutes(router, originate, command, session, testGatewaySync(), nil, nil, nil)

	body := []byte(`{"commandId":"cmd-1","command":"hangup","callId":"call-1","uuid":"uuid-1","fsAddr":"10.0.0.1:8021","legRole":"customer","profile":"api_outbound"}`)
	req := httptest.NewRequest(http.MethodPost, "/esl/control/validate", bytes.NewReader(body))
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

func TestLifecycleApplyRouteRejectsInvalidTransition(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	originate, command, session, _ := testESLRuntime()
	RegisterRoutes(router, originate, command, session, testGatewaySync(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/esl/lifecycle/apply", bytes.NewReader([]byte(`{"initial":"new","event":"CHANNEL_BRIDGE"}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status got %d", rec.Code)
	}
	var result contracts.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeConflict {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCompatibleEslStartRoute(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	originate, command, session, executor := testESLRuntime()
	RegisterRoutes(router, originate, command, session, testGatewaySync(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/esl/call/start?callId=call-1", bytes.NewReader([]byte(`{"userId":7,"callee":"13800000000","extra":"{}"}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d", rec.Code)
	}
	if executor.Count() != 1 {
		t.Fatalf("expected one command, got %d", executor.Count())
	}
}

func TestCompatibleControlHangupRoute(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	originate, command, session, executor := testESLRuntime()
	RegisterRoutes(router, originate, command, session, testGatewaySync(), nil, nil, nil)

	body := []byte(`{"callId":"call-1","uuid":"uuid-1","fsAddr":"10.0.0.1:8021","legRole":"customer","reasonCode":"NORMAL_CLEARING"}`)
	req := httptest.NewRequest(http.MethodPost, "/esl/control/hangup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d", rec.Code)
	}
	if executor.Count() != 1 {
		t.Fatalf("expected one command, got %d", executor.Count())
	}
	if got := executor.Commands[0].Payload["reasonCode"]; got != "NORMAL_CLEARING" {
		t.Fatalf("expected reason code in payload, got %v", got)
	}
}

func TestCompatibleBatchStartRoute(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	originate, command, session, executor := testESLRuntime()
	RegisterRoutes(router, originate, command, session, testGatewaySync(), nil, nil, nil)

	body := []byte(`{"userId":7,"batchTaskId":10,"batchCallTelId":20,"phone":"13800138000","merchantId":88,"extension":"1001","extra":"{\"gatewayName\":\"gw-sh\"}"}`)
	req := httptest.NewRequest(http.MethodPost, "/esl/batch/call/start?callId=call-batch-1", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d body=%s", rec.Code, rec.Body.String())
	}
	if executor.Count() != 1 {
		t.Fatalf("expected one command, got %d", executor.Count())
	}
	if got := executor.Commands[0].Profile; got != contracts.CallFlowBatchOutbound {
		t.Fatalf("unexpected profile: %s", got)
	}
}

func TestApplyFSEventRouteCompletesSession(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	originate, command, session, _ := testESLRuntime()
	RegisterRoutes(router, originate, command, session, testGatewaySync(), nil, nil, nil)

	start := httptest.NewRequest(http.MethodPost, "/esl/call/start?callId=call-1", bytes.NewReader([]byte(`{"userId":7,"callee":"13800000000","extra":"{}"}`)))
	router.ServeHTTP(httptest.NewRecorder(), start)

	create := httptest.NewRequest(http.MethodPost, "/esl/events/apply", bytes.NewReader([]byte(`{"eventId":"evt-1","eventName":"CHANNEL_CREATE","callId":"call-1","uuid":"uuid-1","fsAddr":"10.0.0.1:8021","legRole":"customer","profile":"api_outbound"}`)))
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, create)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status got %d", createRec.Code)
	}

	complete := httptest.NewRequest(http.MethodPost, "/esl/events/apply", bytes.NewReader([]byte(`{"eventId":"evt-2","eventName":"CHANNEL_HANGUP_COMPLETE","callId":"call-1","uuid":"uuid-1","fsAddr":"10.0.0.1:8021","legRole":"customer","profile":"api_outbound","headers":{"hangupCause":"NORMAL_CLEARING"}}`)))
	completeRec := httptest.NewRecorder()
	router.ServeHTTP(completeRec, complete)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("complete status got %d body=%s", completeRec.Code, completeRec.Body.String())
	}
}

func TestGatewaySyncRouteCreatesTargets(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	originate, command, session, _ := testESLRuntime()
	RegisterRoutes(router, originate, command, session, testGatewaySync(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/esl/gateway?gatewayId=7", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			GatewayName string `json:"gatewayName"`
			TargetCount int    `json:"targetCount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeOK || result.Data.GatewayName != "gw-test" || result.Data.TargetCount != 1 {
		t.Fatalf("unexpected gateway sync result: %+v", result)
	}
}

func testESLRuntime() (*esl.OriginateService, *esl.CommandService, *esl.SessionService, *esl.MemoryCommandExecutor) {
	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	originate := &esl.OriginateService{CommandService: command, SessionService: session}
	return originate, command, session, executor
}

func testGatewaySync() *esl.GatewayConfigService {
	return &esl.GatewayConfigService{Gateways: fakeGatewayNameResolver{}, Nodes: fakeGatewaySyncNodeLister{}}
}

type fakeGatewayNameResolver struct{}

func (fakeGatewayNameResolver) GetGatewayNameByID(_ context.Context, id int) (string, error) {
	if id == 7 {
		return "gw-test", nil
	}
	return "", esl.ErrGatewayConfigNotFound
}

type fakeGatewaySyncNodeLister struct{}

func (fakeGatewaySyncNodeLister) ListGatewaySyncNodes(context.Context) ([]esl.GatewaySyncNode, error) {
	return []esl.GatewaySyncNode{{ID: 1, FSAddr: "10.0.0.1:8021", CommandURL: "10.0.0.1:8080"}}, nil
}

func TestControlRouteWithAuthAndAuthBypass(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.ReleaseMode)

	// 1. 初始化内存数据库并进行迁移
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	// 迁移商户表
	type MerchantModel struct {
		ID        int    `gorm:"primaryKey"`
		AppKey    string `gorm:"column:app_key"`
		AppSecret string `gorm:"column:app_secret"`
		Enable    bool   `gorm:"column:enable"`
		DelFlag   bool   `gorm:"column:del_flag"`
	}
	db.Table("cc_mch_info").AutoMigrate(&MerchantModel{})

	// 插入一个测试商户，ID=88
	db.Table("cc_mch_info").Create(&MerchantModel{
		ID:        88,
		AppKey:    "test-key-88",
		AppSecret: "test-secret-88",
		Enable:    true,
		DelFlag:   false,
	})
	// 插入另一个商户，ID=99
	db.Table("cc_mch_info").Create(&MerchantModel{
		ID:        99,
		AppKey:    "test-key-99",
		AppSecret: "test-secret-99",
		Enable:    true,
		DelFlag:   false,
	})

	router := gin.New()
	originate, command, session, _ := testESLRuntime()
	RegisterRoutes(router, originate, command, session, testGatewaySync(), nil, nil, db)

	// 先在 session.Store 中创建一个属于商户 88 的会话
	session.Store.Save(context.Background(), esl.CallSession{
		CallID: "call-1",
		Metadata: map[string]any{
			"merchantId": 88,
		},
	})

	// 情景 A: 没有 App Credentials，直接放行控制（确保对内部/老版本兼容）
	{
		body := []byte(`{"callId":"call-1","uuid":"uuid-1","fsAddr":"10.0.0.1:8021"}`)
		req := httptest.NewRequest(http.MethodPost, "/esl/control/hangup", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("Bypass auth: expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	}

	// 情景 B: 带上商户 88 的合法凭证控制属于 88 的通话 -> 允许
	{
		body := []byte(`{"callId":"call-1","uuid":"uuid-1","fsAddr":"10.0.0.1:8021"}`)
		req := httptest.NewRequest(http.MethodPost, "/esl/control/hangup", bytes.NewReader(body))
		req.Header.Set("X-App-Key", "test-key-88")
		req.Header.Set("X-App-Secret", "test-secret-88")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("Authorized merchant control: expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	}

	// 情景 C: 带上商户 99 的合法凭证去控制商户 88 的通话 -> 越权拦截 403
	{
		body := []byte(`{"callId":"call-1","uuid":"uuid-1","fsAddr":"10.0.0.1:8021"}`)
		req := httptest.NewRequest(http.MethodPost, "/esl/control/hangup", bytes.NewReader(body))
		req.Header.Set("X-App-Key", "test-key-99")
		req.Header.Set("X-App-Secret", "test-secret-99")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("Horizontal authorization bypass: expected 403, got %d", rec.Code)
		}
	}
}
