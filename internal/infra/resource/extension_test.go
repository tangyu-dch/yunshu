package resource

import (
	"context"
	"errors"
	"testing"

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
