package contracts

import "testing"

func TestRoutesFor(t *testing.T) {
	t.Parallel()

	routes := RoutesFor(ServiceCall)
	if len(routes) == 0 {
		t.Fatal("expected ESL routes")
	}
	for _, route := range routes {
		if route.Service != ServiceCall {
			t.Fatalf("unexpected service %q", route.Service)
		}
		if route.PathPrefix == "" {
			t.Fatalf("route %s has empty path prefix", route.Controller)
		}
	}
}

func TestContractRegistriesAreDocumented(t *testing.T) {
	t.Parallel()

	for _, item := range RedisContracts {
		if item.KeyPattern == "" || item.Owner == "" || item.IdempotencyRole == "" {
			t.Fatalf("redis contract is incomplete: %+v", item)
		}
	}
	for _, item := range MQContracts {
		if item.Name == "" || item.AckTiming == "" || item.IdempotencyKey == "" {
			t.Fatalf("mq contract is incomplete: %+v", item)
		}
	}
}
