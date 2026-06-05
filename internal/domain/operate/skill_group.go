package operate

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"yunshu/internal/contracts"
)

var (
	// ErrInvalidSkillGroup 表示技能组参数无效。
	ErrInvalidSkillGroup = errors.New("invalid skill group")
	// ErrSkillGroupNotFound 表示技能组不存在。
	ErrSkillGroupNotFound = errors.New("skill group not found")
	// ErrSkillGroupConflict 表示技能组名称冲突。
	ErrSkillGroupConflict = errors.New("skill group conflict")
	// ErrSkillGroupReferenced 表示技能组已被活动任务引用，不能直接删除。
	ErrSkillGroupReferenced = errors.New("skill group referenced")
)

// SkillGroup 表示  兼容 `skill_group` 表中的技能组配置。
type SkillGroup struct {
	ID          int    `json:"id,omitempty"`
	Name        string `json:"name"`
	MerchantID  int    `json:"merchantId,omitempty"`
	Description string `json:"description,omitempty"`
	Enable      bool   `json:"enable"`
}

type SkillGroupPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
	MerchantID int    `json:"merchantId,omitempty"`
	Enable     *bool  `json:"enable,omitempty"`
}

type SkillGroupPageResult struct {
	PageNumber int          `json:"pageNumber"`
	PageSize   int          `json:"pageSize"`
	Total      int64        `json:"total"`
	Records    []SkillGroup `json:"records"`
}

type SkillGroupRepository interface {
	Page(ctx context.Context, req SkillGroupPageRequest) (SkillGroupPageResult, error)
	GetByID(ctx context.Context, id int) (SkillGroup, error)
	ExistsName(ctx context.Context, name string, merchantID int, excludeID int) (bool, error)
	Save(ctx context.Context, skillGroup SkillGroup) (SkillGroup, error)
	Delete(ctx context.Context, ids []int) error
	ReplaceUsers(ctx context.Context, skillGroupID int, userIDs []int) error
	ReplacePhones(ctx context.Context, skillGroupID int, phoneIDs []int) error
	UsersBySkillGroup(ctx context.Context, skillGroupID int) ([]int, error)
	PhonesBySkillGroup(ctx context.Context, skillGroupID int) ([]int, error)
	HasActiveTasks(ctx context.Context, ids []int) (bool, error)
}

type SkillGroupManagementService struct {
	Repository SkillGroupRepository
	Logger     *slog.Logger
}

func (s *SkillGroupManagementService) Page(ctx context.Context, req SkillGroupPageRequest) (SkillGroupPageResult, error) {
	logger := s.logger()
	req = normalizeSkillGroupPage(req)
	logger.Info("商户端开始分页查询技能组", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name, "merchantId", req.MerchantID)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("商户端分页查询技能组失败", "error", err.Error())
		return SkillGroupPageResult{}, err
	}
	logger.Info("商户端分页查询技能组完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

func (s *SkillGroupManagementService) Save(ctx context.Context, skillGroup SkillGroup) (SkillGroup, error) {
	logger := s.logger()
	normalized, err := normalizeSkillGroupForSave(ctx, skillGroup)
	if err != nil {
		logger.Warn("商户端保存技能组参数无效", "id", skillGroup.ID, "name", skillGroup.Name, "error", err.Error())
		return SkillGroup{}, err
	}
	exists, err := s.Repository.ExistsName(ctx, normalized.Name, normalized.MerchantID, normalized.ID)
	if err != nil {
		logger.Error("商户端校验技能组唯一性失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return SkillGroup{}, err
	}
	if exists {
		logger.Warn("商户端保存技能组冲突", "id", normalized.ID, "name", normalized.Name, "merchantId", normalized.MerchantID)
		return SkillGroup{}, ErrSkillGroupConflict
	}
	logger.Info("商户端开始保存技能组", "id", normalized.ID, "name", normalized.Name, "merchantId", normalized.MerchantID, "enable", normalized.Enable)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("商户端保存技能组失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return SkillGroup{}, err
	}
	logger.Info("商户端保存技能组完成", "id", saved.ID, "name", saved.Name, "merchantId", saved.MerchantID)
	return saved, nil
}

func (s *SkillGroupManagementService) Delete(ctx context.Context, skillGroups []SkillGroup) error {
	logger := s.logger()
	ids := filterPositiveSkillGroupIDs(skillGroups)
	if len(ids) == 0 {
		return ErrInvalidSkillGroup
	}
	referenced, err := s.Repository.HasActiveTasks(ctx, ids)
	if err != nil {
		logger.Error("商户端删除技能组前检查任务引用失败", "skillGroupCount", len(ids), "error", err.Error())
		return err
	}
	if referenced {
		logger.Warn("商户端删除技能组失败，技能组仍被运行中的外呼任务引用", "skillGroupCount", len(ids))
		return ErrSkillGroupReferenced
	}
	logger.Info("商户端开始删除技能组", "skillGroupCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("商户端删除技能组失败", "skillGroupCount", len(ids), "error", err.Error())
		return err
	}
	logger.Info("商户端删除技能组完成", "skillGroupCount", len(ids))
	return nil
}

func (s *SkillGroupManagementService) UsersBySkillGroup(ctx context.Context, skillGroupID int) ([]int, error) {
	logger := s.logger()
	logger.Info("商户端开始查询技能组用户", "skillGroupId", skillGroupID)
	ids, err := s.Repository.UsersBySkillGroup(ctx, skillGroupID)
	if err != nil {
		logger.Error("商户端查询技能组用户失败", "skillGroupId", skillGroupID, "error", err.Error())
		return nil, err
	}
	logger.Info("商户端查询技能组用户完成", "skillGroupId", skillGroupID, "userCount", len(ids))
	return ids, nil
}

func (s *SkillGroupManagementService) PhonesBySkillGroup(ctx context.Context, skillGroupID int) ([]int, error) {
	logger := s.logger()
	logger.Info("商户端开始查询技能组号码", "skillGroupId", skillGroupID)
	ids, err := s.Repository.PhonesBySkillGroup(ctx, skillGroupID)
	if err != nil {
		logger.Error("商户端查询技能组号码失败", "skillGroupId", skillGroupID, "error", err.Error())
		return nil, err
	}
	logger.Info("商户端查询技能组号码完成", "skillGroupId", skillGroupID, "phoneCount", len(ids))
	return ids, nil
}

func (s *SkillGroupManagementService) ReplaceUsers(ctx context.Context, skillGroupID int, userIDs []int) error {
	logger := s.logger()
	userIDs = filterPositiveIDs(userIDs)
	if skillGroupID <= 0 {
		return ErrInvalidSkillGroup
	}
	logger.Info("商户端开始替换技能组用户", "skillGroupId", skillGroupID, "userCount", len(userIDs))
	if err := s.Repository.ReplaceUsers(ctx, skillGroupID, userIDs); err != nil {
		logger.Error("商户端替换技能组用户失败", "skillGroupId", skillGroupID, "error", err.Error())
		return err
	}
	logger.Info("商户端替换技能组用户完成", "skillGroupId", skillGroupID, "userCount", len(userIDs))
	return nil
}

func (s *SkillGroupManagementService) ReplacePhones(ctx context.Context, skillGroupID int, phoneIDs []int) error {
	logger := s.logger()
	phoneIDs = filterPositiveIDs(phoneIDs)
	if skillGroupID <= 0 {
		return ErrInvalidSkillGroup
	}
	logger.Info("商户端开始替换技能组号码", "skillGroupId", skillGroupID, "phoneCount", len(phoneIDs))
	if err := s.Repository.ReplacePhones(ctx, skillGroupID, phoneIDs); err != nil {
		logger.Error("商户端替换技能组号码失败", "skillGroupId", skillGroupID, "error", err.Error())
		return err
	}
	logger.Info("商户端替换技能组号码完成", "skillGroupId", skillGroupID, "phoneCount", len(phoneIDs))
	return nil
}

func (s *SkillGroupManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeSkillGroupPage(req SkillGroupPageRequest) SkillGroupPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}

func normalizeSkillGroupForSave(ctx context.Context, skillGroup SkillGroup) (SkillGroup, error) {
	skillGroup.Name = strings.TrimSpace(skillGroup.Name)
	skillGroup.Description = strings.TrimSpace(skillGroup.Description)
	if skillGroup.Name == "" {
		return SkillGroup{}, ErrInvalidSkillGroup
	}
	if skillGroup.MerchantID <= 0 {
		if tenant, ok := contracts.TenantFromContext(ctx); ok {
			if merchantID, err := strconv.Atoi(strings.TrimSpace(tenant.MerchantID)); err == nil && merchantID > 0 {
				skillGroup.MerchantID = merchantID
			}
		}
	}
	if skillGroup.MerchantID <= 0 {
		return SkillGroup{}, ErrInvalidSkillGroup
	}
	return skillGroup, nil
}

func filterPositiveSkillGroupIDs(skillGroups []SkillGroup) []int {
	ids := make([]int, 0, len(skillGroups))
	for _, skillGroup := range skillGroups {
		if skillGroup.ID > 0 {
			ids = append(ids, skillGroup.ID)
		}
	}
	return ids
}
