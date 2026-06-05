package system

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// ProxyConfigModel 映射系统信令代理配置键值对表。
type ProxyConfigModel struct {
	ConfigKey   string    `gorm:"column:config_key;primaryKey;type:varchar(128)"`
	ConfigValue string    `gorm:"column:config_value;type:text"`
	Description string    `gorm:"column:description;type:varchar(255)"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

// TableName 返回 proxy_config 表名。
func (ProxyConfigModel) TableName() string {
	return "cc_sys_config"
}

// ProxyConfigRepository 基于 GORM 的代理配置仓储实现。
type ProxyConfigRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewProxyConfigRepository 创建 GORM 代理配置仓储。
func NewProxyConfigRepository(db *gorm.DB, logger *slog.Logger) *ProxyConfigRepository {
	return &ProxyConfigRepository{DB: db, Logger: logger}
}

// Get 根据 Key 读取单条配置。
func (r *ProxyConfigRepository) Get(ctx context.Context, key string) (operate.ProxyConfigItem, error) {
	var model ProxyConfigModel
	err := r.DB.WithContext(ctx).Where("config_key = ?", key).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return operate.ProxyConfigItem{}, operate.ErrConfigNotFound
		}
		return operate.ProxyConfigItem{}, err
	}
	return operate.ProxyConfigItem{
		Key:         model.ConfigKey,
		Value:       model.ConfigValue,
		Description: model.Description,
		UpdatedTime: model.UpdatedTime,
	}, nil
}

// Set 保存或更新单条配置项。
func (r *ProxyConfigRepository) Set(ctx context.Context, key, value, desc string) error {
	model := ProxyConfigModel{
		ConfigKey:   key,
		ConfigValue: value,
		Description: desc,
		UpdatedTime: time.Now(),
	}
	return r.DB.WithContext(ctx).Save(&model).Error
}

// List 获取全部配置项列表。
func (r *ProxyConfigRepository) List(ctx context.Context) ([]operate.ProxyConfigItem, error) {
	var models []ProxyConfigModel
	err := r.DB.WithContext(ctx).Find(&models).Error
	if err != nil {
		return nil, err
	}
	items := make([]operate.ProxyConfigItem, 0, len(models))
	for _, m := range models {
		items = append(items, operate.ProxyConfigItem{
			Key:         m.ConfigKey,
			Value:       m.ConfigValue,
			Description: m.Description,
			UpdatedTime: m.UpdatedTime,
		})
	}
	return items, nil
}

// EnsureDefaults 初始化注入默认网络参数种子数据。
func (r *ProxyConfigRepository) EnsureDefaults(ctx context.Context) error {
	defaults := []struct {
		key, val, desc string
	}{
		{operate.KeyKamailioUdpIp, "0.0.0.0", "Kamailio UDP 监听 IP (默认 0.0.0.0)"},
		{operate.KeyKamailioTcpIp, "0.0.0.0", "Kamailio TCP 监听 IP (默认 0.0.0.0)"},
		{operate.KeyKamailioSipPort, "5060", "Kamailio SIP 端口 (默认 5060)"},
		{operate.KeyKamailioWsPort, "5066", "Kamailio WebRTC WebSocket 端口 (默认 5066)"},
		{operate.KeyKamailioExternalIp, "127.0.0.1", "Kamailio 外部映射公网 IP"},
		{operate.KeyRtpengineInternalIp, "0.0.0.0", "RTPEngine 内网 IP / 监听接口 (默认 0.0.0.0)"},
		{operate.KeyRtpengineSdpIp, "127.0.0.1", "RTPEngine 在 SDP 中宣告的公网 IP (NAT 穿透关键)"},
		{operate.KeyRtpengineStartPort, "30000", "RTPEngine 媒体端口范围起始 (默认 30000)"},
		{operate.KeyRtpengineEndPort, "30100", "RTPEngine 媒体端口范围结束 (默认 30100)"},
		{"system.nearby_cities", `{"510100":["511300","510600","510700"],"440100":["440600","441900","440300"]}`, "号码选择相邻/邻近城市匹配配置(JSON)"},
		{operate.KeySipTraceEnable, "0", "是否开启 SIP 信令链路追踪 (1-开启, 0-关闭)"},
	}

	for _, d := range defaults {
		var model ProxyConfigModel
		err := r.DB.WithContext(ctx).Where("config_key = ?", d.key).First(&model).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger().Info("播种系统网络参数默认种子", "key", d.key, "value", d.val)
			err = r.Set(ctx, d.key, d.val, d.desc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ProxyConfigRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
