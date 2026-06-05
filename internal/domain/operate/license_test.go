package operate

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// 用于测试的私钥 PEM，对应于内置的混淆公钥
const testPrivKeyPEM = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIOSrSXQ3ij2D+L6bEaQLpnuU0gmzKo/0N/vKQ/5g/PhpoAoGCCqGSM49
AwEHoUQDQgAEb2UlRHVtw6m/Fh80w79TCs7dJ42ALCzo4S32ZeXmrbatBefo8yrb
GQTN8Ta02MrMW80krcCHe3/hZBDRQMXraw==
-----END EC PRIVATE KEY-----`

// mockProxyConfigRepo 模拟配置仓储
type mockProxyConfigRepo struct {
	configs map[string]string
}

func newMockProxyConfigRepo() *mockProxyConfigRepo {
	return &mockProxyConfigRepo{
		configs: make(map[string]string),
	}
}

func (m *mockProxyConfigRepo) Get(ctx context.Context, key string) (ProxyConfigItem, error) {
	val, ok := m.configs[key]
	if !ok {
		return ProxyConfigItem{}, ErrConfigNotFound
	}
	return ProxyConfigItem{
		Key:         key,
		Value:       val,
		UpdatedTime: time.Now(),
	}, nil
}

func (m *mockProxyConfigRepo) Set(ctx context.Context, key, value, desc string) error {
	m.configs[key] = value
	return nil
}

func (m *mockProxyConfigRepo) List(ctx context.Context) ([]ProxyConfigItem, error) {
	var list []ProxyConfigItem
	for k, v := range m.configs {
		list = append(list, ProxyConfigItem{Key: k, Value: v})
	}
	return list, nil
}

func (m *mockProxyConfigRepo) EnsureDefaults(ctx context.Context) error {
	return nil
}

// 辅助函数：使用私钥为 Claims 签名并返回 License 二进制字节
func generateTestLicense(claims LicenseClaims) ([]byte, error) {
	block, _ := pem.Decode([]byte(testPrivKeyPEM))
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

	licData := LicenseData{
		PayloadRaw: payloadRaw,
		Signature:  sig,
	}

	return json.Marshal(licData)
}

func TestLicenseDeterministicDeploymentID(t *testing.T) {
	repo := newMockProxyConfigRepo()
	service := NewLicenseService(repo, "configs/test.lic", nil)

	id1, err := service.GetDeploymentID()
	if err != nil {
		t.Fatalf("failed to get deployment ID: %v", err)
	}

	id2, err := service.GetDeploymentID()
	if err != nil {
		t.Fatalf("failed to get deployment ID: %v", err)
	}

	if id1 != id2 {
		t.Errorf("deployment IDs are not deterministic: %s != %s", id1, id2)
	}

	if len(id1) < 10 {
		t.Errorf("deployment ID is too short: %s", id1)
	}
}

func TestLicenseVerificationFlow(t *testing.T) {
	repo := newMockProxyConfigRepo()
	licFile := "configs/test_temp.lic"
	defer os.Remove(licFile)

	service := NewLicenseService(repo, licFile, nil)

	depID, err := service.GetDeploymentID()
	if err != nil {
		t.Fatalf("failed to get deployment ID: %v", err)
	}

	now := time.Now().Unix()
	claims := LicenseClaims{
		LicenseID:    "LIC-TEST-001",
		CustomerName: "测试客户",
		MerchantID:   "1001",
		DeploymentID: depID,
		IssuedAt:     now,
		NotBefore:    now - 3600,
		NotAfter:     now + 3600*24*30, // 30天后到期
		Limits: LicenseLimits{
			MaxConcurrentCalls: 50,
			MaxExtensions:      100,
			Features:           []string{"outbound"},
		},
	}

	licBytes, err := generateTestLicense(claims)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	// 1. 验证签名和双因子校验成功
	claimsDec, err := service.VerifyLicense(context.Background(), licBytes)
	if err != nil {
		t.Fatalf("failed to verify license: %v", err)
	}

	if claimsDec.LicenseID != "LIC-TEST-001" {
		t.Errorf("incorrect license ID verified: %s", claimsDec.LicenseID)
	}

	// 2. 模拟写入本地文件并测试热装载
	err = service.SaveLicense(context.Background(), licBytes)
	if err != nil {
		t.Fatalf("failed to save license: %v", err)
	}

	status, err := service.GetLicenseStatus(context.Background())
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if status.Status != "normal" {
		t.Errorf("expected normal status, got: %s", status.Status)
	}
	if status.MaxConcurrentCalls != 50 {
		t.Errorf("expected concurrent calls 50, got: %d", status.MaxConcurrentCalls)
	}
}

func TestLicenseExpiryGracePeriodAndConcurrencyCheck(t *testing.T) {
	repo := newMockProxyConfigRepo()
	service := NewLicenseService(repo, "", nil)

	depID, err := service.GetDeploymentID()
	if err != nil {
		t.Fatalf("failed to get deployment ID: %v", err)
	}

	now := time.Now().Unix()
	claims := LicenseClaims{
		LicenseID:    "LIC-TEST-002",
		CustomerName: "测试客户",
		MerchantID:   "1001",
		DeploymentID: depID,
		IssuedAt:     now - 3600*24*10,
		NotBefore:    now - 3600*24*10,
		NotAfter:     now - 3600, // 已经过期 1 小时
		Limits: LicenseLimits{
			MaxConcurrentCalls: 10,
			MaxExtensions:      20,
			Features:           []string{"outbound"},
		},
	}

	licBytes, err := generateTestLicense(claims)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	// 注入并热加载
	service.cached, err = service.VerifyLicense(context.Background(), licBytes)
	if err != nil {
		t.Fatalf("verification should succeed even if slightly expired due to grace period: %v", err)
	}

	// 处于宽限期内，10 * 0.8 = 8 个最大并发允许
	err = service.CheckConcurrencyLimit(context.Background(), 7)
	if err != nil {
		t.Errorf("should allow 7 concurrent calls under grace period (80%% of 10 = 8): %v", err)
	}

	err = service.CheckConcurrencyLimit(context.Background(), 8)
	if err == nil {
		t.Errorf("should block 8 concurrent calls under grace period (80%% soft limit)")
	}
}

func TestLicenseTimeRollbackProtection(t *testing.T) {
	repo := newMockProxyConfigRepo()
	service := NewLicenseService(repo, "", nil)

	depID, err := service.GetDeploymentID()
	if err != nil {
		t.Fatalf("failed to get deployment ID: %v", err)
	}

	now := time.Now().Unix()
	claims := LicenseClaims{
		LicenseID:    "LIC-TEST-003",
		CustomerName: "测试客户",
		MerchantID:   "1001",
		DeploymentID: depID,
		IssuedAt:     now,
		NotBefore:    now - 3600,
		NotAfter:     now + 3600*24,
		Limits: LicenseLimits{
			MaxConcurrentCalls: 10,
			MaxExtensions:      20,
		},
	}

	licBytes, err := generateTestLicense(claims)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	// 1. 设置正常的可信高水位时间为未来 1 小时后
	futureTime := now + 3600
	err = service.UpdateHighWatermark(context.Background(), futureTime)
	if err != nil {
		t.Fatalf("failed to update high water: %v", err)
	}

	// 2. 校验，当前时间为 now，早于 futureTime 1小时，应该识别为时间回拨并锁定
	_, err = service.VerifyLicense(context.Background(), licBytes)
	if err == nil {
		t.Errorf("should have failed due to time rollback")
	} else if !testing.Short() {
		t.Logf("successfully caught rollback error: %v", err)
	}
}

func TestLicenseMigrationAndRenewal(t *testing.T) {
	repo := newMockProxyConfigRepo()
	service := NewLicenseService(repo, "", nil)

	depID, err := service.GetDeploymentID()
	if err != nil {
		t.Fatalf("failed to get deployment ID: %v", err)
	}

	now := time.Now().Unix()

	// 1. 测试平移类型证书
	claimsMigration := LicenseClaims{
		LicenseID:            "LIC-MIGRATION",
		CustomerName:         "平移测试客户",
		MerchantID:           "1001",
		DeploymentID:         depID,
		LicenseType:          "migration",
		PreviousDeploymentID: "DEPLOY-PREV-UUID-OLD",
		IssuedAt:             now,
		NotBefore:            now - 3600,
		NotAfter:             now + 3600*24*10,
		Limits: LicenseLimits{
			MaxConcurrentCalls: 10,
			MaxExtensions:      20,
		},
	}

	licBytes, err := generateTestLicense(claimsMigration)
	if err != nil {
		t.Fatalf("failed to generate migration license: %v", err)
	}

	claimsDec, err := service.VerifyLicense(context.Background(), licBytes)
	if err != nil {
		t.Fatalf("failed to verify migration license: %v", err)
	}

	if claimsDec.LicenseType != "migration" || claimsDec.PreviousDeploymentID != "DEPLOY-PREV-UUID-OLD" {
		t.Errorf("incorrect license type or previous deployment ID: type=%s, prev=%s", claimsDec.LicenseType, claimsDec.PreviousDeploymentID)
	}

	// 2. 测试续期类型证书
	claimsRenewal := LicenseClaims{
		LicenseID:    "LIC-RENEWAL",
		CustomerName: "续期测试客户",
		MerchantID:   "1001",
		DeploymentID: depID,
		LicenseType:  "renewal",
		IssuedAt:     now,
		NotBefore:    now - 3600,
		NotAfter:     now + 3600*24*365, // 1年
		Limits: LicenseLimits{
			MaxConcurrentCalls: 10,
			MaxExtensions:      20,
		},
	}

	licBytesRenewal, err := generateTestLicense(claimsRenewal)
	if err != nil {
		t.Fatalf("failed to generate renewal license: %v", err)
	}

	claimsDecRenewal, err := service.VerifyLicense(context.Background(), licBytesRenewal)
	if err != nil {
		t.Fatalf("failed to verify renewal license: %v", err)
	}

	if claimsDecRenewal.LicenseType != "renewal" {
		t.Errorf("incorrect license type: expected renewal, got: %s", claimsDecRenewal.LicenseType)
	}
}

func TestLicenseFeatureCheck(t *testing.T) {
	repo := newMockProxyConfigRepo()
	service := NewLicenseService(repo, "", nil)

	depID, err := service.GetDeploymentID()
	if err != nil {
		t.Fatalf("failed to get deployment ID: %v", err)
	}

	now := time.Now().Unix()
	claims := LicenseClaims{
		LicenseID:    "LIC-TEST-004",
		CustomerName: "测试客户",
		MerchantID:   "1001",
		DeploymentID: depID,
		IssuedAt:     now,
		NotBefore:    now - 3600,
		NotAfter:     now + 3600*24,
		Limits: LicenseLimits{
			MaxConcurrentCalls: 10,
			MaxExtensions:      20,
			Features:           []string{"outbound", "billing"},
		},
	}

	licBytes, err := generateTestLicense(claims)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	// 注入并热加载
	service.cached, err = service.VerifyLicense(context.Background(), licBytes)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	// 1. 验证已授权功能模块，应返回 nil (成功)
	if err := service.CheckFeatureLimit(context.Background(), "outbound"); err != nil {
		t.Errorf("should allow outbound feature: %v", err)
	}
	if err := service.CheckFeatureLimit(context.Background(), "billing"); err != nil {
		t.Errorf("should allow billing feature: %v", err)
	}

	// 2. 验证未授权功能模块，应返回错误
	if err := service.CheckFeatureLimit(context.Background(), "recording"); err == nil {
		t.Errorf("should block recording feature since it is not in the authorized features list")
	}
}

type fakeExtRepo struct {
	total int64
}

func (f *fakeExtRepo) Page(ctx context.Context, req ExtensionPageRequest) (ExtensionPageResult, error) {
	return ExtensionPageResult{Total: f.total}, nil
}
func (f *fakeExtRepo) GetByID(ctx context.Context, id int) (Extension, error) {
	return Extension{}, nil
}
func (f *fakeExtRepo) ExistsNumber(ctx context.Context, num string, merchantID int, excludeID int) (bool, error) {
	return false, nil
}
func (f *fakeExtRepo) Save(ctx context.Context, ext Extension) (Extension, error) {
	f.total++
	ext.ID = int(f.total)
	return ext, nil
}
func (f *fakeExtRepo) Delete(ctx context.Context, ids []int) error { return nil }
func (f *fakeExtRepo) SetEnable(ctx context.Context, id int, enable bool) (Extension, error) {
	return Extension{}, nil
}
func (f *fakeExtRepo) DynamicBind(ctx context.Context, extensionNumber string, userID int, merchantID int) error {
	return nil
}

func TestExtensionQuotaCheck(t *testing.T) {
	repo := newMockProxyConfigRepo()
	service := NewLicenseService(repo, "", nil)

	depID, err := service.GetDeploymentID()
	if err != nil {
		t.Fatalf("failed to get deployment ID: %v", err)
	}

	now := time.Now().Unix()
	claims := LicenseClaims{
		LicenseID:    "LIC-LIMIT-TEST",
		CustomerName: "分机限制测试",
		DeploymentID: depID,
		IssuedAt:     now,
		NotBefore:    now - 3600,
		NotAfter:     now + 3600*24,
		Limits: LicenseLimits{
			MaxConcurrentCalls: 10,
			MaxExtensions:      1, // 限制只能建 1 个分机
		},
	}

	licBytes, err := generateTestLicense(claims)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	// 写入 Mock 数据库
	err = repo.Set(context.Background(), "system.license", string(licBytes), "test license")
	if err != nil {
		t.Fatalf("failed to set mock license: %v", err)
	}

	extRepo := &fakeExtRepo{}
	extService := &ExtensionManagementService{
		Repository: extRepo,
		License:    service,
	}

	// 1. 新建第一个分机，应该成功 (当前有 0 个，限制最多 1 个)
	ext1, err := extService.Save(context.Background(), Extension{
		ExtensionNumber: "8005",
		MerchantID:      1001,
		Enable:          true,
	})
	if err != nil {
		t.Fatalf("saving first extension should succeed, got error: %v", err)
	}
	if ext1.ID != 1 {
		t.Errorf("expected extension ID 1, got %d", ext1.ID)
	}

	// 2. 新建第二个分机，应该失败 (当前有 1 个，限制最多 1 个)
	_, err = extService.Save(context.Background(), Extension{
		ExtensionNumber: "8006",
		MerchantID:      1001,
		Enable:          true,
	})
	if err == nil {
		t.Fatal("expected saving second extension to fail due to MaxExtensions quota limit")
	}
	if !strings.Contains(err.Error(), "最大分机数配额限制") {
		t.Errorf("unexpected error message: %v", err)
	}
}
