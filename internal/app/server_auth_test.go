package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	authdomain "yunshu/internal/domain/auth"
	"yunshu/internal/infra/config"
)

func TestConsoleTenantMiddlewareInjectsTenantContext(t *testing.T) {
	t.Parallel()

	server, err := NewServerWithConfig(contracts.ServiceConsole, config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	token, err := server.console.Auth.Login(context.Background(), authdomain.LoginRequest{
		Username:   "admin",
		Password:   "admin123",
		MerchantID: "12",
		UserID:     "34",
		RoleID:     "56",
		Internal:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	server.gin.GET("/tenant-check", func(c *gin.Context) {
		tenant, ok := contracts.TenantFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusOK, contracts.Fail(contracts.CodeNotFound, "tenant missing"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(tenant))
	})

	req := httptest.NewRequest(http.MethodGet, "/tenant-check", nil)
	req.Header.Set("Authorization", "Bearer "+token.Token)
	rec := httptest.NewRecorder()
	server.gin.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestConsoleProtectedRoutesRequireToken(t *testing.T) {
	t.Parallel()

	server, err := NewServerWithConfig(contracts.ServiceConsole, config.Config{})
	if err != nil {
		t.Fatal(err)
	}

	unauthorizedReq := httptest.NewRequest(http.MethodGet, "/operate/gateway?pageNumber=1&pageSize=10", nil)
	unauthorizedRec := httptest.NewRecorder()
	server.gin.ServeHTTP(unauthorizedRec, unauthorizedReq)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d body %s", unauthorizedRec.Code, unauthorizedRec.Body.String())
	}

	token, err := server.console.Auth.Login(context.Background(), authdomain.LoginRequest{
		Username:   "admin",
		Password:   "admin123",
		MerchantID: "",
		UserID:     "9999",
		RoleID:     "super_admin",
		Internal:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	authorizedReq := httptest.NewRequest(http.MethodGet, "/operate/gateway?pageNumber=1&pageSize=10", nil)
	authorizedReq.Header.Set("Authorization", "Bearer "+token.Token)
	authorizedRec := httptest.NewRecorder()
	server.gin.ServeHTTP(authorizedRec, authorizedReq)
	if authorizedRec.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body %s", authorizedRec.Code, authorizedRec.Body.String())
	}
}

func TestConsolePermissionMiddlewareEnforcesRoutePermissions(t *testing.T) {
	t.Parallel()

	server, err := NewServerWithConfig(contracts.ServiceConsole, config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	server.console.Auth.IdentityResolver = nil

	deniedToken, err := server.console.Auth.Login(context.Background(), authdomain.LoginRequest{
		Username:   "merchant",
		Password:   "merchant123",
		MerchantID: "1001",
		UserID:     "2001",
		RoleID:     "merchant_user",
		Internal:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	deniedReq := httptest.NewRequest(http.MethodGet, "/operate/gateway?pageNumber=1&pageSize=10", nil)
	deniedReq.Header.Set("Authorization", "Bearer "+deniedToken.Token)
	deniedRec := httptest.NewRecorder()
	server.gin.ServeHTTP(deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body %s", deniedRec.Code, deniedRec.Body.String())
	}

	allowedToken, err := server.console.Auth.Login(context.Background(), authdomain.LoginRequest{
		Username:   "merchant",
		Password:   "merchant123",
		MerchantID: "1001",
		UserID:     "2001",
		RoleID:     "merchant_admin",
		Internal:   false,
		Permissions: []string{
			string(contracts.PermissionOperateGatewayRead),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	allowedReq := httptest.NewRequest(http.MethodGet, "/operate/gateway?pageNumber=1&pageSize=10", nil)
	allowedReq.Header.Set("Authorization", "Bearer "+allowedToken.Token)
	allowedRec := httptest.NewRecorder()
	server.gin.ServeHTTP(allowedRec, allowedReq)
	if allowedRec.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body %s", allowedRec.Code, allowedRec.Body.String())
	}
}

func TestConsolePermissionLookupUsesDynamicRoutesFirst(t *testing.T) {
	t.Parallel()

	server, err := NewServerWithConfig(contracts.ServiceConsole, config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	server.console.RoutePermissions = fakeRoutePermissionResolver{permission: contracts.PermissionOperateGatewaySync}
	permission, found, err := server.requiredConsolePermission(context.Background(), "/operate/gateway/sync/7", http.MethodPost)
	if err != nil {
		t.Fatal(err)
	}
	if !found || permission != contracts.PermissionOperateGatewaySync {
		t.Fatalf("expected dynamic gateway sync permission, found=%v permission=%s", found, permission)
	}
}

func TestConsoleAuthUsesRedisSessionStoreAcrossInstances(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	cfg := config.Config{Redis: config.RedisConfig{Addrs: []string{server.Addr()}}}
	first, err := NewServerWithConfig(contracts.ServiceConsole, cfg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewServerWithConfig(contracts.ServiceConsole, cfg)
	if err != nil {
		t.Fatal(err)
	}

	ticket, err := first.console.Auth.Login(context.Background(), authdomain.LoginRequest{
		Username:   "admin",
		Password:   "admin123",
		MerchantID: "12",
		UserID:     "34",
		RoleID:     "56",
		Internal:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	loaded, ok := second.console.Auth.Token(context.Background(), ticket.Token)
	if !ok || loaded.Token != ticket.Token || !loaded.Tenant.Internal {
		t.Fatalf("expected shared redis session, got %+v ok=%v", loaded, ok)
	}
}

type fakeRoutePermissionResolver struct {
	permission contracts.PermissionCode
}

func (r fakeRoutePermissionResolver) RequiredPermissionForRequest(context.Context, string, string) (contracts.PermissionCode, bool, error) {
	return r.permission, true, nil
}
