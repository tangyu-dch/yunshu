package operate

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// 用于生成合法测试证书的 EC 私钥（与内置 ECDSA 公钥配对）
const testPrivKeyPEMForRoutes = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIOSrSXQ3ij2D+L6bEaQLpnuU0gmzKo/0N/vKQ/5g/PhpoAoGCCqGSM49
AwEHoUQDQgAEb2UlRHVtw6m/Fh80w79TCs7dJ42ALCzo4S32ZeXmrbatBefo8yrb
GQTN8Ta02MrMW80krcCHe3/hZBDRQMXraw==
-----END EC PRIVATE KEY-----`

// mockProxyConfigRepo 用作 HTTP 测试中 Mock 仓储
type testMockProxyConfigRepo struct {
	configs map[string]string
}

func newTestMockProxyConfigRepo() *testMockProxyConfigRepo {
	return &testMockProxyConfigRepo{configs: make(map[string]string)}
}

func (m *testMockProxyConfigRepo) Get(ctx context.Context, key string) (operatedomain.ProxyConfigItem, error) {
	val, ok := m.configs[key]
	if !ok {
		return operatedomain.ProxyConfigItem{}, operatedomain.ErrConfigNotFound
	}
	return operatedomain.ProxyConfigItem{
		Key:         key,
		Value:       val,
		UpdatedTime: time.Now(),
	}, nil
}

func (m *testMockProxyConfigRepo) Set(ctx context.Context, key, value, desc string) error {
	m.configs[key] = value
	return nil
}

func (m *testMockProxyConfigRepo) List(ctx context.Context) ([]operatedomain.ProxyConfigItem, error) {
	var list []operatedomain.ProxyConfigItem
	for k, v := range m.configs {
		list = append(list, operatedomain.ProxyConfigItem{Key: k, Value: v})
	}
	return list, nil
}

func (m *testMockProxyConfigRepo) EnsureDefaults(ctx context.Context) error {
	return nil
}

// 辅助函数：使用私钥为 Claims 签名并返回 License 二进制字节
func generateTestLicenseForRoutes(claims operatedomain.LicenseClaims) ([]byte, error) {
	block, _ := pem.Decode([]byte(testPrivKeyPEMForRoutes))
	if block == nil {
		return nil, errors.New("failed to decode private key PEM")
	}
	privKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	payloadRaw, err := json.Marshal(claims)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(payloadRaw)
	sig, err := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	if err != nil {
		return nil, err
	}

	licData := operatedomain.LicenseData{
		PayloadRaw: payloadRaw,
		Signature:  sig,
	}

	return json.Marshal(licData)
}

func createMultipartRequest(url string, filename string, content []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	_, err = part.Write(content)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

// 测试场景一：基础流程与租户模式修改
func TestLicenseRoutesWorkflow(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tmpLicPath := "test_yunshu_routes_workflow.lic"
	defer os.Remove(tmpLicPath)

	repo := newTestMockProxyConfigRepo()
	service := operatedomain.NewLicenseService(repo, tmpLicPath, nil)

	router := gin.New()
	RegisterLicenseRoutes(router, service)

	// 1. 测试指纹获取 GET /operate/license/fingerprint
	req1 := httptest.NewRequest(http.MethodGet, "/operate/license/fingerprint", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("get fingerprint status got %d, body: %s", rec1.Code, rec1.Body.String())
	}
	var res1 struct {
		Code int `json:"code"`
		Data struct {
			DeploymentID string `json:"deploymentId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec1.Body.Bytes(), &res1); err != nil {
		t.Fatal(err)
	}
	if res1.Data.DeploymentID == "" {
		t.Fatalf("deploymentId is empty, got: %+v", res1)
	}

	// 2. 测试获取授权状态 GET /operate/license/status (初始状态由于 15 天试用豁免，为 grace_period，模式默认为 single)
	req2 := httptest.NewRequest(http.MethodGet, "/operate/license/status", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("get license status code got %d, body: %s", rec2.Code, rec2.Body.String())
	}
	var res2 struct {
		Code int                                 `json:"code"`
		Data operatedomain.LicenseStatusResponse `json:"data"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &res2); err != nil {
		t.Fatal(err)
	}
	if res2.Data.Status != "grace_period" {
		t.Fatalf("expected grace_period trial state, got %s", res2.Data.Status)
	}
	if res2.Data.TenantMode != "single" {
		t.Fatalf("expected default tenantMode single, got %s", res2.Data.TenantMode)
	}

	// 3. 测试设置租户模式为多租户 POST /operate/license/tenant-mode (成功)
	reqBody3 := `{"mode":"multi"}`
	req3 := httptest.NewRequest(http.MethodPost, "/operate/license/tenant-mode", bytes.NewReader([]byte(reqBody3)))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("post tenant mode got status %d, body: %s", rec3.Code, rec3.Body.String())
	}

	// 4. 再次获取授权状态 GET /operate/license/status，校验 tenantMode 变更为 multi
	req4 := httptest.NewRequest(http.MethodGet, "/operate/license/status", nil)
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, req4)
	var res4 struct {
		Code int                                 `json:"code"`
		Data operatedomain.LicenseStatusResponse `json:"data"`
	}
	if err := json.Unmarshal(rec4.Body.Bytes(), &res4); err != nil {
		t.Fatal(err)
	}
	if res4.Data.TenantMode != "multi" {
		t.Fatalf("expected tenantMode to be multi, got %s", res4.Data.TenantMode)
	}
}

// 测试场景二：证书上传激活（正常和非正常情况）
func TestLicenseRoutesUploadVerification(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tmpLicPath := "test_yunshu_routes_upload.lic"
	defer os.Remove(tmpLicPath)

	repo := newTestMockProxyConfigRepo()
	service := operatedomain.NewLicenseService(repo, tmpLicPath, nil)

	router := gin.New()
	RegisterLicenseRoutes(router, service)

	// 1. 上传损坏/不合法的证书 (激活失败，应返回 400)
	reqErr, err := createMultipartRequest("/operate/license/upload", "yunshu.lic", []byte("corrupted_license_data"))
	if err != nil {
		t.Fatal(err)
	}
	recErr := httptest.NewRecorder()
	router.ServeHTTP(recErr, reqErr)
	if recErr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for corrupted license, got %d, body: %s", recErr.Code, recErr.Body.String())
	}

	// 2. 模拟生成合法的测试证书并上传 (激活成功，应返回 200)
	depID, err := service.GetDeploymentID()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	claims := operatedomain.LicenseClaims{
		LicenseID:    "LIC-ROUTE-MOCK",
		CustomerName: "路由单元测试客户",
		MerchantID:   "1001",
		DeploymentID: depID,
		IssuedAt:     now,
		NotBefore:    now - 100,
		NotAfter:     now + 3600*24*10,
		Limits: operatedomain.LicenseLimits{
			MaxConcurrentCalls: 100,
			MaxExtensions:      200,
		},
	}
	validLicBytes, err := generateTestLicenseForRoutes(claims)
	if err != nil {
		t.Fatal(err)
	}

	reqOk, err := createMultipartRequest("/operate/license/upload", "yunshu.lic", validLicBytes)
	if err != nil {
		t.Fatal(err)
	}
	recOk := httptest.NewRecorder()
	router.ServeHTTP(recOk, reqOk)
	if recOk.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid license, got %d, body: %s", recOk.Code, recOk.Body.String())
	}

	// 3. 再次获取状态，校验是否已变更为 normal 状态并加载了相应限制
	reqStatus := httptest.NewRequest(http.MethodGet, "/operate/license/status", nil)
	recStatus := httptest.NewRecorder()
	router.ServeHTTP(recStatus, reqStatus)
	var res struct {
		Code int                                 `json:"code"`
		Data operatedomain.LicenseStatusResponse `json:"data"`
	}
	if err := json.Unmarshal(recStatus.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Data.Status != "normal" || res.Data.CustomerName != "路由单元测试客户" {
		t.Fatalf("expected normal state with custom customerName, got status: %s, name: %s", res.Data.Status, res.Data.CustomerName)
	}
}

// 测试场景三：模拟 server.go 里的 consoleAccessMiddleware 旁路安全豁免与阻断
func TestLicenseRoutesSecurityAndBypass(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tmpLicPath := "test_yunshu_routes_security.lic"
	defer os.Remove(tmpLicPath)

	repo := newTestMockProxyConfigRepo()
	service := operatedomain.NewLicenseService(repo, tmpLicPath, nil)

	// 定义模拟权限过滤中间件，完全模拟 server.go 的豁免设计
	mockBypassMiddleware := func(isInternal bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			path := c.Request.URL.Path
			// 路由豁免
			if strings.HasPrefix(path, "/operate/license") {
				if path == "/operate/license/tenant-mode" && !isInternal {
					c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "没有权限修改系统架构模式"))
					c.Abort()
					return
				}
				c.Next()
				return
			}
			// 模拟其它常规 operate 接口需要严格权限
			c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权限访问"))
			c.Abort()
		}
	}

	// 1. 模拟商户或非运营管理员登录（isInternal = false）
	routerMch := gin.New()
	routerMch.Use(mockBypassMiddleware(false))
	RegisterLicenseRoutes(routerMch, service)

	// (a) 商户调用 status 接口 (旁路豁免应放行 -> 返回 200)
	reqA := httptest.NewRequest(http.MethodGet, "/operate/license/status", nil)
	recA := httptest.NewRecorder()
	routerMch.ServeHTTP(recA, reqA)
	if recA.Code != http.StatusOK {
		t.Fatalf("expected merchant to pass status check under bypass, got status %d, body: %s", recA.Code, recA.Body.String())
	}

	// (b) 商户尝试修改租户架构模式 (应当被阻断 -> 返回 403)
	reqB := httptest.NewRequest(http.MethodPost, "/operate/license/tenant-mode", bytes.NewReader([]byte(`{"mode":"multi"}`)))
	reqB.Header.Set("Content-Type", "application/json")
	recB := httptest.NewRecorder()
	routerMch.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusForbidden {
		t.Fatalf("expected merchant to be blocked from tenant-mode API, got status %d", recB.Code)
	}

	// 2. 模拟运营内部超级管理员登录（isInternal = true）
	routerOp := gin.New()
	routerOp.Use(mockBypassMiddleware(true))
	RegisterLicenseRoutes(routerOp, service)

	// (a) 运营管理员修改架构模式 (应当成功通过 -> 返回 200)
	reqC := httptest.NewRequest(http.MethodPost, "/operate/license/tenant-mode", bytes.NewReader([]byte(`{"mode":"multi"}`)))
	reqC.Header.Set("Content-Type", "application/json")
	recC := httptest.NewRecorder()
	routerOp.ServeHTTP(recC, reqC)
	if recC.Code != http.StatusOK {
		t.Fatalf("expected operator to modify tenant mode, got status %d, body: %s", recC.Code, recC.Body.String())
	}
}
