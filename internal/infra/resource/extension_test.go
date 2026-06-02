package resource

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
)

func TestExtensionModelMapsTable(t *testing.T) {
	t.Parallel()

	if (ExtensionModel{}).TableName() != "cc_res_extension" {
		t.Fatalf("unexpected table name")
	}
}

func TestOutboundGuardModelsMapTables(t *testing.T) {
	t.Parallel()

	if (MerchantUserModel{}).TableName() != "cc_res_mch_user" {
		t.Fatalf("unexpected merchant user table")
	}
	if (MerchantModel{}).TableName() != "cc_mch_info" {
		t.Fatalf("unexpected merchant table")
	}
	if (MerchantBillingOverviewModel{}).TableName() != "cc_mch_billing_overview" {
		t.Fatalf("unexpected billing overview table")
	}
}

func TestGatewayModelsMapTables(t *testing.T) {
	t.Parallel()

	if (ChannelModel{}).TableName() != "cc_tel_channel" {
		t.Fatalf("unexpected channel table")
	}
	if (GatewayModel{}).TableName() != "cc_tel_gateway" {
		t.Fatalf("unexpected gateway table")
	}
	if (PoolModel{}).TableName() != "cc_tel_pool" {
		t.Fatalf("unexpected pool table")
	}
}

func TestPhoneResourceModelsMapTables(t *testing.T) {
	t.Parallel()

	if (PoolPhoneModel{}).TableName() != "cc_res_pool_phone" {
		t.Fatalf("unexpected pool phone table")
	}
	if (PoolPhoneSkillGroupModel{}).TableName() != "cc_res_pool_phone_skill_group" {
		t.Fatalf("unexpected pool phone skill group table")
	}
	if (SkillGroupModel{}).TableName() != "cc_res_skill_group" {
		t.Fatalf("unexpected skill group table")
	}
	if (UserSkillGroupModel{}).TableName() != "cc_res_user_skill_group" {
		t.Fatalf("unexpected user skill group table")
	}
}

type fakeStatusReader struct {
	status esl.ExtensionStatus
	ok     bool
	err    error
}

func (r fakeStatusReader) GetExtensionStatus(context.Context, string) (esl.ExtensionStatus, bool, error) {
	return r.status, r.ok, r.err
}

func TestOutboundGuardRejectsOfflineExtension(t *testing.T) {
	t.Parallel()

	guard := &OutboundGuard{Statuses: fakeStatusReader{status: esl.ExtensionStatusOffline, ok: true}}
	err := guard.validateExtensionStatus(context.Background(), "1001")
	if !errors.Is(err, esl.ErrOutboundRejected) {
		t.Fatalf("expected outbound rejected, got %v", err)
	}
}

func TestOutboundGuardAllowsIdleExtension(t *testing.T) {
	t.Parallel()

	guard := &OutboundGuard{Statuses: fakeStatusReader{status: esl.ExtensionStatusIdle, ok: true}}
	if err := guard.validateExtensionStatus(context.Background(), "1001"); err != nil {
		t.Fatalf("expected idle extension allowed, got %v", err)
	}
}

func TestOutboundGuardValidateAPICall(t *testing.T) {
	// 初始化 SQLite 内存数据库作为全真测试环境
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}

	// 自动迁移所有相关的物理表结构
	err = db.AutoMigrate(&MerchantUserModel{}, &MerchantModel{}, &MerchantBillingOverviewModel{})
	if err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	// 定义公共的时间戳和默认的分机实例
	now := time.Now().UTC()
	defaultExt := esl.Extension{
		ID:              999,
		ExtensionNumber: "8001",
		UserID:          101,
		MerchantID:      501,
	}

	t.Run("参数不完整拒绝", func(t *testing.T) {
		guard := NewOutboundGuard(db, fakeStatusReader{status: esl.ExtensionStatusIdle, ok: true}, nil)
		req := contracts.ApiCallReq{UserID: 0, Callee: "13800000000"} // UserID 缺失
		err := guard.ValidateAPICall(context.Background(), req, defaultExt)
		if err == nil || !errors.Is(err, esl.ErrOutboundRejected) {
			t.Fatalf("expected parameters missing rejection, got %v", err)
		}
	})

	t.Run("用户记录不存在拒绝", func(t *testing.T) {
		guard := NewOutboundGuard(db, fakeStatusReader{status: esl.ExtensionStatusIdle, ok: true}, nil)
		req := contracts.ApiCallReq{UserID: 9999, Callee: "13800000000"} // 数据库查无此人
		err := guard.ValidateAPICall(context.Background(), req, defaultExt)
		if !errors.Is(err, esl.ErrMerchantUserNotFound) {
			t.Fatalf("expected user not found error, got %v", err)
		}
	})

	t.Run("用户已被停用拒绝", func(t *testing.T) {
		// 写入停用的用户数据
		user := MerchantUserModel{
			ID:         102,
			MerchantID: 501,
			Username:   "disabled_user",
			Enable:     false, // 未启用
			DelFlag:    false,
		}
		db.Create(&user)
		defer db.Delete(&user)

		guard := NewOutboundGuard(db, fakeStatusReader{status: esl.ExtensionStatusIdle, ok: true}, nil)
		req := contracts.ApiCallReq{UserID: 102, Callee: "13800000000"}
		err := guard.ValidateAPICall(context.Background(), req, defaultExt)
		if err == nil || !errors.Is(err, esl.ErrOutboundRejected) {
			t.Fatalf("expected user disabled rejection, got %v", err)
		}
	})

	t.Run("坐席分机不属于当前用户拒绝", func(t *testing.T) {
		// 写入正常的用户数据
		user := MerchantUserModel{
			ID:         103,
			MerchantID: 501,
			Username:   "valid_user",
			Enable:     true,
			DelFlag:    false,
		}
		db.Create(&user)
		defer db.Delete(&user)

		guard := NewOutboundGuard(db, fakeStatusReader{status: esl.ExtensionStatusIdle, ok: true}, nil)
		req := contracts.ApiCallReq{UserID: 103, Callee: "13800000000"}
		// 传给 ValidateAPICall 的分机其 UserID 绑定的是 999 (不匹配当前用户 103)
		mismatchExt := defaultExt
		mismatchExt.UserID = 999 

		err := guard.ValidateAPICall(context.Background(), req, mismatchExt)
		if err == nil || !errors.Is(err, esl.ErrOutboundRejected) {
			t.Fatalf("expected extension ownership mismatch rejection, got %v", err)
		}
	})

	t.Run("商户不存在拒绝", func(t *testing.T) {
		// 写入一个绑定了不存在的商户的用户
		user := MerchantUserModel{
			ID:         104,
			MerchantID: 99999, // 不存在的商户ID
			Username:   "no_merchant_user",
			Enable:     true,
			DelFlag:    false,
		}
		db.Create(&user)
		defer db.Delete(&user)

		guard := NewOutboundGuard(db, fakeStatusReader{status: esl.ExtensionStatusIdle, ok: true}, nil)
		req := contracts.ApiCallReq{UserID: 104, Callee: "13800000000"}
		
		noMchExt := defaultExt
		noMchExt.UserID = 104
		noMchExt.MerchantID = 99999

		err := guard.ValidateAPICall(context.Background(), req, noMchExt)
		if !errors.Is(err, esl.ErrMerchantNotFound) {
			t.Fatalf("expected merchant not found error, got %v", err)
		}
	})

	t.Run("商户账号停用拒绝", func(t *testing.T) {
		// 写入正常的用户与停用的商户数据
		user := MerchantUserModel{
			ID:         105,
			MerchantID: 502,
			Username:   "disabled_mch_user",
			Enable:     true,
			DelFlag:    false,
		}
		merchant := MerchantModel{
			ID:     502,
			Name:   "停用商户",
			Enable: false, // 已停用
		}
		db.Create(&user)
		db.Create(&merchant)
		defer db.Delete(&user)
		defer db.Delete(&merchant)

		guard := NewOutboundGuard(db, fakeStatusReader{status: esl.ExtensionStatusIdle, ok: true}, nil)
		req := contracts.ApiCallReq{UserID: 105, Callee: "13800000000"}
		
		disabledMchExt := defaultExt
		disabledMchExt.UserID = 105
		disabledMchExt.MerchantID = 502

		err := guard.ValidateAPICall(context.Background(), req, disabledMchExt)
		if err == nil || !errors.Is(err, esl.ErrOutboundRejected) {
			t.Fatalf("expected merchant disabled rejection, got %v", err)
		}
	})

	t.Run("商户服务过期拒绝", func(t *testing.T) {
		user := MerchantUserModel{
			ID:         106,
			MerchantID: 503,
			Username:   "expired_mch_user",
			Enable:     true,
			DelFlag:    false,
		}
		expiredTime := now.Add(-1 * time.Hour) // 已过期 1 小时
		merchant := MerchantModel{
			ID:          503,
			Name:        "过期商户",
			Enable:      true,
			ExpiredTime: &expiredTime,
		}
		db.Create(&user)
		db.Create(&merchant)
		defer db.Delete(&user)
		defer db.Delete(&merchant)

		guard := NewOutboundGuard(db, fakeStatusReader{status: esl.ExtensionStatusIdle, ok: true}, nil)
		guard.Now = func() time.Time { return now }
		req := contracts.ApiCallReq{UserID: 106, Callee: "13800000000"}
		
		expiredMchExt := defaultExt
		expiredMchExt.UserID = 106
		expiredMchExt.MerchantID = 503

		err := guard.ValidateAPICall(context.Background(), req, expiredMchExt)
		if err == nil || !errors.Is(err, esl.ErrOutboundRejected) {
			t.Fatalf("expected merchant expired rejection, got %v", err)
		}
	})

	t.Run("预付费欠费拒绝", func(t *testing.T) {
		user := MerchantUserModel{
			ID:         107,
			MerchantID: 504,
			Username:   "debt_user",
			Enable:     true,
			DelFlag:    false,
		}
		merchant := MerchantModel{
			ID:     504,
			Name:   "欠费商户",
			Enable: true,
		}
		billing := MerchantBillingOverviewModel{
			MerchantID:     504,
			PaymentMode:    paymentModePrepaid,
			CurrentBalance: -5.0, // 欠费
			CreditLimit:    0.0,
		}
		db.Create(&user)
		db.Create(&merchant)
		db.Create(&billing)
		defer db.Delete(&user)
		defer db.Delete(&merchant)
		defer db.Delete(&billing)

		guard := NewOutboundGuard(db, fakeStatusReader{status: esl.ExtensionStatusIdle, ok: true}, nil)
		guard.Now = func() time.Time { return now }
		req := contracts.ApiCallReq{UserID: 107, Callee: "13800000000"}
		
		debtMchExt := defaultExt
		debtMchExt.UserID = 107
		debtMchExt.MerchantID = 504

		err := guard.ValidateAPICall(context.Background(), req, debtMchExt)
		if err == nil || !errors.Is(err, esl.ErrOutboundRejected) {
			t.Fatalf("expected prepaid debt rejection, got %v", err)
		}
	})

	t.Run("分机忙碌状态拒绝", func(t *testing.T) {
		user := MerchantUserModel{
			ID:         108,
			MerchantID: 505,
			Username:   "busy_ext_user",
			Enable:     true,
			DelFlag:    false,
		}
		merchant := MerchantModel{
			ID:     505,
			Name:   "正常商户",
			Enable: true,
		}
		db.Create(&user)
		db.Create(&merchant)
		defer db.Delete(&user)
		defer db.Delete(&merchant)

		// 模拟分机状态为 ringing 忙碌中
		guard := NewOutboundGuard(db, fakeStatusReader{status: esl.ExtensionStatusRinging, ok: true}, nil)
		guard.Now = func() time.Time { return now }
		req := contracts.ApiCallReq{UserID: 108, Callee: "13800000000"}
		
		busyMchExt := defaultExt
		busyMchExt.UserID = 108
		busyMchExt.MerchantID = 505
		busyMchExt.ExtensionNumber = "8002"

		err := guard.ValidateAPICall(context.Background(), req, busyMchExt)
		if err == nil || !errors.Is(err, esl.ErrOutboundRejected) {
			t.Fatalf("expected busy extension rejection, got %v", err)
		}
	})

	t.Run("绿灯全链路校验成功", func(t *testing.T) {
		user := MerchantUserModel{
			ID:         109,
			MerchantID: 506,
			Username:   "green_light_user",
			Enable:     true,
			DelFlag:    false,
		}
		merchant := MerchantModel{
			ID:     506,
			Name:   "绿灯商户",
			Enable: true,
		}
		billing := MerchantBillingOverviewModel{
			MerchantID:     506,
			PaymentMode:    paymentModePrepaid,
			CurrentBalance: 100.0, // 正常余额
			CreditLimit:    50.0,
		}
		db.Create(&user)
		db.Create(&merchant)
		db.Create(&billing)
		defer db.Delete(&user)
		defer db.Delete(&merchant)
		defer db.Delete(&billing)

		guard := NewOutboundGuard(db, fakeStatusReader{status: esl.ExtensionStatusIdle, ok: true}, nil)
		guard.Now = func() time.Time { return now }
		req := contracts.ApiCallReq{UserID: 109, Callee: "13800000000"}
		
		successExt := defaultExt
		successExt.UserID = 109
		successExt.MerchantID = 506
		successExt.ExtensionNumber = "8003"

		err := guard.ValidateAPICall(context.Background(), req, successExt)
		if err != nil {
			t.Fatalf("expected validation success (nil), got %v", err)
		}
	})
}
