package contracts

import "testing"

func TestNewEventEnvelopeDefaults(t *testing.T) {
	t.Parallel()

	envelope := NewEventEnvelope("evt-1", "call.created", "cmd-1", "call", "call-1", ServiceCall, map[string]string{"callId": "call-1"})
	if envelope.EventVersion != 1 {
		t.Fatalf("event version got %d", envelope.EventVersion)
	}
	if envelope.OccurredAt.IsZero() {
		t.Fatal("occurred at should be set")
	}
	if envelope.Headers == nil {
		t.Fatal("headers should be initialized")
	}
}

func TestErrorContractsAreUnique(t *testing.T) {
	t.Parallel()

	seen := map[int]bool{}
	for _, item := range ErrorContracts {
		if seen[item.Code] {
			t.Fatalf("duplicate error code %d", item.Code)
		}
		seen[item.Code] = true
		if item.Key == "" || item.Message == "" || item.Owner == "" {
			t.Fatalf("incomplete error contract: %+v", item)
		}
	}
}

func TestTenantContext(t *testing.T) {
	t.Parallel()

	ctx := WithTenant(t.Context(), TenantContext{MerchantID: "m1", UserID: "u1"})
	tenant, ok := TenantFromContext(ctx)
	if !ok {
		t.Fatal("tenant missing from context")
	}
	if tenant.MerchantID != "m1" || tenant.UserID != "u1" {
		t.Fatalf("unexpected tenant: %+v", tenant)
	}
}
