package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

var (
	// ErrInvalidPhoneGroup 表示号码组参数无效。
	ErrInvalidPhoneGroup = errors.New("invalid phone group")
	// ErrPhoneGroupNotFound 表示号码组不存在。
	ErrPhoneGroupNotFound = errors.New("phone group not found")
	// ErrPhoneGroupConflict 表示号码组名称冲突。
	ErrPhoneGroupConflict = errors.New("phone group conflict")
)

// PhoneGroup 表示  兼容 `merchant_phone_group` 表中的号码组配置。
type PhoneGroup struct {
	ID         int    `json:"id,omitempty"`
	Name       string `json:"name"`
	Remark     string `json:"remark,omitempty"`
	Desc       string `json:"desc,omitempty"`
	MerchantID int    `json:"merchantId"`
	Enable     bool   `json:"enable"`
}

type PhoneGroupPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
	MerchantID int    `json:"merchantId,omitempty"`
	Enable     *bool  `json:"enable,omitempty"`
}

type PhoneGroupPageResult struct {
	PageNumber int          `json:"pageNumber"`
	PageSize   int          `json:"pageSize"`
	Total      int64        `json:"total"`
	Records    []PhoneGroup `json:"records"`
}

type PhoneGroupRepository interface {
	Page(ctx context.Context, req PhoneGroupPageRequest) (PhoneGroupPageResult, error)
	GetByID(ctx context.Context, id int) (PhoneGroup, error)
	ExistsName(ctx context.Context, name string, merchantID int, excludeID int) (bool, error)
	Save(ctx context.Context, group PhoneGroup) (PhoneGroup, error)
	Delete(ctx context.Context, ids []int) error
	ReplacePhones(ctx context.Context, groupID int, merchantID int, phoneIDs []int) error
	ReplaceSkillGroups(ctx context.Context, groupID int, merchantID int, skillGroupIDs []int) error
	PhonesByGroup(ctx context.Context, groupID int) ([]int, error)
	SkillGroupsByGroup(ctx context.Context, groupID int) ([]int, error)
}

type PhoneGroupManagementService struct {
	Repository PhoneGroupRepository
	Logger     *slog.Logger
}

func (s *PhoneGroupManagementService) Page(ctx context.Context, req PhoneGroupPageRequest) (PhoneGroupPageResult, error) {
	logger := s.logger()
	req = normalizePhoneGroupPage(req)
	logger.Info("商户端开始分页查询号码组", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name, "merchantId", req.MerchantID)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("商户端分页查询号码组失败", "error", err.Error())
		return PhoneGroupPageResult{}, err
	}
	logger.Info("商户端分页查询号码组完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

func (s *PhoneGroupManagementService) Save(ctx context.Context, group PhoneGroup) (PhoneGroup, error) {
	logger := s.logger()
	normalized, err := normalizePhoneGroupForSave(group)
	if err != nil {
		logger.Warn("商户端保存号码组参数无效", "id", group.ID, "name", group.Name, "error", err.Error())
		return PhoneGroup{}, err
	}
	exists, err := s.Repository.ExistsName(ctx, normalized.Name, normalized.MerchantID, normalized.ID)
	if err != nil {
		logger.Error("商户端校验号码组唯一性失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return PhoneGroup{}, err
	}
	if exists {
		logger.Warn("商户端保存号码组冲突", "id", normalized.ID, "name", normalized.Name, "merchantId", normalized.MerchantID)
		return PhoneGroup{}, ErrPhoneGroupConflict
	}
	logger.Info("商户端开始保存号码组", "id", normalized.ID, "name", normalized.Name, "merchantId", normalized.MerchantID, "enable", normalized.Enable)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("商户端保存号码组失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return PhoneGroup{}, err
	}
	logger.Info("商户端保存号码组完成", "id", saved.ID, "name", saved.Name, "merchantId", saved.MerchantID)
	return saved, nil
}

func (s *PhoneGroupManagementService) Delete(ctx context.Context, groups []PhoneGroup) error {
	logger := s.logger()
	ids := filterPositivePhoneGroupIDs(groups)
	if len(ids) == 0 {
		return ErrInvalidPhoneGroup
	}
	logger.Info("商户端开始删除号码组", "groupCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("商户端删除号码组失败", "groupCount", len(ids), "error", err.Error())
		return err
	}
	logger.Info("商户端删除号码组完成", "groupCount", len(ids))
	return nil
}

func (s *PhoneGroupManagementService) ReplacePhones(ctx context.Context, groupID int, merchantID int, phoneIDs []int) error {
	logger := s.logger()
	phoneIDs = filterPositiveIDs(phoneIDs)
	if groupID <= 0 || merchantID <= 0 {
		return ErrInvalidPhoneGroup
	}
	logger.Info("商户端开始替换号码组号码", "phoneGroupId", groupID, "merchantId", merchantID, "phoneCount", len(phoneIDs))
	if err := s.Repository.ReplacePhones(ctx, groupID, merchantID, phoneIDs); err != nil {
		logger.Error("商户端替换号码组号码失败", "phoneGroupId", groupID, "error", err.Error())
		return err
	}
	logger.Info("商户端替换号码组号码完成", "phoneGroupId", groupID, "phoneCount", len(phoneIDs))
	return nil
}

func (s *PhoneGroupManagementService) ReplaceSkillGroups(ctx context.Context, groupID int, merchantID int, skillGroupIDs []int) error {
	logger := s.logger()
	skillGroupIDs = filterPositiveIDs(skillGroupIDs)
	if groupID <= 0 || merchantID <= 0 {
		return ErrInvalidPhoneGroup
	}
	logger.Info("商户端开始替换号码组技能组", "phoneGroupId", groupID, "merchantId", merchantID, "skillGroupCount", len(skillGroupIDs))
	if err := s.Repository.ReplaceSkillGroups(ctx, groupID, merchantID, skillGroupIDs); err != nil {
		logger.Error("商户端替换号码组技能组失败", "phoneGroupId", groupID, "error", err.Error())
		return err
	}
	logger.Info("商户端替换号码组技能组完成", "phoneGroupId", groupID, "skillGroupCount", len(skillGroupIDs))
	return nil
}

func (s *PhoneGroupManagementService) PhonesByGroup(ctx context.Context, groupID int) ([]int, error) {
	logger := s.logger()
	logger.Info("商户端开始查询号码组号码", "phoneGroupId", groupID)
	ids, err := s.Repository.PhonesByGroup(ctx, groupID)
	if err != nil {
		logger.Error("商户端查询号码组号码失败", "phoneGroupId", groupID, "error", err.Error())
		return nil, err
	}
	return ids, nil
}

func (s *PhoneGroupManagementService) SkillGroupsByGroup(ctx context.Context, groupID int) ([]int, error) {
	logger := s.logger()
	logger.Info("商户端开始查询号码组技能组", "phoneGroupId", groupID)
	ids, err := s.Repository.SkillGroupsByGroup(ctx, groupID)
	if err != nil {
		logger.Error("商户端查询号码组技能组失败", "phoneGroupId", groupID, "error", err.Error())
		return nil, err
	}
	return ids, nil
}

func (s *PhoneGroupManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizePhoneGroupPage(req PhoneGroupPageRequest) PhoneGroupPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}

func normalizePhoneGroupForSave(group PhoneGroup) (PhoneGroup, error) {
	group.Name = strings.TrimSpace(group.Name)
	group.Remark = strings.TrimSpace(group.Remark)
	group.Desc = strings.TrimSpace(group.Desc)
	if group.Name == "" || group.MerchantID <= 0 {
		return PhoneGroup{}, ErrInvalidPhoneGroup
	}
	return group, nil
}

func filterPositivePhoneGroupIDs(groups []PhoneGroup) []int {
	ids := make([]int, 0, len(groups))
	for _, group := range groups {
		if group.ID > 0 {
			ids = append(ids, group.ID)
		}
	}
	return ids
}
