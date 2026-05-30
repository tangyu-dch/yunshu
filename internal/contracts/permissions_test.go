package contracts

import "testing"

func TestRequiredPermissionForGatewaySync(t *testing.T) {
	t.Parallel()

	permission, ok := RequiredPermissionForRequest("/operate/gateway/sync/7", "POST")
	if !ok || permission != PermissionOperateGatewaySync {
		t.Fatalf("expected gateway sync permission, ok=%v permission=%s", ok, permission)
	}
}
