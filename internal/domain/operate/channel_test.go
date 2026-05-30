package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestChannelManagementService(t *testing.T) {
	t.Parallel()

	repo := newFakeChannelRepository()
	service := &operate.ChannelManagementService{Repository: repo}

	// 1. 正常保存渠道
	channel, err := service.Save(context.Background(), operate.Channel{
		Name:   "测试渠道",
		Enable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if channel.ID == 0 {
		t.Fatalf("expected positive channel ID")
	}

	// 2. 唯一性冲突校验
	_, err = service.Save(context.Background(), operate.Channel{
		Name: "测试渠道",
	})
	if !errors.Is(err, operate.ErrChannelConflict) {
		t.Fatalf("expected ErrChannelConflict, got %v", err)
	}

	// 3. 空参数校验
	_, err = service.Save(context.Background(), operate.Channel{
		Name: "", // 空名称
	})
	if !errors.Is(err, operate.ErrInvalidChannel) {
		t.Fatalf("expected ErrInvalidChannel, got %v", err)
	}

	// 4. 分页查询
	page, err := service.Page(context.Background(), operate.ChannelPageRequest{
		PageNumber: 1,
		PageSize:   10,
		Name:       "测试",
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", page)
	}

	// 5. 批量删除
	err = service.Delete(context.Background(), []operate.Channel{channel})
	if err != nil {
		t.Fatal(err)
	}

	// 验证删除
	_, err = repo.GetByID(context.Background(), channel.ID)
	if !errors.Is(err, operate.ErrChannelNotFound) {
		t.Fatalf("expected ErrChannelNotFound, got %v", err)
	}

	// 无效删除校验
	err = service.Delete(context.Background(), []operate.Channel{})
	if !errors.Is(err, operate.ErrInvalidChannel) {
		t.Fatalf("expected ErrInvalidChannel for empty delete, got %v", err)
	}
}

// fakeChannelRepository
type fakeChannelRepository struct {
	channels map[int]operate.Channel
	nextID   int
}

func newFakeChannelRepository() *fakeChannelRepository {
	return &fakeChannelRepository{channels: make(map[int]operate.Channel), nextID: 1}
}

func (r *fakeChannelRepository) Page(_ context.Context, req operate.ChannelPageRequest) (operate.ChannelPageResult, error) {
	list := make([]operate.Channel, 0)
	for _, v := range r.channels {
		if req.Name != "" && !strings.Contains(v.Name, req.Name) {
			continue
		}
		list = append(list, v)
	}
	return operate.ChannelPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(list)),
		Records:    list,
	}, nil
}

func (r *fakeChannelRepository) GetByID(_ context.Context, id int) (operate.Channel, error) {
	channel, ok := r.channels[id]
	if !ok {
		return operate.Channel{}, operate.ErrChannelNotFound
	}
	return channel, nil
}

func (r *fakeChannelRepository) ExistsName(_ context.Context, name string, excludeID int) (bool, error) {
	for k, v := range r.channels {
		if k == excludeID {
			continue
		}
		if v.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeChannelRepository) Save(_ context.Context, channel operate.Channel) (operate.Channel, error) {
	if channel.ID == 0 {
		channel.ID = r.nextID
		r.nextID++
	}
	r.channels[channel.ID] = channel
	return channel, nil
}

func (r *fakeChannelRepository) Delete(_ context.Context, ids []int) error {
	removed := 0
	for _, id := range ids {
		if _, ok := r.channels[id]; ok {
			delete(r.channels, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrChannelNotFound
	}
	return nil
}
