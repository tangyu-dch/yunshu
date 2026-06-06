package auth

import (
	"context"
	"errors"
	"testing"

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

type fakeLoginPermissionResolver struct {
	permissions []string
}

func (r fakeLoginPermissionResolver) ResolveLoginPermissions(context.Context, LoginRequest) ([]string, bool, error) {
	return r.permissions, true, nil
}
