package extensionstatus

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/domain/esl"
)

func TestRedisReaderGetsExtensionStatus(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	reader := NewRedisReader(client)
	ctx := context.Background()

	server.HSet(redisExtensionStatusKey, "1001", "1")
	status, ok, err := reader.GetExtensionStatus(ctx, "1001")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || status != esl.ExtensionStatusIdle {
		t.Fatalf("unexpected status: %v ok=%v", status, ok)
	}
}

func TestRedisReaderMissingStatusIsOffline(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	reader := NewRedisReader(client)

	status, ok, err := reader.GetExtensionStatus(context.Background(), "1001")
	if err != nil {
		t.Fatal(err)
	}
	if ok || status != esl.ExtensionStatusOffline {
		t.Fatalf("unexpected missing status: %v ok=%v", status, ok)
	}
}
