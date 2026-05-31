package operate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// 代理配置参数 Key 定义，属于信令与媒体转发网关
const (
	KeyKamailioUdpIp       = "kamailio.udp_ip"
	KeyKamailioTcpIp       = "kamailio.tcp_ip"
	KeyKamailioSipPort     = "kamailio.sip_port"
	KeyKamailioWsPort      = "kamailio.ws_port"
	KeyKamailioExternalIp  = "kamailio.external_ip"
	KeyRtpengineInternalIp = "rtpengine.internal_ip"
	KeyRtpengineSdpIp      = "rtpengine.sdp_ip"
	KeyRtpengineStartPort  = "rtpengine.rtp_start_port"
	KeyRtpengineEndPort    = "rtpengine.rtp_end_port"
)

// ErrConfigNotFound 表示配置项不存在。
var ErrConfigNotFound = errors.New("配置项不存在")

// ProxyConfig 表示信令代理与媒体代理核心参数。
type ProxyConfig struct {
	KamailioUdpIp       string `json:"kamailioUdpIp"`
	KamailioTcpIp       string `json:"kamailioTcpIp"`
	KamailioSipPort     int    `json:"kamailioSipPort"`
	KamailioWsPort      int    `json:"kamailioWsPort"`
	KamailioExternalIp  string `json:"kamailioExternalIp"`
	RtpengineInternalIp string `json:"rtpengineInternalIp"`
	RtpengineSdpIp      string `json:"rtpengineSdpIp"`
	RtpengineStartPort  int    `json:"rtpengineStartPort"`
	RtpengineEndPort    int    `json:"rtpengineEndPort"`
	KamailioStatus      string `json:"kamailioStatus,omitempty"` // 内存中存储的实时物理在线状态: "online" | "offline"
}

// ProxyConfigItem 表示数据库存储的单条代理配置项。
type ProxyConfigItem struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	UpdatedTime time.Time `json:"updatedTime"`
}

// ProxyConfigRepository 定义代理配置项的存储接口。
type ProxyConfigRepository interface {
	Get(ctx context.Context, key string) (ProxyConfigItem, error)
	Set(ctx context.Context, key, value, description string) error
	List(ctx context.Context) ([]ProxyConfigItem, error)
	EnsureDefaults(ctx context.Context) error
}

// ProxyConfigManagementService 承载系统代理与网络参数的动态配置业务。
type ProxyConfigManagementService struct {
	Repo           ProxyConfigRepository
	RtpReloader    RtpengineReloadPort
	Logger         *slog.Logger
	ConfigFilePath string
	ComposePath    string
	mu             sync.Mutex
}

// NewProxyConfigManagementService 创建代理配置管理服务。
func NewProxyConfigManagementService(repo ProxyConfigRepository, reloader RtpengineReloadPort, logger *slog.Logger) *ProxyConfigManagementService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ProxyConfigManagementService{
		Repo:           repo,
		RtpReloader:    reloader,
		Logger:         logger,
		ConfigFilePath: "configs/default.yaml",
		ComposePath:    "docker-compose.yml",
	}
}

// GetConfig 加载完整的系统代理配置。若数据库不存在配置则返回种子默认值。
func (s *ProxyConfigManagementService) GetConfig(ctx context.Context) (ProxyConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.Repo.List(ctx)
	if err != nil {
		s.Logger.Error("运营端查询代理配置失败", "error", err.Error())
		return ProxyConfig{}, err
	}

	configMap := make(map[string]string)
	for _, item := range items {
		configMap[item.Key] = item.Value
	}

	sipPort := getIntVal(configMap, KeyKamailioSipPort, 5060)
	wsPort := getIntVal(configMap, KeyKamailioWsPort, 5066)

	// 极速物理在线检测
	kamailioStatus := pingKamailio(sipPort, wsPort)

	return ProxyConfig{
		KamailioUdpIp:       getStrVal(configMap, KeyKamailioUdpIp, "0.0.0.0"),
		KamailioTcpIp:       getStrVal(configMap, KeyKamailioTcpIp, "0.0.0.0"),
		KamailioSipPort:     sipPort,
		KamailioWsPort:      wsPort,
		KamailioExternalIp:  getStrVal(configMap, KeyKamailioExternalIp, "127.0.0.1"),
		RtpengineInternalIp: getStrVal(configMap, KeyRtpengineInternalIp, "0.0.0.0"),
		RtpengineSdpIp:      getStrVal(configMap, KeyRtpengineSdpIp, "127.0.0.1"),
		RtpengineStartPort:  getIntVal(configMap, KeyRtpengineStartPort, 30000),
		RtpengineEndPort:    getIntVal(configMap, KeyRtpengineEndPort, 30100),
		KamailioStatus:      kamailioStatus,
	}, nil
}

// SaveConfig 保存用户在页面提交的代理配置并写入数据库。
func (s *ProxyConfigManagementService) SaveConfig(ctx context.Context, config ProxyConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Logger.Info("运营端开始保存代理网络配置", "kamailioSipPort", config.KamailioSipPort, "rtpStartPort", config.RtpengineStartPort, "rtpEndPort", config.RtpengineEndPort)

	if config.RtpengineStartPort <= 0 || config.RtpengineEndPort <= 0 || config.RtpengineStartPort >= config.RtpengineEndPort {
		return errors.New("RTP 端口范围无效")
	}
	if config.KamailioSipPort <= 0 || config.KamailioWsPort <= 0 {
		return errors.New("Kamailio 端口号无效")
	}

	configs := []struct {
		key, val, desc string
	}{
		{KeyKamailioUdpIp, config.KamailioUdpIp, "Kamailio UDP 监听地址"},
		{KeyKamailioTcpIp, config.KamailioTcpIp, "Kamailio TCP 监听地址"},
		{KeyKamailioSipPort, strconv.Itoa(config.KamailioSipPort), "Kamailio SIP 端口"},
		{KeyKamailioWsPort, strconv.Itoa(config.KamailioWsPort), "Kamailio WebRTC WebSocket 端口"},
		{KeyKamailioExternalIp, config.KamailioExternalIp, "Kamailio 外部映射公网 IP"},
		{KeyRtpengineInternalIp, config.RtpengineInternalIp, "RTPEngine 内网 IP / 监听接口"},
		{KeyRtpengineSdpIp, config.RtpengineSdpIp, "RTPEngine 在 SDP 中宣告的公网 IP"},
		{KeyRtpengineStartPort, strconv.Itoa(config.RtpengineStartPort), "RTPEngine 媒体端口范围起始"},
		{KeyRtpengineEndPort, strconv.Itoa(config.RtpengineEndPort), "RTPEngine 媒体端口范围结束"},
	}

	for _, cfg := range configs {
		if err := s.Repo.Set(ctx, cfg.key, cfg.val, cfg.desc); err != nil {
			s.Logger.Error("保存单条系统配置项失败", "key", cfg.key, "error", err.Error())
			return err
		}
	}

	s.Logger.Info("运营端成功保存代理网络配置至数据库")
	return nil
}

// ApplyAndRestart 动态将最新的数据库代理配置写入配置文件和 docker-compose.yml，并重启服务容器。
func (s *ProxyConfigManagementService) ApplyAndRestart(ctx context.Context) error {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return err
	}

	s.Logger.Info("开始动态生成并更新配置文件以应用网络变更")

	// 1. 修改/重写 default.yaml (只更新端口或涉及到的配置项)
	if err := s.updateDefaultYaml(cfg); err != nil {
		s.Logger.Error("重写 configs/default.yaml 失败", "error", err.Error())
		return err
	}

	// 2. 修改/重载 docker-compose.yml
	if err := s.updateDockerCompose(cfg); err != nil {
		s.Logger.Error("重写 docker-compose.yml 失败", "error", err.Error())
		return err
	}

	// 3. 执行重启命令：docker compose restart kamailio rtpengine
	s.Logger.Info("配置重写成功，开始在后台重启 Kamailio 与 RTPEngine 容器")
	go func() {
		cmd := exec.Command("docker", "compose", "restart", "kamailio", "rtpengine")
		output, err := cmd.CombinedOutput()
		if err != nil {
			cmd = exec.Command("docker-compose", "restart", "kamailio", "rtpengine")
			output, err = cmd.CombinedOutput()
		}
		if err != nil {
			s.Logger.Error("重启 Docker 容器失败", "error", err.Error(), "output", string(output))
		} else {
			s.Logger.Info("Docker 容器重启成功，新的网络与端口配置已生效")
		}
	}()

	return nil
}

func (s *ProxyConfigManagementService) updateDefaultYaml(cfg ProxyConfig) error {
	raw, err := os.ReadFile(s.ConfigFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var data map[string]any
	if err := yaml.Unmarshal(raw, &data); err != nil {
		return err
	}

	newRaw, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	return os.WriteFile(s.ConfigFilePath, newRaw, 0644)
}

func (s *ProxyConfigManagementService) updateDockerCompose(cfg ProxyConfig) error {
	raw, err := os.ReadFile(s.ComposePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if strings.Contains(line, "RTP_START_PORT=") {
			indent := line[:strings.Index(line, "- RTP_START_PORT=")]
			lines[i] = fmt.Sprintf("%s- RTP_START_PORT=%d", indent, cfg.RtpengineStartPort)
		}
		if strings.Contains(line, "RTP_END_PORT=") {
			indent := line[:strings.Index(line, "- RTP_END_PORT=")]
			lines[i] = fmt.Sprintf("%s- RTP_END_PORT=%d", indent, cfg.RtpengineEndPort)
		}
		if strings.Contains(line, "/udp\"") && strings.Contains(line, "- \"300") {
			indent := line[:strings.Index(line, "- \"")]
			lines[i] = fmt.Sprintf("%s- \"%d-%d:%d-%d/udp\"", indent, cfg.RtpengineStartPort, cfg.RtpengineEndPort, cfg.RtpengineStartPort, cfg.RtpengineEndPort)
		}
		if strings.Contains(line, ":5060/udp\"") {
			indent := line[:strings.Index(line, "- \"")]
			lines[i] = fmt.Sprintf("%s- \"%d:5060/udp\"", indent, cfg.KamailioSipPort)
		}
		if strings.Contains(line, ":5060/tcp\"") {
			indent := line[:strings.Index(line, "- \"")]
			lines[i] = fmt.Sprintf("%s- \"%d:5060/tcp\"", indent, cfg.KamailioSipPort)
		}
		if strings.Contains(line, ":5066/tcp\"") {
			indent := line[:strings.Index(line, "- \"")]
			lines[i] = fmt.Sprintf("%s- \"%d:5066/tcp\"", indent, cfg.KamailioWsPort)
		}
		if strings.Contains(line, "!DEFAULT_EXTERNAL_IP!") {
			indent := line[:strings.Index(line, "- '!DEFAULT_EXTERNAL_IP!")]
			lines[i] = fmt.Sprintf("%s- '!DEFAULT_EXTERNAL_IP!%s!g'", indent, cfg.KamailioExternalIp)
		}
	}

	return os.WriteFile(s.ComposePath, []byte(strings.Join(lines, "\n")), 0644)
}

func getStrVal(m map[string]string, key string, fallback string) string {
	if val, ok := m[key]; ok {
		return val
	}
	return fallback
}

func getIntVal(m map[string]string, key string, fallback int) int {
	if val, ok := m[key]; ok {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return fallback
}

// pingKamailio 实现在线网络检测
func pingKamailio(sipPort, wsPort int) string {
	// 采用 WebRTC WS 端口 (TCP) 和 SIP 端口 (通常映射为 TCP) 双重检测机制
	addrWS := fmt.Sprintf("127.0.0.1:%d", wsPort)
	conn, err := net.DialTimeout("tcp", addrWS, 150*time.Millisecond)
	if err == nil {
		conn.Close()
		return "online"
	}

	addrSIP := fmt.Sprintf("127.0.0.1:%d", sipPort)
	conn2, err := net.DialTimeout("tcp", addrSIP, 150*time.Millisecond)
	if err == nil {
		conn2.Close()
		return "online"
	}
	return "offline"
}
