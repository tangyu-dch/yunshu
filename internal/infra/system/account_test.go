package system

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	authdomain "yunshu/internal/domain/auth"
	"yunshu/internal/domain/operate"
)

func TestConsoleAccountRepositoryEnsureDefaultsSeedsLoginAccounts(t *testing.T) {
	t.Parallel()

	db := openConsoleAccountTestDB(t)
	repo := NewConsoleAccountRepository(db, nil)
	if err := repo.EnsureDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}

	identity, err := repo.ResolveLoginIdentity(context.Background(), authdomain.LoginRequest{Username: "admin", Password: "admin123"})
	if err != nil {
		t.Fatal(err)
	}
	if identity.RoleID != "super_admin" || !identity.Internal || identity.DataScope != operate.DataScopeGlobal {
		t.Fatalf("unexpected admin identity: %+v", identity)
	}

	operator, err := repo.ResolveLoginIdentity(context.Background(), authdomain.LoginRequest{Username: "operator", Password: "operator123"})
	if err != nil {
		t.Fatal(err)
	}
	if operator.RoleID != "operate_lead" || operator.Internal || operator.DataScope != operate.DataScopeGlobal {
		t.Fatalf("unexpected operator identity: %+v", operator)
	}
}

func TestConsoleAccountRepositoryEnsureDefaultsDoesNotOverwriteExistingAccount(t *testing.T) {
	t.Parallel()

	db := openConsoleAccountTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("custom123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&ConsoleAccountModel{
		Username:     "admin",
		PasswordHash: string(hash),
		RoleID:       "operate_staff",
		AccountType:  operate.AccountTypeOperate,
		DataScope:    operate.DataScopeGlobal,
		Enable:       true,
		CreatedTime:  now,
		UpdatedTime:  now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	repo := NewConsoleAccountRepository(db, nil)
	if err := repo.EnsureDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}

	identity, err := repo.ResolveLoginIdentity(context.Background(), authdomain.LoginRequest{Username: "admin", Password: "custom123"})
	if err != nil {
		t.Fatal(err)
	}
	if identity.RoleID != "operate_staff" || identity.Internal {
		t.Fatalf("expected existing account to stay unchanged, got %+v", identity)
	}
	if _, err := repo.ResolveLoginIdentity(context.Background(), authdomain.LoginRequest{Username: "admin", Password: "admin123"}); err == nil {
		t.Fatal("expected seeded password not to overwrite existing account")
	}
}

func TestConsoleAccountRepositoryMerchantAdminUnique(t *testing.T) {
	t.Parallel()

	db := openConsoleAccountTestDB(t)
	repo := NewConsoleAccountRepository(db, nil)
	if _, err := repo.Save(context.Background(), operate.Account{
		Username:    "merchant-admin-1",
		Password:    "secret",
		MerchantID:  "1001",
		RoleID:      "merchant_admin",
		AccountType: operate.AccountTypeMerchantAdmin,
		DataScope:   operate.DataScopeMerchant,
		Enable:      true,
	}); err != nil {
		t.Fatal(err)
	}
	exists, err := repo.ActiveMerchantAdminExists(context.Background(), "1001", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected active merchant admin")
	}
}

func TestConsoleAccountRepositoryRejectsMerchantLoginWhenMerchantUnavailable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		merchant MerchantModel
	}{
		{
			name: "disabled",
			merchant: MerchantModel{
				ID:     1001,
				Name:   "停用商户",
				Enable: false,
			},
		},
		{
			name: "deleted",
			merchant: MerchantModel{
				ID:      1001,
				Name:    "删除商户",
				Enable:  true,
				DelFlag: true,
			},
		},
		{
			name: "expired",
			merchant: MerchantModel{
				ID:          1001,
				Name:        "过期商户",
				Enable:      true,
				ExpiredTime: ptrTime(time.Now().Add(-time.Hour)),
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db := openConsoleAccountTestDB(t)
			repo := NewConsoleAccountRepository(db, nil)
			if err := db.Create(&tc.merchant).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Save(context.Background(), operate.Account{
				Username:    "merchant-admin",
				Password:    "secret",
				MerchantID:  "1001",
				RoleID:      "merchant_admin",
				AccountType: operate.AccountTypeMerchantAdmin,
				DataScope:   operate.DataScopeMerchant,
				Enable:      true,
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.ResolveLoginIdentity(context.Background(), authdomain.LoginRequest{Username: "merchant-admin", Password: "secret"}); err == nil {
				t.Fatal("expected merchant login to be rejected")
			}
		})
	}
}

func TestConsoleAccountRepositoryAllowsMerchantLoginWhenMerchantActive(t *testing.T) {
	t.Parallel()

	db := openConsoleAccountTestDB(t)
	repo := NewConsoleAccountRepository(db, nil)
	if err := db.Create(&MerchantModel{ID: 1001, Name: "启用商户", Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Save(context.Background(), operate.Account{
		Username:    "merchant-admin",
		Password:    "secret",
		MerchantID:  "1001",
		RoleID:      "merchant_admin",
		AccountType: operate.AccountTypeMerchantAdmin,
		DataScope:   operate.DataScopeMerchant,
		Enable:      true,
	}); err != nil {
		t.Fatal(err)
	}
	identity, err := repo.ResolveLoginIdentity(context.Background(), authdomain.LoginRequest{Username: "merchant-admin", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if identity.MerchantID != "1001" || identity.RoleID != "merchant_admin" {
		t.Fatalf("unexpected identity: %+v", identity)
	}
}

func openConsoleAccountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ConsoleAccountModel{}, &MerchantModel{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
