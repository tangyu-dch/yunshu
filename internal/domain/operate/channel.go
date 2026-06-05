package operate

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
)

var (
	// ErrInvalidChannel 表示渠道参数无效。
	ErrInvalidChannel = errors.New("invalid channel")
	// ErrChannelNotFound 表示渠道不存在。
	ErrChannelNotFound = errors.New("channel not found")
	// ErrChannelConflict 表示渠道名称冲突。
	ErrChannelConflict = errors.New("channel conflict")
	// ErrChannelReferenced 表示渠道已被网关引用，不能直接删除。
	ErrChannelReferenced = errors.New("channel referenced")
)

// Channel 表示  兼容 `channel` 表中的渠道配置。
type Channel struct {
	ID        int             `json:"id,omitempty"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config,omitempty"`
	BlindArea json.RawMessage `json:"blindArea,omitempty"`
	Remark    string          `json:"remark,omitempty"`
	Enable    bool            `json:"enable"`
}

type ChannelPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
	Enable     *bool  `json:"enable,omitempty"`
}

type ChannelPageResult struct {
	PageNumber int       `json:"pageNumber"`
	PageSize   int       `json:"pageSize"`
	Total      int64     `json:"total"`
	Records    []Channel `json:"records"`
}

type ChannelRepository interface {
	Page(ctx context.Context, req ChannelPageRequest) (ChannelPageResult, error)
	GetByID(ctx context.Context, id int) (Channel, error)
	ExistsName(ctx context.Context, name string, excludeID int) (bool, error)
	Save(ctx context.Context, channel Channel) (Channel, error)
	Delete(ctx context.Context, ids []int) error
	HasBindings(ctx context.Context, ids []int) (bool, error)
}

type ChannelManagementService struct {
	Repository ChannelRepository
	Logger     *slog.Logger
}

func (s *ChannelManagementService) Page(ctx context.Context, req ChannelPageRequest) (ChannelPageResult, error) {
	logger := s.logger()
	req = normalizeChannelPage(req)
	logger.Info("运营端开始分页查询渠道", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询渠道失败", "error", err.Error())
		return ChannelPageResult{}, err
	}
	logger.Info("运营端分页查询渠道完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

func (s *ChannelManagementService) Save(ctx context.Context, channel Channel) (Channel, error) {
	logger := s.logger()
	normalized, err := normalizeChannelForSave(channel)
	if err != nil {
		logger.Warn("运营端保存渠道参数无效", "id", channel.ID, "name", channel.Name, "error", err.Error())
		return Channel{}, err
	}
	exists, err := s.Repository.ExistsName(ctx, normalized.Name, normalized.ID)
	if err != nil {
		logger.Error("运营端校验渠道唯一性失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return Channel{}, err
	}
	if exists {
		logger.Warn("运营端保存渠道冲突", "id", normalized.ID, "name", normalized.Name)
		return Channel{}, ErrChannelConflict
	}
	logger.Info("运营端开始保存渠道", "id", normalized.ID, "name", normalized.Name, "enable", normalized.Enable)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("运营端保存渠道失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return Channel{}, err
	}
	logger.Info("运营端保存渠道完成", "id", saved.ID, "name", saved.Name, "enable", saved.Enable)
	return saved, nil
}

func (s *ChannelManagementService) Delete(ctx context.Context, channels []Channel) error {
	logger := s.logger()
	ids := filterPositiveChannelIDs(channels)
	if len(ids) == 0 {
		return ErrInvalidChannel
	}
	referenced, err := s.Repository.HasBindings(ctx, ids)
	if err != nil {
		logger.Error("运营端删除渠道前检查引用失败", "channelCount", len(ids), "error", err.Error())
		return err
	}
	if referenced {
		logger.Warn("运营端删除渠道失败，渠道仍被网关引用", "channelCount", len(ids))
		return ErrChannelReferenced
	}
	logger.Info("运营端开始删除渠道", "channelCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("运营端删除渠道失败", "channelCount", len(ids), "error", err.Error())
		return err
	}
	logger.Info("运营端删除渠道完成", "channelCount", len(ids))
	return nil
}

func (s *ChannelManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeChannelPage(req ChannelPageRequest) ChannelPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}

func normalizeChannelForSave(channel Channel) (Channel, error) {
	channel.Name = strings.TrimSpace(channel.Name)
	channel.Remark = strings.TrimSpace(channel.Remark)
	if channel.Name == "" {
		return Channel{}, ErrInvalidChannel
	}
	if len(channel.Config) == 0 {
		channel.Config = nil
	}
	if len(channel.BlindArea) == 0 {
		channel.BlindArea = nil
	}
	return channel, nil
}

func filterPositiveChannelIDs(channels []Channel) []int {
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		if channel.ID > 0 {
			ids = append(ids, channel.ID)
		}
	}
	return ids
}
