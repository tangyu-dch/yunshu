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
	if err := db.AutoMigrate(&ConsoleAccountModel{}, &MerchantModel{}, &merchantUserModel{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func TestConsoleAccountRepositoryMerchantUserCascades(t *testing.T) {
	t.Parallel()

	db := openConsoleAccountTestDB(t)
	repo := NewConsoleAccountRepository(db, nil)

	// 1. 测试级联创建
	acc, err := repo.Save(context.Background(), operate.Account{
		Username:       "agent-001",
		Password:       "agent123",
		MerchantID:     "1001",
		RoleID:         "merchant_user",
		AccountType:    operate.AccountTypeMerchantUser,
		DataScope:      operate.DataScopeMerchant,
		Enable:         true,
		OrganizationID: 88,
		SeatNumber:     "8008",
	})
	if err != nil {
		t.Fatal(err)
	}

	if acc.ID == 0 || acc.UserID == "" {
		t.Fatalf("expected account ID and UserID to be populated, got ID: %d, UserID: %s", acc.ID, acc.UserID)
	}

	// 验证 cc_res_mch_user 级联写入
	var seat merchantUserModel
	if err := db.Where("id = ?", acc.ID).First(&seat).Error; err != nil {
		t.Fatal("expected cascading merchant_user record to be created:", err)
	}
	if seat.OrganizationID != 88 || seat.SeatNumber != "8008" || seat.Username != "agent-001" || !seat.Enable {
		t.Fatalf("unexpected cascading record values: %+v", seat)
	}

	// 2. 测试通过 GetByID 加载回填
	loaded, err := repo.GetByID(context.Background(), acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.OrganizationID != 88 || loaded.SeatNumber != "8008" {
		t.Fatalf("expected organizationId and seatNumber to be loaded, got org: %d, seat: %s", loaded.OrganizationID, loaded.SeatNumber)
	}

	// 3. 测试通过 Page 加载回填
	page, err := repo.Page(context.Background(), operate.AccountPageRequest{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rec := range page.Records {
		if rec.ID == acc.ID {
			found = true
			if rec.OrganizationID != 88 || rec.SeatNumber != "8008" {
				t.Fatalf("expected org/seat to be loaded in Page, got org: %d, seat: %s", rec.OrganizationID, rec.SeatNumber)
			}
		}
	}
	if !found {
		t.Fatal("expected created account to be found in page results")
	}

	// 4. 测试级联修改
	acc.OrganizationID = 99
	acc.SeatNumber = "9009"
	_, err = repo.Save(context.Background(), acc)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Where("id = ?", acc.ID).First(&seat).Error; err != nil {
		t.Fatal(err)
	}
	if seat.OrganizationID != 99 || seat.SeatNumber != "9009" {
		t.Fatalf("expected seat to be updated, got org: %d, seat: %s", seat.OrganizationID, seat.SeatNumber)
	}

	// 5. 测试级联启用停用状态更新
	_, err = repo.SetEnable(context.Background(), acc.ID, false, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", acc.ID).First(&seat).Error; err != nil {
		t.Fatal(err)
	}
	if seat.Enable {
		t.Fatal("expected seat enable state to be set to false")
	}

	// 6. 测试级联逻辑删除
	err = repo.Delete(context.Background(), []int{acc.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", acc.ID).First(&seat).Error; err != nil {
		t.Fatal(err)
	}
	if !seat.DelFlag || seat.Enable {
		t.Fatal("expected seat to be logically deleted and disabled")
	}
}
