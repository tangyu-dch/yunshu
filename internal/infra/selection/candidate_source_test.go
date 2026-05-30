package selection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/domain/cti"
)

func TestRedisCandidateSourceCachesResults(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	source := &RedisCandidateSource{
		Client: client,
		Source: fakeCandidateSource{candidates: []cti.NumberCandidate{{Phone: "1001", GatewayID: "9"}}},
		TTL:    time.Minute,
	}

	first, err := source.CandidatesForUser(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.CandidatesForUser(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || first[0].Phone != second[0].Phone {
		t.Fatalf("unexpected candidates: first=%+v second=%+v", first, second)
	}
}

func TestRedisCandidateSourceRejectsInvalidUser(t *testing.T) {
	t.Parallel()

	source := &RedisCandidateSource{}
	if _, err := source.CandidatesForUser(context.Background(), 0); err == nil {
		t.Fatal("expected invalid user error")
	}
}

func TestRedisCandidateSourcePropagatesSourceError(t *testing.T) {
	t.Parallel()

	want := errors.New("boom")
	source := &RedisCandidateSource{Source: fakeCandidateSource{err: want}}
	if _, err := source.CandidatesForUser(context.Background(), 7); !errors.Is(err, want) {
		t.Fatalf("expected source error, got %v", err)
	}
}

type fakeCandidateSource struct {
	candidates []cti.NumberCandidate
	err        error
}

func (f fakeCandidateSource) CandidatesForUser(context.Context, int) ([]cti.NumberCandidate, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.candidates, nil
}
