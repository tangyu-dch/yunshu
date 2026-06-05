package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/contracts"
)

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
