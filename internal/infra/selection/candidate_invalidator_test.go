package selection

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisCandidateCacheInvalidatorDeletesKeys(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	if err := client.Set(context.Background(), "cti:phone_resource:user:7", "[]", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.Set(context.Background(), "cti:phone_resource:user:8", "[]", 0).Err(); err != nil {
		t.Fatal(err)
	}

	invalidator := &RedisCandidateCacheInvalidator{Client: client}
	if err := invalidator.InvalidateCandidateCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	if server.Exists("cti:phone_resource:user:7") || server.Exists("cti:phone_resource:user:8") {
		t.Fatal("expected keys deleted")
	}
}
