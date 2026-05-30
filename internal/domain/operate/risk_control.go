package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

var (
	// ErrInvalidRiskControl 表示风控策略参数无效。
	ErrInvalidRiskControl = errors.New("invalid risk control")
	// ErrRiskControlNotFound 表示风控策略不存在。
	ErrRiskControlNotFound = errors.New("risk control not found")
	// ErrRiskControlConflict 表示风控策略冲突或重名。
	ErrRiskControlConflict = errors.New("risk control conflict")
)

// RiskControl 表示  兼容 `risk_control` 表中的风控配置。
type RiskControl struct {
	ID                  int    `json:"id,omitempty"`
	Name                string `json:"name"`
	Remark              string `json:"remark,omitempty"`
	BlackLevelFlag      bool   `json:"blackLevelFlag"`
	BlackLevel          string `json:"blackLevel,omitempty"`
	BlindAreaFlag       bool   `json:"blindAreaFlag"`
	BlindArea           string `json:"blindArea,omitempty"`
	CalleeFrequencyFlag bool   `json:"calleeFrequencyFlag"`
	CalleeFrequency     string `json:"calleeFrequency,omitempty"`
}

// RiskControlMerchant 表示 `risk_control_merchant` 表中的商户风控绑定关系。
type RiskControlMerchant struct {
	RiskID     int  `json:"riskId"`
	MerchantID int  `json:"merchantId"`
	Enable     bool `json:"enable"`
}

type RiskControlPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
}

type RiskControlPageResult struct {
	PageNumber int           `json:"pageNumber"`
	PageSize   int           `json:"pageSize"`
	Total      int64         `json:"total"`
	Records    []RiskControl `json:"records"`
}

type RiskControlRepository interface {
	Page(ctx context.Context, req RiskControlPageRequest) (RiskControlPageResult, error)
	GetByID(ctx context.Context, id int) (RiskControl, error)
	ExistsName(ctx context.Context, name string, excludeID int) (bool, error)
	Save(ctx context.Context, rc RiskControl) (RiskControl, error)
	Delete(ctx context.Context, ids []int) error
	GetMerchants(ctx context.Context, riskID int) ([]RiskControlMerchant, error)
	SaveMerchants(ctx context.Context, riskID int, bindings []RiskControlMerchant) error
}

type RiskControlManagementService struct {
	Repository RiskControlRepository
	Logger     *slog.Logger
}

func (s *RiskControlManagementService) Page(ctx context.Context, req RiskControlPageRequest) (RiskControlPageResult, error) {
	logger := s.logger()
	req = normalizeRiskControlPage(req)
	logger.Info("运营端开始分页查询风控策略", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询风控策略失败", "error", err.Error())
		return RiskControlPageResult{}, err
	}
	logger.Info("运营端分页查询风控策略完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

func (s *RiskControlManagementService) GetByID(ctx context.Context, id int) (RiskControl, error) {
	logger := s.logger()
	logger.Info("运营端开始获取风控策略详情", "id", id)
	rc, err := s.Repository.GetByID(ctx, id)
	if err != nil {
		logger.Error("运营端获取风控策略详情失败", "id", id, "error", err.Error())
		return RiskControl{}, err
	}
	logger.Info("运营端获取风控策略详情完成", "id", id)
	return rc, nil
}

func (s *RiskControlManagementService) Save(ctx context.Context, rc RiskControl) (RiskControl, error) {
	logger := s.logger()
	normalized, err := normalizeRiskControlForSave(rc)
	if err != nil {
		logger.Warn("运营端保存风控策略参数无效", "id", rc.ID, "name", rc.Name, "error", err.Error())
		return RiskControl{}, err
	}
	exists, err := s.Repository.ExistsName(ctx, normalized.Name, normalized.ID)
	if err != nil {
		logger.Error("运营端校验风控策略唯一性失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return RiskControl{}, err
	}
	if exists {
		logger.Warn("运营端保存风控策略冲突", "id", normalized.ID, "name", normalized.Name)
		return RiskControl{}, ErrRiskControlConflict
	}
	logger.Info("运营端开始保存风控策略", "id", normalized.ID, "name", normalized.Name)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("运营端保存风控策略失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return RiskControl{}, err
	}
	logger.Info("运营端保存风控策略完成", "id", saved.ID, "name", saved.Name)
	return saved, nil
}

func (s *RiskControlManagementService) Delete(ctx context.Context, rcs []RiskControl) error {
	logger := s.logger()
	ids := filterPositiveRiskControlIDs(rcs)
	if len(ids) == 0 {
		return ErrInvalidRiskControl
	}
	logger.Info("运营端开始删除风控策略", "count", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("运营端删除风控策略失败", "count", len(ids), "error", err.Error())
		return err
	}
	logger.Info("运营端删除风控策略完成", "count", len(ids))
	return nil
}

func (s *RiskControlManagementService) GetMerchants(ctx context.Context, riskID int) ([]RiskControlMerchant, error) {
	logger := s.logger()
	logger.Info("运营端开始查询风控绑定的商户列表", "riskId", riskID)
	bindings, err := s.Repository.GetMerchants(ctx, riskID)
	if err != nil {
		logger.Error("运营端查询风控绑定商户失败", "riskId", riskID, "error", err.Error())
		return nil, err
	}
	logger.Info("运营端查询风控绑定商户完成", "riskId", riskID, "count", len(bindings))
	return bindings, nil
}

func (s *RiskControlManagementService) SaveMerchants(ctx context.Context, riskID int, bindings []RiskControlMerchant) error {
	logger := s.logger()
	logger.Info("运营端开始保存风控绑定商户关系", "riskId", riskID, "count", len(bindings))
	// 校验数据一致性
	for _, b := range bindings {
		if b.RiskID != riskID {
			logger.Warn("运营端保存风控绑定商户关系参数错误: 绑定的 riskId 不一致", "riskId", riskID, "bindingRiskId", b.RiskID)
			return ErrInvalidRiskControl
		}
	}
	if err := s.Repository.SaveMerchants(ctx, riskID, bindings); err != nil {
		logger.Error("运营端保存风控绑定商户关系失败", "riskId", riskID, "error", err.Error())
		return err
	}
	logger.Info("运营端保存风控绑定商户关系完成", "riskId", riskID)
	return nil
}

func (s *RiskControlManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeRiskControlPage(req RiskControlPageRequest) RiskControlPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}

func normalizeRiskControlForSave(rc RiskControl) (RiskControl, error) {
	rc.Name = strings.TrimSpace(rc.Name)
	rc.Remark = strings.TrimSpace(rc.Remark)
	if rc.Name == "" {
		return RiskControl{}, ErrInvalidRiskControl
	}
	// 对盲区及频次配置等进行空值规整
	rc.BlackLevel = strings.TrimSpace(rc.BlackLevel)
	rc.BlindArea = strings.TrimSpace(rc.BlindArea)
	rc.CalleeFrequency = strings.TrimSpace(rc.CalleeFrequency)
	return rc, nil
}

func filterPositiveRiskControlIDs(rcs []RiskControl) []int {
	ids := make([]int, 0, len(rcs))
	for _, rc := range rcs {
		if rc.ID > 0 {
			ids = append(ids, rc.ID)
		}
	}
	return ids
}
