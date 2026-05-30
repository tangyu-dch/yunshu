package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestGatewayManagementServiceSavePageAndDelete(t *testing.T) {
	t.Parallel()

	syncer := &fakeGatewaySynchronizer{}
	cache := &fakeGatewayCacheInvalidator{}
	service := &GatewayManagementService{Repository: newFakeGatewayRepository(), Synchronizer: syncer, Cache: cache, Logger: slog.Default()}
	result, err := service.Save(context.Background(), validGateway())
	if err != nil {
		t.Fatal(err)
	}
	if !result.SyncRequired || !result.SyncDispatched || result.SyncAction != "create" || result.Gateway.ID == 0 {
		t.Fatalf("unexpected mutation result: %+v", result)
	}

	page, err := service.Page(context.Background(), GatewayPageRequest{PageNumber: 1, PageSize: 10, Name: "gw"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page: %+v", page)
	}

	deleted, err := service.Delete(context.Background(), []Gateway{{ID: result.Gateway.ID, Name: result.Gateway.Name}})
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.SyncRequired || !deleted.SyncDispatched || deleted.SyncAction != "delete" {
		t.Fatalf("unexpected delete result: %+v", deleted)
	}
	if syncer.count != 2 {
		t.Fatalf("expected two sync calls, got %d", syncer.count)
	}
	if cache.count != 2 {
		t.Fatalf("expected two cache invalidations, got %d", cache.count)
	}
}

func TestGatewayManagementServiceRejectsConflict(t *testing.T) {
	t.Parallel()

	service := &GatewayManagementService{Repository: newFakeGatewayRepository(), Logger: slog.Default()}
	if _, err := service.Save(context.Background(), validGateway()); err != nil {
		t.Fatal(err)
	}
	_, err := service.Save(context.Background(), validGateway())
	if !errors.Is(err, ErrGatewayConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestGatewayManagementServiceRejectsInvalidGateway(t *testing.T) {
	t.Parallel()

	service := &GatewayManagementService{Repository: newFakeGatewayRepository(), Logger: slog.Default()}
	_, err := service.Save(context.Background(), Gateway{Name: "无效", Description: "描述"})
	if !errors.Is(err, ErrInvalidGateway) {
		t.Fatalf("expected invalid gateway, got %v", err)
	}
}

func validGateway() Gateway {
	return Gateway{
		Name:           "gw1",
		Description:    "网关一",
		ChannelID:      1,
		Concurrency:    10,
		Model:          2,
		Realm:          "10.0.0.1",
		Port:           "5060",
		Priority:       1,
		SupplementRing: false,
		RateID:         1,
		Enable:         true,
		GatewayCode:    []string{"PCMU", "PCMA"},
		NumberPool:     []int{100, 200},
	}
}

type fakeGatewayRepository struct {
	nextID   int
	gateways map[int]Gateway
}

func newFakeGatewayRepository() *fakeGatewayRepository {
	return &fakeGatewayRepository{nextID: 1, gateways: map[int]Gateway{}}
}

func (r *fakeGatewayRepository) Page(_ context.Context, req GatewayPageRequest) (GatewayPageResult, error) {
	records := make([]Gateway, 0, len(r.gateways))
	for _, gateway := range r.gateways {
		if req.Name != "" && !strings.Contains(gateway.Name, req.Name) && !strings.Contains(gateway.Description, req.Name) {
			continue
		}
		records = append(records, gateway)
	}
	return GatewayPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: int64(len(records)), Records: records}, nil
}

func (r *fakeGatewayRepository) GetByID(_ context.Context, id int) (Gateway, error) {
	gateway, ok := r.gateways[id]
	if !ok {
		return Gateway{}, ErrGatewayNotFound
	}
	return gateway, nil
}

func (r *fakeGatewayRepository) ExistsNameOrDescription(_ context.Context, name, description string, excludeID int) (bool, error) {
	for id, gateway := range r.gateways {
		if id == excludeID {
			continue
		}
		if gateway.Name == name || gateway.Description == description {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeGatewayRepository) Save(_ context.Context, gateway Gateway) (Gateway, error) {
	if gateway.ID == 0 {
		gateway.ID = r.nextID
		r.nextID++
	}
	r.gateways[gateway.ID] = gateway
	return gateway, nil
}

func (r *fakeGatewayRepository) Delete(_ context.Context, ids []int) error {
	for _, id := range ids {
		if _, ok := r.gateways[id]; !ok {
			return ErrGatewayNotFound
		}
		delete(r.gateways, id)
	}
	return nil
}

func (r *fakeGatewayRepository) BindPools(_ context.Context, _ int, _ []int) error {
	return nil
}

func (r *fakeGatewayRepository) UnbindPools(_ context.Context, _ int) error {
	return nil
}

type fakeGatewaySynchronizer struct {
	count int
}

func (s *fakeGatewaySynchronizer) SyncGatewayConfig(_ context.Context, _ string, _ Gateway) error {
	s.count++
	return nil
}

type fakeGatewayCacheInvalidator struct {
	count int
}

func (i *fakeGatewayCacheInvalidator) InvalidateCandidateCache(context.Context) error {
	i.count++
	return nil
}
