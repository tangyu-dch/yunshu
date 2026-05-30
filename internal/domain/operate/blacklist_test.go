package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestBlacklistManagementSavePageAndDelete(t *testing.T) {
	t.Parallel()

	service := &operate.BlacklistManagementService{Repository: newFakeBlacklistRepository()}
	saved, err := service.Save(context.Background(), operate.Blacklist{
		Name:                "投诉黑名单",
		VerificationChannel: operate.BlacklistVerificationChannelDongXin,
		GatewayIDs:          []int{1, 2},
		Remark:              "重点拦截",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 {
		t.Fatalf("expected blacklist id")
	}
	page, err := service.Page(context.Background(), operate.BlacklistPageRequest{PageNumber: 1, PageSize: 10, Name: "投诉"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", page)
	}
	if err := service.Delete(context.Background(), saved.ID); err != nil {
		t.Fatal(err)
	}
}

func TestBlacklistManagementRejectsConflict(t *testing.T) {
	t.Parallel()

	service := &operate.BlacklistManagementService{Repository: newFakeBlacklistRepository()}
	blacklist := operate.Blacklist{Name: "投诉黑名单", VerificationChannel: operate.BlacklistVerificationChannelDongXin}
	if _, err := service.Save(context.Background(), blacklist); err != nil {
		t.Fatal(err)
	}
	_, err := service.Save(context.Background(), blacklist)
	if !errors.Is(err, operate.ErrBlacklistConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

type fakeBlacklistRepository struct {
	nextID     int
	blacklists map[int]operate.Blacklist
	channels   map[int]operate.BlacklistChannel
}

func newFakeBlacklistRepository() *fakeBlacklistRepository {
	return &fakeBlacklistRepository{
		nextID:     1,
		blacklists: map[int]operate.Blacklist{},
		channels: map[int]operate.BlacklistChannel{
			1: {Code: 1, Name: "东信易通黑名单", Vendor: "DONG_XIN", Enable: true},
			2: {Code: 2, Name: "羽乐黑名单", Vendor: "YU_LE", Enable: true},
		},
	}
}

func (r *fakeBlacklistRepository) Page(_ context.Context, req operate.BlacklistPageRequest) (operate.BlacklistPageResult, error) {
	records := make([]operate.Blacklist, 0, len(r.blacklists))
	for _, blacklist := range r.blacklists {
		if req.Name != "" && !strings.Contains(blacklist.Name, req.Name) {
			continue
		}
		records = append(records, blacklist)
	}
	return operate.BlacklistPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: int64(len(records)), Records: records}, nil
}

func (r *fakeBlacklistRepository) GetByID(_ context.Context, id int) (operate.Blacklist, error) {
	blacklist, ok := r.blacklists[id]
	if !ok {
		return operate.Blacklist{}, operate.ErrBlacklistNotFound
	}
	return blacklist, nil
}

func (r *fakeBlacklistRepository) ExistsName(_ context.Context, name string, excludeID int) (bool, error) {
	for id, blacklist := range r.blacklists {
		if id == excludeID {
			continue
		}
		if blacklist.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeBlacklistRepository) Save(_ context.Context, blacklist operate.Blacklist) (operate.Blacklist, error) {
	if blacklist.ID == 0 {
		blacklist.ID = r.nextID
		r.nextID++
	}
	r.blacklists[blacklist.ID] = blacklist
	return blacklist, nil
}

func (r *fakeBlacklistRepository) Delete(_ context.Context, id int) error {
	if _, ok := r.blacklists[id]; !ok {
		return operate.ErrBlacklistNotFound
	}
	delete(r.blacklists, id)
	return nil
}

func (r *fakeBlacklistRepository) PageNumbers(_ context.Context, req operate.BlacklistNumberPageRequest) (operate.BlacklistNumberPageResult, error) {
	return operate.BlacklistNumberPageResult{}, nil
}

func (r *fakeBlacklistRepository) SaveNumber(_ context.Context, num operate.BlacklistNumber) (operate.BlacklistNumber, error) {
	return num, nil
}

func (r *fakeBlacklistRepository) DeleteNumbers(_ context.Context, phones []string) error {
	return nil
}

func (r *fakeBlacklistRepository) ListChannels(ctx context.Context) ([]operate.BlacklistChannel, error) {
	result := make([]operate.BlacklistChannel, 0, len(r.channels))
	for _, item := range r.channels {
		result = append(result, item)
	}
	return result, nil
}

func (r *fakeBlacklistRepository) SaveChannel(ctx context.Context, channel operate.BlacklistChannel) error {
	r.channels[channel.Code] = channel
	return nil
}

func (r *fakeBlacklistRepository) DeleteChannel(ctx context.Context, code int) error {
	delete(r.channels, code)
	return nil
}
