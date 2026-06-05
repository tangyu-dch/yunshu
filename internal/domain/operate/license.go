package operate

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// 默认内置的公钥 PEM 字节数组（经 0x5A 异或混淆以防止直接静态提取）
var pubKeyBytes = []byte{
	0x77, 0x77, 0x77, 0x77, 0x77, 0x18, 0x1F, 0x1D, 0x13, 0x14, 0x7A, 0x0A, 0x0F, 0x18, 0x16, 0x13,
	0x19, 0x7A, 0x11, 0x1F, 0x03, 0x77, 0x77, 0x77, 0x77, 0x77, 0x50, 0x17, 0x1C, 0x31, 0x2D, 0x1F,
	0x2D, 0x03, 0x12, 0x11, 0x35, 0x00, 0x13, 0x20, 0x30, 0x6A, 0x19, 0x1B, 0x0B, 0x03, 0x13, 0x11,
	0x35, 0x00, 0x13, 0x20, 0x30, 0x6A, 0x1E, 0x1B, 0x0B, 0x39, 0x1E, 0x0B, 0x3D, 0x1B, 0x1F, 0x38,
	0x68, 0x0F, 0x36, 0x08, 0x12, 0x0C, 0x2E, 0x2D, 0x6C, 0x37, 0x75, 0x1C, 0x32, 0x62, 0x6A, 0x2D,
	0x6D, 0x63, 0x0E, 0x19, 0x29, 0x6D, 0x3E, 0x10, 0x6E, 0x68, 0x1B, 0x50, 0x16, 0x19, 0x20, 0x35,
	0x6E, 0x09, 0x69, 0x68, 0x00, 0x3F, 0x02, 0x37, 0x28, 0x38, 0x3B, 0x2E, 0x18, 0x3F, 0x3C, 0x35,
	0x62, 0x23, 0x28, 0x38, 0x1D, 0x0B, 0x0E, 0x14, 0x62, 0x0E, 0x3B, 0x6A, 0x68, 0x17, 0x28, 0x17,
	0x0D, 0x62, 0x6A, 0x31, 0x28, 0x39, 0x19, 0x12, 0x3F, 0x69, 0x75, 0x32, 0x00, 0x18, 0x1E, 0x08,
	0x0B, 0x17, 0x02, 0x28, 0x3B, 0x2D, 0x67, 0x67, 0x50, 0x77, 0x77, 0x77, 0x77, 0x77, 0x1F, 0x14,
	0x1E, 0x7A, 0x0A, 0x0F, 0x18, 0x16, 0x13, 0x19, 0x7A, 0x11, 0x1F, 0x03, 0x77, 0x77, 0x77, 0x77,
	0x77, 0x50,
}

// 缓存水位时间的配置键
const KeyLicenseHighWater = "license.status.hw"

// 首次运行试用期限的配置键
const KeySystemInstallTime = "system.install_time"

// HardwareFingerprint 物理/虚拟硬件指纹
type HardwareFingerprint struct {
	UUID       string   `json:"uuid"`
	MACs       []string `json:"macs"`
	DiskSerial string   `json:"disk_serial"`
	Hostname   string   `json:"hostname"`
}

// LicenseLimits 授权资源限制
type LicenseLimits struct {
	MaxExtensions      int      `json:"max_extensions"`
	MaxConcurrentCalls int      `json:"max_concurrent_calls"`
	Features           []string `json:"features"`
}

// LicenseClaims 授权书主体负荷
type LicenseClaims struct {
	LicenseID            string        `json:"license_id"`
	CustomerName         string        `json:"customer_name"`
	MerchantID           string        `json:"merchant_id"`
	DeploymentID         string        `json:"deployment_id"`
	LicenseType          string        `json:"license_type"`                     // "initial" (首次) | "renewal" (续期) | "migration" (平移)
	PreviousDeploymentID string        `json:"previous_deployment_id,omitempty"` // 被平移的前一个部署ID
	IssuedAt             int64         `json:"issued_at"`
	NotBefore            int64         `json:"not_before"`
	NotAfter             int64         `json:"not_after"`
	Limits               LicenseLimits `json:"limits"`
}

// LicenseData 授权文件打包结构
type LicenseData struct {
	PayloadRaw []byte `json:"payload"`
	Signature  []byte `json:"signature"`
}

// LicenseStatusResponse 运营管理端展示的授权状态
type LicenseStatusResponse struct {
	LicenseID            string   `json:"licenseId"`
	CustomerName         string   `json:"customerName"`
	DeploymentID         string   `json:"deploymentId"`
	LicenseType          string   `json:"licenseType"`                    // "initial" | "renewal" | "migration"
	PreviousDeploymentID string   `json:"previousDeploymentId,omitempty"` // 前一个部署 ID
	NotBefore            string   `json:"notBefore"`
	NotAfter             string   `json:"notAfter"`
	RemainingDays        int      `json:"remainingDays"`
	MaxConcurrentCalls   int      `json:"maxConcurrentCalls"`
	MaxExtensions        int      `json:"maxExtensions"`
	Features             []string `json:"features"`
	Status               string   `json:"status"` // "normal" | "grace_period" | "expired" | "time_rollback_locked" | "unlicensed"
	StatusMsg            string   `json:"statusMsg"`
	TenantMode           string   `json:"tenantMode"` // "single" | "multi"
}

// LicenseService 全局授权管理服务
type LicenseService struct {
	Repo        ProxyConfigRepository
	LicensePath string
	Logger      *slog.Logger
	mu          sync.RWMutex
	cached      *LicenseClaims
	cachedErr   error
}

// NewLicenseService 创建授权管理服务
func NewLicenseService(repo ProxyConfigRepository, licensePath string, logger *slog.Logger) *LicenseService {
	if logger == nil {
		logger = slog.Default()
	}
	if licensePath == "" {
		licensePath = "configs/yunshu.lic"
	}
	return &LicenseService{
		Repo:        repo,
		LicensePath: licensePath,
		Logger:      logger,
	}
}

// GetPubKey 获取还原后的公钥
func GetPubKey() (*ecdsa.PublicKey, error) {
	pemBytes := make([]byte, len(pubKeyBytes))
	for i, b := range pubKeyBytes {
		pemBytes[i] = b ^ 0x5A
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("【云枢授权】公钥 PEM 解析失败")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("【云枢授权】解析 PKIX 公钥失败: %w", err)
	}
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("【云枢授权】公钥不是有效的 ECDSA 类型")
	}
	return ecdsaPub, nil
}

// GetHardwareFingerprint 采集本地硬件指纹
func (s *LicenseService) GetHardwareFingerprint() (HardwareFingerprint, error) {
	var fp HardwareFingerprint

	// 1. 采集 MAC 地址（过滤虚拟网卡和回环地址，保留物理网卡前缀）
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			name := strings.ToLower(iface.Name)
			// 只保留物理网卡前缀 (en, eth, wlan, em)
			isPhysical := false
			for _, prefix := range []string{"en", "eth", "wlan", "em"} {
				if strings.HasPrefix(name, prefix) {
					isPhysical = true
					break
				}
			}
			if !isPhysical {
				continue
			}
			mac := iface.HardwareAddr.String()
			if mac != "" {
				fp.MACs = append(fp.MACs, mac)
			}
		}
		// 对 MACs 进行字母排序以确保稳定性
		sort.Strings(fp.MACs)
	}

	// 2. 采集主板 UUID (Linux)
	uuidBytes, err := os.ReadFile("/sys/class/dmi/id/product_uuid")
	if err == nil {
		fp.UUID = strings.TrimSpace(string(uuidBytes))
	}

	// 3. 采集主硬盘序列号 (Linux)
	diskBytes, err := os.ReadFile("/sys/block/sda/device/serial")
	if err == nil {
		fp.DiskSerial = strings.TrimSpace(string(diskBytes))
	} else {
		// 备用探测 nvme 硬盘
		diskBytes, err = os.ReadFile("/sys/class/nvme/nvme0/serial")
		if err == nil {
			fp.DiskSerial = strings.TrimSpace(string(diskBytes))
		}
	}

	// 4. 采集 Hostname (跨平台兜底)
	hostname, _ := os.Hostname()
	fp.Hostname = hostname

	// 如果处于非 Linux 开发环境，且未采集到任何硬件标识，则生成稳定 Mock 标识以保证开发调试可用
	if fp.UUID == "" && len(fp.MACs) == 0 && fp.DiskSerial == "" {
		fp.UUID = "MOCK-DEV-UUID-88AB7CCF9"
		fp.MACs = []string{"00:11:22:33:44:55"}
		fp.DiskSerial = "MOCK-DEV-DISK-998822"
	}

	return fp, nil
}

// GetDeploymentID 确定性计算唯一部署 ID
func (s *LicenseService) GetDeploymentID() (string, error) {
	fp, err := s.GetHardwareFingerprint()
	if err != nil {
		return "", err
	}

	// 稳定匹配硬件，通过 SHA256 加盐防碰撞，并进行截断分组
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("salt:yunshu:uuid:%s:disk:%s:macs:%v:host:%s", fp.UUID, fp.DiskSerial, fp.MACs, fp.Hostname)))
	hashBytes := h.Sum(nil)
	hexStr := fmt.Sprintf("%02X", hashBytes)

	// 格式化输出为 DEPLOY-XXXX-XXXX-XXXX
	if len(hexStr) >= 12 {
		return fmt.Sprintf("DEPLOY-%s-%s-%s", hexStr[0:4], hexStr[4:8], hexStr[8:12]), nil
	}
	return "DEPLOY-YUNSHU-FALLBACK", nil
}

// VerifyLicense 核心验签与双因子绑定校验
func (s *LicenseService) VerifyLicense(ctx context.Context, licenseBytes []byte) (*LicenseClaims, error) {
	pubKey, err := GetPubKey()
	if err != nil {
		return nil, err
	}

	// 1. 反序列化
	var data LicenseData
	if err := json.Unmarshal(licenseBytes, &data); err != nil {
		return nil, errors.New("【云枢授权】证书数据解析失败，文件可能损坏")
	}

	// 2. 校验数字签名
	hash := sha256.Sum256(data.PayloadRaw)
	if !ecdsa.VerifyASN1(pubKey, hash[:], data.Signature) {
		return nil, errors.New("【云枢授权】证书签名校验未通过，内容已被篡改")
	}

	// 3. 校验 Claims
	var claims LicenseClaims
	if err := json.Unmarshal(data.PayloadRaw, &claims); err != nil {
		return nil, errors.New("【云枢授权】证书负荷解析失败")
	}

	// 4. 校验确定性部署 ID 是否完全一致（防虚拟机克隆）
	depID, err := s.GetDeploymentID()
	if err != nil {
		return nil, fmt.Errorf("【云枢授权】提取本机部署 ID 失败: %w", err)
	}
	if claims.DeploymentID != depID {
		return nil, fmt.Errorf("【云枢授权】证书部署标识(%s)与本机硬件标识(%s)不匹配，禁止运行", claims.DeploymentID, depID)
	}

	// 打印证书类型日志，区分平移和续期
	switch claims.LicenseType {
	case "migration":
		s.Logger.Info("【云枢授权】检测到授权证书平移生效", "原部署ID", claims.PreviousDeploymentID, "新部署ID", claims.DeploymentID, "有效期保持不变", time.Unix(claims.NotAfter, 0).Format("2006-01-02"))
	case "renewal":
		s.Logger.Info("【云枢授权】检测到授权证书续期生效", "部署ID", claims.DeploymentID, "新有效期至", time.Unix(claims.NotAfter, 0).Format("2006-01-02"))
	default:
		s.Logger.Info("【云枢授权】检测到首次/标准授权证书生效", "部署ID", claims.DeploymentID, "有效期至", time.Unix(claims.NotAfter, 0).Format("2006-01-02"))
	}

	// 5. 校验授权时间限制
	now := time.Now().Unix()
	if now < claims.NotBefore {
		return nil, errors.New("【云枢授权】证书尚未生效")
	}

	// 允许 15 天宽限期
	gracePeriodSecs := int64(15 * 24 * 3600)
	if now > claims.NotAfter+gracePeriodSecs {
		return nil, errors.New("【云枢授权】证书已于 " + time.Unix(claims.NotAfter, 0).Format("2006-01-02") + " 过期，宽限期也已结束，系统已锁定呼叫能力")
	}

	// 6. 校验时间回拨保护
	if s.Repo != nil {
		hwItem, err := s.Repo.Get(ctx, KeyLicenseHighWater)
		if err == nil && hwItem.Value != "" {
			hwTime, decryptErr := decryptTimestamp(hwItem.Value)
			if decryptErr == nil {
				// 容忍 5 分钟以内的系统时钟微调
				if hwTime-now > 300 {
					return nil, errors.New("【云枢授权】检测到系统时钟恶意回拨，证书锁定。可信水位时间: " + time.Unix(hwTime, 0).Format("2006-01-02 15:04:05"))
				}
			}
		}
		// 静默更新高水位
		_ = s.UpdateHighWatermark(ctx, now)
	}

	return &claims, nil
}

// LoadAndVerify 从本地磁盘/数据库加载并校验 License
func (s *LicenseService) LoadAndVerify(ctx context.Context) (*LicenseClaims, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	depID, err := s.GetDeploymentID()
	if err != nil {
		s.cached = nil
		s.cachedErr = err
		return nil, err
	}

	var data []byte
	var loadedFromDB bool

	// 1. 尝试从数据库加载
	if s.Repo != nil {
		item, err := s.Repo.Get(ctx, "system.license")
		if err == nil && item.Value != "" {
			data = []byte(item.Value)
			loadedFromDB = true
		}
	}

	// 2. 如果数据库没有，或者为空，尝试从本地文件加载
	if len(data) == 0 {
		fileData, err := os.ReadFile(s.LicensePath)
		if err == nil {
			data = fileData
		}
	}

	// 3. 如果两者都没有，则走试用期逻辑
	if len(data) == 0 {
		// 证书不存在，尝试读取首次启动试用期限（15天试用豁免）
		installTime, trialErr := s.getOrSeedInstallTime(ctx)
		if trialErr == nil {
			now := time.Now().Unix()
			graceSecs := int64(15 * 24 * 3600)
			if now <= installTime+graceSecs {
				// 在 15 天试用期内，生成临时测试 Claims 放行核心功能
				trialClaims := &LicenseClaims{
					LicenseID:    "LIC-TRIAL-TEMP",
					CustomerName: "私有化部署试用环境",
					DeploymentID: depID,
					LicenseType:  "trial",
					IssuedAt:     installTime,
					NotBefore:    installTime,
					NotAfter:     installTime + graceSecs,
					Limits: LicenseLimits{
						MaxConcurrentCalls: 10,
						MaxExtensions:      50,
						Features:           []string{"trial"},
					},
				}
				s.cached = trialClaims
				s.cachedErr = nil
				return trialClaims, nil
			}
		}

		s.cached = nil
		s.cachedErr = errors.New("【云枢授权】未找到授权证书，且已超出 15 天试用宽限期。请上传激活文件")
		return nil, s.cachedErr
	}

	claims, err := s.VerifyLicense(ctx, data)
	if err != nil {
		s.cached = nil
		s.cachedErr = err
		return nil, err
	}

	// 如果是从本地文件加载成功的，且数据库中还没有该凭证，在校验通过后，自动同步写入数据库
	if !loadedFromDB && s.Repo != nil {
		if errDb := s.Repo.Set(ctx, "system.license", string(data), "云枢激活授权证书内容"); errDb != nil {
			s.Logger.Error("【云枢授权】自动同步证书到数据库失败", "error", errDb.Error())
		}
	}

	s.cached = claims
	s.cachedErr = nil
	return claims, nil
}

// UpdateHighWatermark 更新数据库中的最高历史时间戳
func (s *LicenseService) UpdateHighWatermark(ctx context.Context, t int64) error {
	if s.Repo == nil {
		return nil
	}

	// 读取当前的
	hwItem, err := s.Repo.Get(ctx, KeyLicenseHighWater)
	if err == nil && hwItem.Value != "" {
		currHW, errDec := decryptTimestamp(hwItem.Value)
		if errDec == nil && t <= currHW {
			return nil // 不可倒退更新
		}
	}

	encrypted := encryptTimestamp(t)
	return s.Repo.Set(ctx, KeyLicenseHighWater, encrypted, "云枢授权系统-历史最高时间戳（防回拨）")
}

// CheckConcurrencyLimit 运行时校验并发上限限制
func (s *LicenseService) CheckConcurrencyLimit(ctx context.Context, activeCount int) error {
	s.mu.RLock()
	claims := s.cached
	cachedErr := s.cachedErr
	s.mu.RUnlock()

	// 如果启动时未校通过授权，以启动时报错为准
	if cachedErr != nil {
		return cachedErr
	}
	if claims == nil {
		// 再次触发热加载
		var err error
		claims, err = s.LoadAndVerify(ctx)
		if err != nil {
			return err
		}
	}

	now := time.Now().Unix()

	// 1. 是否过期校验
	if now > claims.NotAfter {
		graceSecs := int64(15 * 24 * 3600)
		if now > claims.NotAfter+graceSecs {
			return errors.New("【云枢授权】授权已过期，宽限期也已耗尽，请联系供应商续期")
		}
		// 处于宽限期内，限制最大并发为额定的 80% (软熔断)
		maxCalls := int(float64(claims.Limits.MaxConcurrentCalls) * 0.8)
		if maxCalls < 1 {
			maxCalls = 1
		}
		if activeCount >= maxCalls {
			return fmt.Errorf("【云枢授权】当前处于过期宽限期内，软熔断限制最大并发为额定的80%%（当前允许最大并发：%d，当前活动并发：%d）", maxCalls, activeCount)
		}
		return nil
	}

	// 2. 额定并发限制校验
	if activeCount >= claims.Limits.MaxConcurrentCalls {
		return fmt.Errorf("【云枢授权】超出购买套餐最大并发数限制（限制并发数：%d，当前活动并发：%d）", claims.Limits.MaxConcurrentCalls, activeCount)
	}

	return nil
}

// CheckFeatureLimit 校验指定的功能模块是否已被授权
func (s *LicenseService) CheckFeatureLimit(ctx context.Context, feature string) error {
	s.mu.RLock()
	claims := s.cached
	cachedErr := s.cachedErr
	s.mu.RUnlock()

	if cachedErr != nil {
		return cachedErr
	}
	if claims == nil {
		var err error
		claims, err = s.LoadAndVerify(ctx)
		if err != nil {
			return err
		}
	}

	// 试用期默认放行所有基础功能模块
	if claims.LicenseType == "trial" {
		return nil
	}

	for _, f := range claims.Limits.Features {
		if f == feature || f == "*" {
			return nil
		}
	}

	return fmt.Errorf("【云枢授权】功能模块[%s]未获授权，请联系管理员导入有效证书", feature)
}

// GetLicenseStatus 获取完整系统授权状态详情
func (s *LicenseService) GetLicenseStatus(ctx context.Context) (LicenseStatusResponse, error) {
	depID, err := s.GetDeploymentID()
	if err != nil {
		return LicenseStatusResponse{}, err
	}

	tenantMode := "single"
	if s.Repo != nil {
		item, err := s.Repo.Get(ctx, "tenant.mode")
		if err == nil && item.Value != "" {
			tenantMode = item.Value
		}
	}

	claims, err := s.LoadAndVerify(ctx)
	if err != nil {
		status := "unlicensed"
		statusMsg := err.Error()

		// 区分具体异常状态
		if strings.Contains(statusMsg, "回拨") {
			status = "time_rollback_locked"
		} else if strings.Contains(statusMsg, "过期") {
			status = "expired"
		}

		return LicenseStatusResponse{
			DeploymentID: depID,
			Status:       status,
			StatusMsg:    statusMsg,
			TenantMode:   tenantMode,
		}, nil
	}

	now := time.Now().Unix()
	remainingSecs := claims.NotAfter - now
	remainingDays := int(remainingSecs / (24 * 3600))
	if remainingSecs > 0 && remainingSecs%(24*3600) > 0 {
		remainingDays++
	}

	status := "normal"
	statusMsg := "授权服务运行正常"
	if claims.LicenseType == "trial" {
		status = "grace_period"
		statusMsg = fmt.Sprintf("系统当前未激活，处于 15 天试用宽限期内（试用截止：%s，试用期后将锁定核心功能）", time.Unix(claims.NotAfter, 0).Format("2006-01-02"))
	} else if now > claims.NotAfter {
		status = "grace_period"
		statusMsg = fmt.Sprintf("授权已过期，当前处于宽限期内（宽限期截止：%s，并发上限调为80%%）", time.Unix(claims.NotAfter+(15*24*3600), 0).Format("2006-01-02"))
	}

	return LicenseStatusResponse{
		LicenseID:            claims.LicenseID,
		CustomerName:         claims.CustomerName,
		DeploymentID:         claims.DeploymentID,
		LicenseType:          claims.LicenseType,
		PreviousDeploymentID: claims.PreviousDeploymentID,
		NotBefore:            time.Unix(claims.NotBefore, 0).Format("2006-01-02"),
		NotAfter:             time.Unix(claims.NotAfter, 0).Format("2006-01-02"),
		RemainingDays:        remainingDays,
		MaxConcurrentCalls:   claims.Limits.MaxConcurrentCalls,
		MaxExtensions:        claims.Limits.MaxExtensions,
		Features:             claims.Limits.Features,
		Status:               status,
		StatusMsg:            statusMsg,
		TenantMode:           tenantMode,
	}, nil
}

// SaveLicense 保存证书文件并热重新装载
func (s *LicenseService) SaveLicense(ctx context.Context, licenseBytes []byte) error {
	// 先进行合规性验证，防止用户上传无效证书导致系统停摆
	_, err := s.VerifyLicense(ctx, licenseBytes)
	if err != nil {
		return err
	}

	// 写入数据库
	if s.Repo != nil {
		if err := s.Repo.Set(ctx, "system.license", string(licenseBytes), "云枢激活授权证书内容"); err != nil {
			s.Logger.Error("【云枢授权】保存证书到数据库失败", "error", err.Error())
		}
	}

	// 写入本地 configs 目录 (作为一个本地备份，若写入失败打日志即可，不影响核心逻辑)
	dir := filepath.Dir(s.LicensePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.Logger.Warn("【云枢授权】创建证书备份文件夹失败", "error", err.Error())
	} else {
		if err := os.WriteFile(s.LicensePath, licenseBytes, 0644); err != nil {
			s.Logger.Warn("【云枢授权】写入证书备份文件失败", "error", err.Error())
		}
	}

	// 热更新内存加载
	_, _ = s.LoadAndVerify(ctx)
	return nil
}

// 简单轻量级的对称加密/混淆：XOR 混淆字节并转换为 Base64
func encryptTimestamp(t int64) string {
	str := fmt.Sprintf("%d", t)
	bytes := []byte(str)
	for i := range bytes {
		bytes[i] ^= 0x3F
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func decryptTimestamp(s string) (int64, error) {
	bytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return 0, err
	}
	for i := range bytes {
		bytes[i] ^= 0x3F
	}
	var t int64
	_, err = fmt.Sscanf(string(bytes), "%d", &t)
	if err != nil {
		return 0, err
	}
	return t, nil
}

func (s *LicenseService) getOrSeedInstallTime(ctx context.Context) (int64, error) {
	if s.Repo == nil {
		return time.Now().Unix(), nil
	}
	item, err := s.Repo.Get(ctx, KeySystemInstallTime)
	if err == nil && item.Value != "" {
		t, decryptErr := decryptTimestamp(item.Value)
		if decryptErr == nil {
			return t, nil
		}
	}
	// 播种安装时间
	now := time.Now().Unix()
	encrypted := encryptTimestamp(now)
	_ = s.Repo.Set(ctx, KeySystemInstallTime, encrypted, "云枢授权系统-系统首次安装启动时间戳（试用宽限起点）")
	return now, nil
}
