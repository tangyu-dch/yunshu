package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/contracts"
)

func TestAuthServiceLoginTokenAndLogout(t *testing.T) {
	t.Parallel()

	service := &AuthService{Store: NewMemorySessionStore()}
	ticket, err := service.Login(context.Background(), LoginRequest{Username: "admin", Password: "admin123", Internal: true})
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Token == "" || !ticket.Tenant.Internal {
		t.Fatalf("unexpected ticket: %+v", ticket)
	}
	loaded, ok := service.Token(context.Background(), ticket.Token)
	if !ok || loaded.Token != ticket.Token {
		t.Fatalf("expected token to resolve")
	}
	if err := service.Logout(context.Background(), ticket.Token); err != nil {
		t.Fatal(err)
	}
	if _, ok := service.Token(context.Background(), ticket.Token); ok {
		t.Fatalf("expected token to be revoked")
	}
}

func TestAuthServiceRejectsInvalidLogin(t *testing.T) {
	t.Parallel()

	service := &AuthService{Store: NewMemorySessionStore()}
	_, err := service.Login(context.Background(), LoginRequest{})
	if !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("expected invalid login, got %v", err)
	}
}

func TestAuthServiceUsesDynamicPermissionResolver(t *testing.T) {
	t.Parallel()

	service := &AuthService{
		Store:       NewMemorySessionStore(),
		Permissions: fakeLoginPermissionResolver{permissions: []string{string(contracts.PermissionOperateGatewaySync)}},
	}
	ticket, err := service.Login(context.Background(), LoginRequest{Username: "admin", Password: "admin123", RoleID: "operate_sync"})
	if err != nil {
		t.Fatal(err)
	}
	if !ticket.Tenant.HasPermission(string(contracts.PermissionOperateGatewaySync)) {
		t.Fatalf("expected dynamic permission in token, got %+v", ticket.Tenant.Permissions)
	}
}

func TestRedisSessionStoreIssueGetRevoke(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	store := NewRedisSessionStore(client, "")
	ticket, err := store.Issue(context.Background(), contractsTenant("12", "34", "56", true), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	loaded, ok := store.Get(context.Background(), ticket.Token)
	if !ok || loaded.Token != ticket.Token || !loaded.Tenant.Internal {
		t.Fatalf("unexpected ticket: %+v ok=%v", loaded, ok)
	}
	if err := store.Revoke(context.Background(), ticket.Token); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Get(context.Background(), ticket.Token); ok {
		t.Fatalf("expected revoked token to be missing")
	}
}

func TestRedisSessionStoreExpiresWithTTL(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	store := NewRedisSessionStore(client, "")
	ticket, err := store.Issue(context.Background(), contractsTenant("12", "34", "56", true), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	server.FastForward(2 * time.Second)
	if _, ok := store.Get(context.Background(), ticket.Token); ok {
		t.Fatalf("expected expired token to be missing")
	}
}

func contractsTenant(merchantID, userID, roleID string, internal bool) contracts.TenantContext {
	return contracts.TenantContext{
		MerchantID: merchantID,
		UserID:     userID,
		RoleID:     roleID,
		Internal:   internal,
	}
}

type fakeLoginPermissionResolver struct {
	permissions []string
}

func (r fakeLoginPermissionResolver) ResolveLoginPermissions(context.Context, LoginRequest) ([]string, bool, error) {
	return r.permissions, true, nil
}
