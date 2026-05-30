package operate_test

import (
	"context"
	"errors"
	"testing"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/operate"
)

func TestAccountServiceRejectsSecondMerchantAdmin(t *testing.T) {
	t.Parallel()

	service := &operate.AccountManagementService{Repository: newFakeAccountRepository()}
	ctx := contracts.WithTenant(context.Background(), contracts.TenantContext{Internal: true, RoleID: "super_admin"})
	first, err := service.Save(ctx, operate.Account{Username: "admin-a", Password: "secret", MerchantID: "1001", AccountType: operate.AccountTypeMerchantAdmin, Enable: true})
	if err != nil {
		t.Fatal(err)
	}
	if first.Account.ID == 0 {
		t.Fatal("expected first merchant admin id")
	}
	_, err = service.Save(ctx, operate.Account{Username: "admin-b", Password: "secret", MerchantID: "1001", AccountType: operate.AccountTypeMerchantAdmin, Enable: true})
	if !errors.Is(err, operate.ErrAccountConflict) {
		t.Fatalf("expected merchant admin conflict, got %v", err)
	}
}

func TestAccountServiceMerchantAdminCanOnlyMaintainOwnMerchantUsers(t *testing.T) {
	t.Parallel()

	service := &operate.AccountManagementService{Repository: newFakeAccountRepository()}
	ctx := contracts.WithTenant(context.Background(), contracts.TenantContext{
		MerchantID: "1001",
		UserID:     "2001",
		RoleID:     "merchant_admin",
		Permissions: []string{
			string(contracts.PermissionMerchantAccountWrite),
		},
	})
	saved, err := service.Save(ctx, operate.Account{Username: "agent-a", Password: "secret", AccountType: operate.AccountTypeMerchantUser, Enable: true})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Account.MerchantID != "1001" || saved.Account.AccountType != operate.AccountTypeMerchantUser {
		t.Fatalf("unexpected merchant user: %+v", saved.Account)
	}
	_, err = service.Save(ctx, operate.Account{Username: "admin-a", Password: "secret", MerchantID: "1001", AccountType: operate.AccountTypeMerchantAdmin, Enable: true})
	if !errors.Is(err, operate.ErrAccountForbidden) {
		t.Fatalf("expected merchant admin mutation to be forbidden, got %v", err)
	}
	_, err = service.Save(ctx, operate.Account{Username: "agent-b", Password: "secret", MerchantID: "1002", AccountType: operate.AccountTypeMerchantUser, Enable: true})
	if !errors.Is(err, operate.ErrAccountForbidden) {
		t.Fatalf("expected cross-merchant mutation to be forbidden, got %v", err)
	}
}

func TestAccountServiceOperatePermissionRequired(t *testing.T) {
	t.Parallel()

	service := &operate.AccountManagementService{Repository: newFakeAccountRepository()}
	denied := contracts.WithTenant(context.Background(), contracts.TenantContext{RoleID: "operate_staff"})
	_, err := service.Save(denied, operate.Account{Username: "merchant-admin", Password: "secret", MerchantID: "1001", AccountType: operate.AccountTypeMerchantAdmin, Enable: true})
	if !errors.Is(err, operate.ErrAccountForbidden) {
		t.Fatalf("expected missing operate permission to be forbidden, got %v", err)
	}

	allowed := contracts.WithTenant(context.Background(), contracts.TenantContext{
		RoleID:      "operate_staff",
		Permissions: []string{string(contracts.PermissionOperateAccountWrite)},
	})
	result, err := service.Save(allowed, operate.Account{Username: "merchant-admin", Password: "secret", MerchantID: "1001", AccountType: operate.AccountTypeMerchantAdmin, Enable: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Account.RoleID != "merchant_admin" {
		t.Fatalf("expected merchant admin role, got %+v", result.Account)
	}
}

type fakeAccountRepository struct {
	accounts map[int]operate.Account
	nextID   int
}

func newFakeAccountRepository() *fakeAccountRepository {
	return &fakeAccountRepository{
		accounts: map[int]operate.Account{},
		nextID:   1,
	}
}

func (r *fakeAccountRepository) Page(_ context.Context, req operate.AccountPageRequest) (operate.AccountPageResult, error) {
	records := make([]operate.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		records = append(records, account)
	}
	return operate.AccountPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: int64(len(records)), Records: records}, nil
}

func (r *fakeAccountRepository) GetByID(_ context.Context, id int) (operate.Account, error) {
	account, ok := r.accounts[id]
	if !ok {
		return operate.Account{}, operate.ErrAccountNotFound
	}
	return account, nil
}

func (r *fakeAccountRepository) ExistsUsername(_ context.Context, username string, excludeID int) (bool, error) {
	for id, account := range r.accounts {
		if id == excludeID {
			continue
		}
		if account.Username == username {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeAccountRepository) ActiveMerchantAdminExists(_ context.Context, merchantID string, excludeID int) (bool, error) {
	for id, account := range r.accounts {
		if id == excludeID {
			continue
		}
		if account.MerchantID == merchantID && account.AccountType == operate.AccountTypeMerchantAdmin && account.Enable {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeAccountRepository) Save(_ context.Context, account operate.Account) (operate.Account, error) {
	if account.ID == 0 {
		account.ID = r.nextID
		r.nextID++
	}
	r.accounts[account.ID] = account
	return account, nil
}

func (r *fakeAccountRepository) Delete(_ context.Context, ids []int) error {
	removed := 0
	for _, id := range ids {
		if _, ok := r.accounts[id]; ok {
			delete(r.accounts, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrAccountNotFound
	}
	return nil
}

func (r *fakeAccountRepository) SetEnable(_ context.Context, id int, enable bool, operator string) (operate.Account, error) {
	account, ok := r.accounts[id]
	if !ok {
		return operate.Account{}, operate.ErrAccountNotFound
	}
	account.Enable = enable
	account.UpdatedBy = operator
	r.accounts[id] = account
	return account, nil
}

func (r *fakeAccountRepository) ResetPassword(_ context.Context, id int, password string, operator string) error {
	account, ok := r.accounts[id]
	if !ok {
		return operate.ErrAccountNotFound
	}
	account.Password = password
	account.UpdatedBy = operator
	r.accounts[id] = account
	return nil
}

var _ operate.AccountRepository = (*fakeAccountRepository)(nil)
