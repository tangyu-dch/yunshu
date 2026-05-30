package system

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/contracts"
	authdomain "yunshu/internal/domain/auth"
)

func TestPermissionRepositoryResolveLoginPermissions(t *testing.T) {
	t.Parallel()

	db := openPermissionTestDB(t)
	repo := NewPermissionRepository(db, nil)
	if err := db.Create(&ConsoleRoleModel{Code: "operate_sync", Name: "网关同步角色", Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ConsolePermissionModel{Code: string(contracts.PermissionOperateGatewaySync), Name: "网关同步", Module: "operate", Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ConsoleRolePermissionModel{RoleID: "operate_sync", PermissionCode: string(contracts.PermissionOperateGatewaySync), Enable: true}).Error; err != nil {
		t.Fatal(err)
	}

	permissions, ok, err := repo.ResolveLoginPermissions(context.Background(), authdomain.LoginRequest{RoleID: "operate_sync"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || len(permissions) != 1 || permissions[0] != string(contracts.PermissionOperateGatewaySync) {
		t.Fatalf("unexpected permissions: ok=%v permissions=%v", ok, permissions)
	}
}

func TestPermissionRepositoryRequiredPermissionForRequest(t *testing.T) {
	t.Parallel()

	db := openPermissionTestDB(t)
	repo := NewPermissionRepository(db, nil)
	if err := db.Create(&ConsoleRoutePermissionModel{
		PathPrefix:     "/operate/gateway/sync/",
		Method:         "POST",
		PermissionCode: string(contracts.PermissionOperateGatewaySync),
		Enable:         true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	permission, ok, err := repo.RequiredPermissionForRequest(context.Background(), "/operate/gateway/sync/7", "POST")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || permission != contracts.PermissionOperateGatewaySync {
		t.Fatalf("unexpected route permission: ok=%v permission=%s", ok, permission)
	}
}

func TestPermissionRepositoryFallsBackWhenMissing(t *testing.T) {
	t.Parallel()

	db := openPermissionTestDB(t)
	repo := NewPermissionRepository(db, nil)
	permissions, ok, err := repo.ResolveLoginPermissions(context.Background(), authdomain.LoginRequest{RoleID: "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if ok || len(permissions) != 0 {
		t.Fatalf("expected no database hit, ok=%v permissions=%v", ok, permissions)
	}
}

func TestPermissionRepositoryEnsureDefaultsSeedsRolesAndRoutes(t *testing.T) {
	t.Parallel()

	db := openPermissionTestDB(t)
	repo := NewPermissionRepository(db, nil)
	if err := repo.EnsureDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}

	var roleCount int64
	if err := db.Model(&ConsoleRoleModel{}).Count(&roleCount).Error; err != nil {
		t.Fatal(err)
	}
	if roleCount == 0 {
		t.Fatal("expected default roles to be seeded")
	}

	permission, ok, err := repo.RequiredPermissionForRequest(context.Background(), "/operate/gateway/sync/7", "POST")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || permission != contracts.PermissionOperateGatewaySync {
		t.Fatalf("expected seeded route permission, ok=%v permission=%s", ok, permission)
	}
}

func openPermissionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ConsoleRoleModel{}, &ConsolePermissionModel{}, &ConsoleRolePermissionModel{}, &ConsoleRoutePermissionModel{}); err != nil {
		t.Fatal(err)
	}
	return db
}
