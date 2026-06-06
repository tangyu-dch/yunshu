package cti

// cti_test 测试 CallControlService。
//
// 验证强拆、监听、强插的业务隔离机制、主管分机匹配规则、
// 监听/强插所对应的 FreeSWITCH ESL originate 命令参数及多租户 SIP 路由域构造。

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	"yunshu/pkg/idempotency"
)

type mockExtensionResolver struct {
	exts map[int]esl.Extension
}

func (m mockExtensionResolver) GetByUserID(ctx context.Context, userID int) (esl.Extension, error) {
	ext, ok := m.exts[userID]
	if !ok {
		return esl.Extension{}, errors.New("not found")
	}
	return ext, nil
}

func TestCallControlService_Hangup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	callID := "test-call-123"

	// 1. 测试会话不存在
	{
		store := esl.NewMemorySessionStore()
		executor := &esl.MemoryCommandExecutor{}
		command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		resolver := mockExtensionResolver{}
		service := NewCallControlService(store, command, resolver, nil)

		err := service.Hangup(ctx, 100, contracts.CallHangupReq{CallID: callID})
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("expected ErrSessionNotFound, got %v", err)
		}
	}

	// 2. 跨商户越权强拆
	{
		store := esl.NewMemorySessionStore()
		executor := &esl.MemoryCommandExecutor{}
		command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		resolver := mockExtensionResolver{}
		service := NewCallControlService(store, command, resolver, nil)

		sess := esl.CallSession{
			CallID: callID,
			UUIDs: map[string]contracts.LegRole{
				"uuid-agent":    contracts.LegRoleAgent,
				"uuid-customer": contracts.LegRoleCustomer,
			},
			FSAddr: "127.0.0.1:8021",
			Metadata: map[string]any{
				"merchantId": "100", // 属于商户 100
			},
		}
		_ = store.Save(ctx, sess)

		err := service.Hangup(ctx, 999, contracts.CallHangupReq{CallID: callID})
		if !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("expected ErrPermissionDenied, got %v", err)
		}
	}

	// 3. 测试指定通道不存在
	{
		store := esl.NewMemorySessionStore()
		executor := &esl.MemoryCommandExecutor{}
		command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		resolver := mockExtensionResolver{}
		service := NewCallControlService(store, command, resolver, nil)

		sess := esl.CallSession{
			CallID: callID,
			UUIDs: map[string]contracts.LegRole{
				"uuid-agent":    contracts.LegRoleAgent,
				"uuid-customer": contracts.LegRoleCustomer,
			},
			FSAddr: "127.0.0.1:8021",
			Metadata: map[string]any{
				"merchantId": "100",
			},
		}
		_ = store.Save(ctx, sess)

		err := service.Hangup(ctx, 100, contracts.CallHangupReq{CallID: callID, UUID: "uuid-non-exist"})
		if err == nil || !strings.Contains(err.Error(), "不在当前通话会话中") {
			t.Fatalf("expected error indicating UUID not in session, got %v", err)
		}
	}

	// 4. 正常强拆单个通道
	{
		store := esl.NewMemorySessionStore()
		executor := &esl.MemoryCommandExecutor{}
		command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		resolver := mockExtensionResolver{}
		service := NewCallControlService(store, command, resolver, nil)

		sess := esl.CallSession{
			CallID: callID,
			UUIDs: map[string]contracts.LegRole{
				"uuid-agent":    contracts.LegRoleAgent,
				"uuid-customer": contracts.LegRoleCustomer,
			},
			FSAddr: "127.0.0.1:8021",
			Metadata: map[string]any{
				"merchantId": "100",
			},
		}
		_ = store.Save(ctx, sess)

		err := service.Hangup(ctx, 100, contracts.CallHangupReq{CallID: callID, UUID: "uuid-agent"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if executor.Count() != 1 {
			t.Fatalf("expected 1 command executed, got %d", executor.Count())
		}
		cmds := executor.Commands
		if cmds[0].Command != "hangup" || cmds[0].UUID != "uuid-agent" || cmds[0].FSAddr != "127.0.0.1:8021" {
			t.Fatalf("unexpected command details: %+v", cmds[0])
		}
	}

	// 5. 正常强拆整个呼叫
	{
		store := esl.NewMemorySessionStore()
		executor := &esl.MemoryCommandExecutor{}
		command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		resolver := mockExtensionResolver{}
		service := NewCallControlService(store, command, resolver, nil)

		sess := esl.CallSession{
			CallID: callID,
			UUIDs: map[string]contracts.LegRole{
				"uuid-agent":    contracts.LegRoleAgent,
				"uuid-customer": contracts.LegRoleCustomer,
			},
			FSAddr: "127.0.0.1:8021",
			Metadata: map[string]any{
				"merchantId": "100",
			},
		}
		_ = store.Save(ctx, sess)

		err := service.Hangup(ctx, 100, contracts.CallHangupReq{CallID: callID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if executor.Count() != 2 {
			t.Fatalf("expected 2 commands executed, got %d", executor.Count())
		}
	}
}

func TestCallControlService_Eavesdrop(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	targetCallID := "target-call-123"
	resolver := mockExtensionResolver{
		exts: map[int]esl.Extension{
			88: {
				ID:              1,
				UserID:          88,
				MerchantID:      100,
				ExtensionNumber: "1008",
				SipDomain:       "test.domain",
			},
			99: {
				ID:              2,
				UserID:          99,
				MerchantID:      999,
				ExtensionNumber: "2008",
				SipDomain:       "other.domain",
			},
		},
	}

	// 1. 越权拦截：主管 (UserID 99) 属于商户 999，但尝试监听商户 100 的通话
	{
		store := esl.NewMemorySessionStore()
		executor := &esl.MemoryCommandExecutor{}
		command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		service := NewCallControlService(store, command, resolver, nil)

		sess := esl.CallSession{
			CallID: targetCallID,
			UUIDs: map[string]contracts.LegRole{
				"uuid-agent":    contracts.LegRoleAgent,
				"uuid-customer": contracts.LegRoleCustomer,
			},
			FSAddr: "192.168.1.100:8021",
			Metadata: map[string]any{
				"merchantId": "100",
			},
		}
		_ = store.Save(ctx, sess)

		err := service.Eavesdrop(ctx, 999, contracts.CallEavesdropReq{
			UserID:       99,
			TargetCallID: targetCallID,
			Mode:         "spy",
		})
		if !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("expected ErrPermissionDenied, got %v", err)
		}
	}

	// 2. 正常监听 (Spy)
	{
		store := esl.NewMemorySessionStore()
		executor := &esl.MemoryCommandExecutor{}
		command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		service := NewCallControlService(store, command, resolver, nil)

		sess := esl.CallSession{
			CallID: targetCallID,
			UUIDs: map[string]contracts.LegRole{
				"uuid-agent":    contracts.LegRoleAgent,
				"uuid-customer": contracts.LegRoleCustomer,
			},
			FSAddr: "192.168.1.100:8021",
			Metadata: map[string]any{
				"merchantId": "100",
			},
		}
		_ = store.Save(ctx, sess)

		err := service.Eavesdrop(ctx, 100, contracts.CallEavesdropReq{
			UserID:       88,
			TargetCallID: targetCallID,
			Mode:         "spy",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if executor.Count() != 1 {
			t.Fatalf("expected 1 command executed, got %d", executor.Count())
		}
		cmd := executor.Commands[0]
		if cmd.Command != "originate" || cmd.FSAddr != "192.168.1.100:8021" {
			t.Fatalf("unexpected target command fields: %+v", cmd)
		}
		
		dest, _ := cmd.Payload["destination"].(string)
		if dest != "1008" {
			t.Fatalf("expected destination 1008, got %s", dest)
		}
		domain, _ := cmd.Payload["domainOrGateway"].(string)
		if domain != "test.domain;fs_path=sip:127.0.0.1:5060" {
			t.Fatalf("expected domain test.domain;fs_path=sip:127.0.0.1:5060, got %s", domain)
		}
		app, _ := cmd.Payload["executeApp"].(string)
		if app != "eavesdrop(uuid-agent)" {
			t.Fatalf("expected app eavesdrop(uuid-agent), got %s", app)
		}
		options, _ := cmd.Payload["options"].(map[string]any)
		if options["eavesdrop_whisper"] != "false" {
			t.Fatalf("expected eavesdrop_whisper false, got %v", options["eavesdrop_whisper"])
		}
	}

	// 3. 强插/单向对坐席插话 (Whisper)
	{
		store := esl.NewMemorySessionStore()
		executor := &esl.MemoryCommandExecutor{}
		command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		service := NewCallControlService(store, command, resolver, nil)

		sess := esl.CallSession{
			CallID: targetCallID,
			UUIDs: map[string]contracts.LegRole{
				"uuid-agent":    contracts.LegRoleAgent,
				"uuid-customer": contracts.LegRoleCustomer,
			},
			FSAddr: "192.168.1.100:8021",
			Metadata: map[string]any{
				"merchantId": "100",
			},
		}
		_ = store.Save(ctx, sess)

		err := service.Eavesdrop(ctx, 100, contracts.CallEavesdropReq{
			UserID:       88,
			TargetCallID: targetCallID,
			Mode:         "whisper",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cmd := executor.Commands[0]
		options, _ := cmd.Payload["options"].(map[string]any)
		if options["eavesdrop_whisper"] != "true" || options["eavesdrop_whisper_aleg"] != "true" || options["eavesdrop_whisper_bleg"] != "false" {
			t.Fatalf("unexpected whisper options: %+v", options)
		}
	}

	// 4. 三方强插 (Barge)
	{
		store := esl.NewMemorySessionStore()
		executor := &esl.MemoryCommandExecutor{}
		command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		service := NewCallControlService(store, command, resolver, nil)

		sess := esl.CallSession{
			CallID: targetCallID,
			UUIDs: map[string]contracts.LegRole{
				"uuid-agent":    contracts.LegRoleAgent,
				"uuid-customer": contracts.LegRoleCustomer,
			},
			FSAddr: "192.168.1.100:8021",
			Metadata: map[string]any{
				"merchantId": "100",
			},
		}
		_ = store.Save(ctx, sess)

		err := service.Eavesdrop(ctx, 100, contracts.CallEavesdropReq{
			UserID:       88,
			TargetCallID: targetCallID,
			Mode:         "barge",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cmd := executor.Commands[0]
		options, _ := cmd.Payload["options"].(map[string]any)
		if options["eavesdrop_whisper"] != "true" || options["eavesdrop_whisper_aleg"] != "true" || options["eavesdrop_whisper_bleg"] != "true" {
			t.Fatalf("unexpected barge options: %+v", options)
		}
	}
}
