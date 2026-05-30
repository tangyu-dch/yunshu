package operate

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
)

var (
	// ErrInvalidGateway 表示运营端提交的网关配置缺少生产必需字段。
	ErrInvalidGateway = errors.New("invalid gateway")
	// ErrGatewayNotFound 表示请求的网关不存在或已逻辑删除。
	ErrGatewayNotFound = errors.New("gateway not found")
	// ErrGatewayConflict 表示网关名称或描述与现有未删除网关冲突。
	ErrGatewayConflict = errors.New("gateway conflict")
)

var gatewayNamePattern = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// Gateway 表示  兼容 `gateway` 表中的运营网关配置。
//
// 网关配置影响 CTI 选号、ESL originate 网关名、并发控制和号码改写规则。管理端
// 写入后必须触发运行时同步；当前领域结果会明确返回 SyncRequired，后续可接入 MQ
// 或 ESL gateway sync adapter。
type Gateway struct {
	ID                    int      `json:"id,omitempty"`
	Name                  string   `json:"name"`
	PreName               string   `json:"preName,omitempty"`
	Description           string   `json:"description"`
	ChannelID             int      `json:"channelId"`
	Concurrency           int      `json:"concurrency"`
	Model                 int      `json:"model"`
	Username              string   `json:"username,omitempty"`
	Password              string   `json:"-"`
	Realm                 string   `json:"realm"`
	Port                  string   `json:"port"`
	Priority              int      `json:"priority"`
	Remark                string   `json:"remark,omitempty"`
	BroadcastTime         int      `json:"broadcastTime,omitempty"`
	BroadcastTimeFlag     bool     `json:"broadcastTimeFlag,omitempty"`
	CallerPrefix          string   `json:"callerPrefix,omitempty"`
	CallerPrefixFlag      bool     `json:"callerPrefixFlag,omitempty"`
	CalleePrefix          string   `json:"calleePrefix,omitempty"`
	CalleePrefixFlag      bool     `json:"calleePrefixFlag,omitempty"`
	CallerRewriteRule     string   `json:"callerRewriteRule,omitempty"`
	CalleeRewriteRule     string   `json:"calleeRewriteRule,omitempty"`
	SupplementRing        bool     `json:"supplementRing"`
	SupplementRingFile    string   `json:"supplementRingFile,omitempty"`
	CalleeNumberLimit     bool     `json:"calleeNumberLimit,omitempty"`
	CalleeNumberLimitType string   `json:"calleeNumberLimitType,omitempty"`
	CodecPrefs            string   `json:"codecPrefs,omitempty"`
	GatewayCode           []string `json:"gatewayCode,omitempty"`
	RateID                int      `json:"rateId"`
	Enable                bool     `json:"enable"`
	NumberPool            []int    `json:"numberPool,omitempty"`
}

// GatewayPageRequest 表示运营端网关分页查询条件。
type GatewayPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
	Enable     *bool  `json:"enable,omitempty"`
	ChannelID  int    `json:"channelId,omitempty"`
}

// GatewayPageResult 是  Page 语义的轻量兼容结果。
type GatewayPageResult struct {
	PageNumber int       `json:"pageNumber"`
	PageSize   int       `json:"pageSize"`
	Total      int64     `json:"total"`
	Records    []Gateway `json:"records"`
}

// GatewayMutationResult 描述网关配置写入后的同步要求。
type GatewayMutationResult struct {
	Gateway        Gateway `json:"gateway,omitempty"`
	SyncAction     string  `json:"syncAction"`
	SyncRequired   bool    `json:"syncRequired"`
	SyncDispatched bool    `json:"syncDispatched"`
}

// GatewayRepository 定义运营端网关配置仓储能力。
type GatewayRepository interface {
	Page(ctx context.Context, req GatewayPageRequest) (GatewayPageResult, error)
	GetByID(ctx context.Context, id int) (Gateway, error)
	ExistsNameOrDescription(ctx context.Context, name, description string, excludeID int) (bool, error)
	Save(ctx context.Context, gateway Gateway) (Gateway, error)
	Delete(ctx context.Context, ids []int) error
	BindPools(ctx context.Context, gatewayID int, poolIDs []int) error
	UnbindPools(ctx context.Context, gatewayID int) error
}

// GatewaySynchronizer 将管理端变更同步到 cc-call/ESL 运行时。
type GatewaySynchronizer interface {
	SyncGatewayConfig(ctx context.Context, action string, gateway Gateway) error
}

// GatewayCacheInvalidator 用于清理 CTI 选号候选缓存。
type GatewayCacheInvalidator interface {
	InvalidateCandidateCache(ctx context.Context) error
}

// GatewayManagementService 承载运营端网关管理业务。
type GatewayManagementService struct {
	Repository   GatewayRepository
	Synchronizer GatewaySynchronizer
	Cache        GatewayCacheInvalidator
	Logger       *slog.Logger
}

// Page 分页查询未删除网关配置。
func (s *GatewayManagementService) Page(ctx context.Context, req GatewayPageRequest) (GatewayPageResult, error) {
	logger := s.logger()
	req = normalizeGatewayPage(req)
	logger.Info("运营端开始分页查询网关", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name, "channelId", req.ChannelID)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询网关失败", "error", err.Error())
		return GatewayPageResult{}, err
	}
	logger.Info("运营端分页查询网关完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Save 新增或更新网关配置，并返回需要同步 ESL 的动作。
func (s *GatewayManagementService) Save(ctx context.Context, gateway Gateway) (GatewayMutationResult, error) {
	logger := s.logger()
	normalized, err := normalizeGatewayForSave(gateway)
	if err != nil {
		logger.Warn("运营端保存网关参数无效", "id", gateway.ID, "name", gateway.Name, "error", err.Error())
		return GatewayMutationResult{}, err
	}
	exists, err := s.Repository.ExistsNameOrDescription(ctx, normalized.Name, normalized.Description, normalized.ID)
	if err != nil {
		logger.Error("运营端校验网关唯一性失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return GatewayMutationResult{}, err
	}
	if exists {
		logger.Warn("运营端保存网关冲突", "id", normalized.ID, "name", normalized.Name, "description", normalized.Description)
		return GatewayMutationResult{}, ErrGatewayConflict
	}

	action := "create"
	if normalized.ID > 0 {
		action = "update"
	}
	logger.Info("运营端开始保存网关", "id", normalized.ID, "name", normalized.Name, "action", action, "enable", normalized.Enable)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("运营端保存网关失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return GatewayMutationResult{}, err
	}
	if len(normalized.NumberPool) > 0 {
		if err := s.Repository.BindPools(ctx, saved.ID, normalized.NumberPool); err != nil {
			logger.Error("运营端绑定网关号码池失败", "gatewayId", saved.ID, "poolCount", len(normalized.NumberPool), "error", err.Error())
			return GatewayMutationResult{}, err
		}
	} else if action == "update" {
		if err := s.Repository.UnbindPools(ctx, saved.ID); err != nil {
			logger.Error("运营端解绑网关号码池失败", "gatewayId", saved.ID, "error", err.Error())
			return GatewayMutationResult{}, err
		}
	}
	result := GatewayMutationResult{Gateway: saved, SyncAction: action, SyncRequired: true}
	if dispatched, err := s.dispatchGatewaySync(ctx, action, saved); err != nil {
		if action == "update" {
			return GatewayMutationResult{}, err
		}
		logger.Warn("运营端网关运行时同步失败，已保留配置变更并等待补偿", "id", saved.ID, "name", saved.Name, "syncAction", action, "error", err.Error())
	} else {
		result.SyncDispatched = dispatched
	}
	if err := s.invalidateCache(ctx); err != nil {
		logger.Warn("运营端网关候选缓存失效失败", "id", saved.ID, "name", saved.Name, "error", err.Error())
	}
	logger.Info("运营端保存网关完成", "id", saved.ID, "name", saved.Name, "syncAction", action, "syncRequired", true, "syncDispatched", result.SyncDispatched)
	return result, nil
}

// Delete 逻辑删除网关，并解绑号码池。
func (s *GatewayManagementService) Delete(ctx context.Context, gateways []Gateway) (GatewayMutationResult, error) {
	logger := s.logger()
	ids := make([]int, 0, len(gateways))
	for _, gateway := range gateways {
		if gateway.ID > 0 {
			ids = append(ids, gateway.ID)
		}
	}
	if len(ids) == 0 {
		return GatewayMutationResult{}, ErrInvalidGateway
	}
	logger.Info("运营端开始删除网关", "gatewayCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("运营端删除网关失败", "gatewayCount", len(ids), "error", err.Error())
		return GatewayMutationResult{}, err
	}
	for _, id := range ids {
		if err := s.Repository.UnbindPools(ctx, id); err != nil {
			logger.Error("运营端删除后解绑号码池失败", "gatewayId", id, "error", err.Error())
			return GatewayMutationResult{}, err
		}
	}
	dispatched := false
	for _, gateway := range gateways {
		if gateway.Name == "" {
			continue
		}
		ok, err := s.dispatchGatewaySync(ctx, "delete", gateway)
		if err != nil {
			logger.Warn("运营端删除网关运行时同步失败，已保留删除变更并等待补偿", "gatewayId", gateway.ID, "name", gateway.Name, "error", err.Error())
			continue
		}
		dispatched = dispatched || ok
	}
	if err := s.invalidateCache(ctx); err != nil {
		logger.Warn("运营端网关候选缓存失效失败", "gatewayCount", len(ids), "error", err.Error())
	}
	logger.Info("运营端删除网关完成", "gatewayCount", len(ids), "syncAction", "delete", "syncRequired", true, "syncDispatched", dispatched)
	return GatewayMutationResult{SyncAction: "delete", SyncRequired: true, SyncDispatched: dispatched}, nil
}

// Sync 手动触发单个网关的运行时同步。
//
// 该入口用于运营端“同步网关”按钮，不修改配置表，只把已保存的 gateway 配置推送到
// cc-call/ESL 运行时，并清理选号候选缓存。
func (s *GatewayManagementService) Sync(ctx context.Context, id int) (GatewayMutationResult, error) {
	logger := s.logger()
	if id <= 0 {
		return GatewayMutationResult{}, ErrInvalidGateway
	}
	logger.Info("运营端开始手动同步网关", "gatewayId", id)
	gateway, err := s.Repository.GetByID(ctx, id)
	if err != nil {
		logger.Error("运营端手动同步网关读取配置失败", "gatewayId", id, "error", err.Error())
		return GatewayMutationResult{}, err
	}
	dispatched, err := s.dispatchGatewaySync(ctx, "update", gateway)
	if err != nil {
		logger.Error("运营端手动同步网关失败", "gatewayId", id, "name", gateway.Name, "error", err.Error())
		return GatewayMutationResult{}, err
	}
	if err := s.invalidateCache(ctx); err != nil {
		logger.Warn("运营端手动同步网关后候选缓存失效失败", "gatewayId", id, "name", gateway.Name, "error", err.Error())
	}
	logger.Info("运营端手动同步网关完成", "gatewayId", id, "name", gateway.Name, "syncDispatched", dispatched)
	return GatewayMutationResult{Gateway: gateway, SyncAction: "sync", SyncRequired: true, SyncDispatched: dispatched}, nil
}

func (s *GatewayManagementService) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *GatewayManagementService) dispatchGatewaySync(ctx context.Context, action string, gateway Gateway) (bool, error) {
	if s.Synchronizer == nil {
		s.logger().Warn("运营端网关运行时同步未配置", "gatewayId", gateway.ID, "name", gateway.Name, "syncAction", action, "impact", "生产环境必须配置 cc-call 同步地址或补偿任务")
		return false, nil
	}
	if err := s.Synchronizer.SyncGatewayConfig(ctx, action, gateway); err != nil {
		return false, err
	}
	return true, nil
}

func (s *GatewayManagementService) invalidateCache(ctx context.Context) error {
	if s.Cache == nil {
		s.logger().Warn("运营端网关候选缓存未配置", "impact", "生产环境应配置 Redis 失效器")
		return nil
	}
	return s.Cache.InvalidateCandidateCache(ctx)
}

func normalizeGatewayPage(req GatewayPageRequest) GatewayPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}

func normalizeGatewayForSave(gateway Gateway) (Gateway, error) {
	gateway.Name = strings.TrimSpace(gateway.Name)
	gateway.Description = strings.TrimSpace(gateway.Description)
	gateway.Realm = strings.TrimSpace(gateway.Realm)
	gateway.Port = strings.TrimSpace(gateway.Port)
	if gateway.Name == "" || len(gateway.Name) > 10 || !gatewayNamePattern.MatchString(gateway.Name) {
		return Gateway{}, ErrInvalidGateway
	}
	if gateway.Description == "" || len([]rune(gateway.Description)) > 15 {
		return Gateway{}, ErrInvalidGateway
	}
	if gateway.ChannelID <= 0 || gateway.Concurrency <= 0 || gateway.Priority < 0 || gateway.RateID <= 0 {
		return Gateway{}, ErrInvalidGateway
	}
	if gateway.Model == 0 {
		gateway.Model = 2
	}
	if gateway.Model != 1 && gateway.Model != 2 {
		return Gateway{}, ErrInvalidGateway
	}
	if gateway.Realm == "" || gateway.Port == "" {
		return Gateway{}, ErrInvalidGateway
	}
	if len(gateway.GatewayCode) > 0 {
		gateway.CodecPrefs = strings.Join(gateway.GatewayCode, ",")
	}
	if gateway.BroadcastTime < 0 {
		gateway.BroadcastTime = 0
	}
	return gateway, nil
}
