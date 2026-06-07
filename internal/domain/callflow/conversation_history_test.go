package callflow

import (
	"testing"
	"time"

	"yunshu/internal/contracts"
)

// ============================================================================
// 1. 对话历史操作测试
// ============================================================================

func TestConversationHistory_AddMessages(t *testing.T) {
	history := &contracts.ConversationHistory{
		CallID:    "test-123",
		Messages:  []contracts.LLMMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 添加系统消息
	systemPrompt := "You are a helpful assistant"
	history.AddSystemMessage(systemPrompt)
	if len(history.Messages) != 1 {
		t.Errorf("AddSystemMessage() wrong message count: got %d, want 1", len(history.Messages))
	}
	if history.Messages[0].Role != "system" {
		t.Errorf("AddSystemMessage() wrong role: got %s, want system", history.Messages[0].Role)
	}
	if history.Messages[0].Content != systemPrompt {
		t.Errorf("AddSystemMessage() wrong content")
	}

	// 添加用户消息
	userMessage := "Hello, how are you?"
	history.AddUserMessage(userMessage)
	if len(history.Messages) != 2 {
		t.Errorf("AddUserMessage() wrong message count: got %d, want 2", len(history.Messages))
	}
	if history.Messages[1].Role != "user" {
		t.Errorf("AddUserMessage() wrong role")
	}

	// 添加助手消息
	assistantMessage := "I'm doing well, thank you!"
	history.AddAssistantMessage(assistantMessage)
	if len(history.Messages) != 3 {
		t.Errorf("AddAssistantMessage() wrong message count: got %d, want 3", len(history.Messages))
	}
	if history.Messages[2].Role != "assistant" {
		t.Errorf("AddAssistantMessage() wrong role")
	}
}

func TestConversationHistory_Truncate(t *testing.T) {
	history := &contracts.ConversationHistory{
		CallID:    "test-truncate",
		Messages:  []contracts.LLMMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 添加系统消息
	history.AddSystemMessage("System prompt")

	// 添加多个用户和助手消息
	for i := 0; i < 10; i++ {
		history.AddUserMessage("User message")
		history.AddAssistantMessage("Assistant message")
	}

	totalMessages := len(history.Messages)
	if totalMessages != 21 { // 1 system + 20 other
		t.Errorf("Initial message count wrong: got %d, want 21", totalMessages)
	}

	// 截断到 5 条
	history.Truncate(5)
	if len(history.Messages) > 5 {
		t.Errorf("Truncate() resulted in too many messages: got %d, want <= 5", len(history.Messages))
	}
}

func TestConversationHistory_Timestamps(t *testing.T) {
	history := &contracts.ConversationHistory{
		CallID:    "test-timestamp",
		Messages:  []contracts.LLMMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	history.AddUserMessage("Test")

	oldUpdatedAt := history.UpdatedAt
	time.Sleep(1 * time.Millisecond) // 确保时间不同

	// 再次添加消息，更新时间应该更新
	history.AddAssistantMessage("Response")
	if !history.UpdatedAt.After(oldUpdatedAt) {
		t.Error("UpdatedAt should have been updated after adding a message")
	}
}

// ============================================================================
// 2. 综合测试
// ============================================================================

func TestConversationHistory_FullConversation(t *testing.T) {
	history := &contracts.ConversationHistory{
		CallID:    "full-test-call",
		Messages:  []contracts.LLMMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 模拟完整对话
	history.AddSystemMessage("You are a customer service representative")
	history.AddUserMessage("I have a problem")
	history.AddAssistantMessage("What seems to be the issue?")
	history.AddUserMessage("My account isn't working")
	history.AddAssistantMessage("Let me check that for you")

	if len(history.Messages) != 5 {
		t.Errorf("Full conversation has wrong message count: got %d, want 5", len(history.Messages))
	}

	// 验证角色顺序正确
	expectedRoles := []string{"system", "user", "assistant", "user", "assistant"}
	for i, msg := range history.Messages {
		if msg.Role != expectedRoles[i] {
			t.Errorf("Message %d has wrong role: got %s, want %s", i, msg.Role, expectedRoles[i])
		}
	}
}

// ============================================================================
// 3. 边界情况测试
// ============================================================================

func TestConversationHistory_EmptyHistory(t *testing.T) {
	history := &contracts.ConversationHistory{
		CallID:    "test-empty",
		Messages:  []contracts.LLMMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 截断空历史应该不报错
	history.Truncate(10)
	if len(history.Messages) != 0 {
		t.Error("Truncating empty history should not add messages")
	}
}

func TestConversationHistory_TruncateToZero(t *testing.T) {
	history := &contracts.ConversationHistory{
		CallID:    "test-truncate-zero",
		Messages:  []contracts.LLMMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	history.AddSystemMessage("System")
	history.AddUserMessage("User")

	// 截断到 0
	history.Truncate(0)
	if len(history.Messages) != 0 {
		t.Errorf("Truncate(0) should clear all messages, got %d", len(history.Messages))
	}
}