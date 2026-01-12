package src

import (
	"encoding/json"
	"testing"
)

// ========================================
// RealtimeProtocol 测试
// ========================================

func TestNewRealtimeProtocol(t *testing.T) {
	protocol := NewRealtimeProtocol()
	if protocol == nil {
		t.Fatal("NewRealtimeProtocol() 返回 nil")
	}
}

func TestRealtimeProtocol_GetDefaultHost(t *testing.T) {
	protocol := NewRealtimeProtocol()

	host := protocol.GetDefaultHost()

	if host == "" {
		t.Error("GetDefaultHost() 返回空字符串")
	}

	// 验证是 WebSocket URL
	if !containsString(host, "wss://") {
		t.Errorf("GetDefaultHost() = %v, 应该是 wss:// 开头", host)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRealtimeProtocol_GetProtocolName(t *testing.T) {
	protocol := NewRealtimeProtocol()

	name := protocol.GetProtocolName()

	if name == "" {
		t.Error("GetProtocolName() 返回空字符串")
	}
}

func TestRealtimeProtocol_FormatSubscription_Single(t *testing.T) {
	protocol := NewRealtimeProtocol()

	subscriptions := []Subscription{
		{
			Topic: TopicActivity,
			Type:  MessageTypeTrades,
		},
	}

	data, err := protocol.FormatSubscription(subscriptions)
	if err != nil {
		t.Fatalf("FormatSubscription() error = %v", err)
	}

	// 验证返回有效的 JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("FormatSubscription() 返回无效 JSON: %v", err)
	}

	// 验证包含 action 字段
	if action, ok := result["action"].(string); !ok || action != "subscribe" {
		t.Errorf("FormatSubscription() action = %v, want subscribe", result["action"])
	}

	// 验证包含 subscriptions 数组
	if subs, ok := result["subscriptions"].([]interface{}); !ok || len(subs) != 1 {
		t.Errorf("FormatSubscription() subscriptions 数量错误")
	}
}

func TestRealtimeProtocol_FormatSubscription_Multiple(t *testing.T) {
	protocol := NewRealtimeProtocol()

	subscriptions := []Subscription{
		{
			Topic: TopicActivity,
			Type:  MessageTypeTrades,
		},
		{
			Topic:   TopicClobMarket,
			Type:    MessageTypePriceChange,
			Filters: `["token1"]`,
		},
	}

	data, err := protocol.FormatSubscription(subscriptions)
	if err != nil {
		t.Fatalf("FormatSubscription() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("FormatSubscription() 返回无效 JSON: %v", err)
	}

	// 验证包含 2 个订阅
	subs, ok := result["subscriptions"].([]interface{})
	if !ok {
		t.Error("FormatSubscription() subscriptions 不是数组")
	} else if len(subs) != 2 {
		t.Errorf("FormatSubscription() subscriptions 数量 = %d, want 2", len(subs))
	}
}

func TestRealtimeProtocol_FormatSubscription_Empty(t *testing.T) {
	protocol := NewRealtimeProtocol()

	subscriptions := []Subscription{}

	data, err := protocol.FormatSubscription(subscriptions)
	if err != nil {
		t.Fatalf("FormatSubscription() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("FormatSubscription() 返回无效 JSON: %v", err)
	}

	// 空订阅列表应该产生空数组
	if subs, ok := result["subscriptions"].([]interface{}); !ok || len(subs) != 0 {
		t.Errorf("FormatSubscription() 空订阅应返回空数组")
	}
}

func TestRealtimeProtocol_ParseMessage_Valid(t *testing.T) {
	protocol := NewRealtimeProtocol()

	input := []byte(`{
		"connection_id": "conn123",
		"timestamp": 1700000000000,
		"topic": "activity",
		"type": "trades",
		"payload": {"key": "value"}
	}`)

	msg, err := protocol.ParseMessage(input)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}

	if msg == nil {
		t.Fatal("ParseMessage() 返回 nil")
	}

	if msg.ConnectionID != "conn123" {
		t.Errorf("ParseMessage() ConnectionID = %v, want conn123", msg.ConnectionID)
	}

	if msg.Topic != TopicActivity {
		t.Errorf("ParseMessage() Topic = %v, want activity", msg.Topic)
	}

	if msg.Type != MessageTypeTrades {
		t.Errorf("ParseMessage() Type = %v, want trades", msg.Type)
	}
}

func TestRealtimeProtocol_ParseMessage_Invalid(t *testing.T) {
	protocol := NewRealtimeProtocol()

	input := []byte(`invalid json`)

	_, err := protocol.ParseMessage(input)
	if err == nil {
		t.Error("ParseMessage() 无效 JSON 应返回错误")
	}
}

func TestRealtimeProtocol_ParseMessage_Empty(t *testing.T) {
	protocol := NewRealtimeProtocol()

	input := []byte(`{}`)

	msg, err := protocol.ParseMessage(input)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}

	// 空对象应该解析成功，字段为零值
	if msg.ConnectionID != "" {
		t.Errorf("ParseMessage() 空对象 ConnectionID 应为空")
	}
}

// ========================================
// Protocol 接口测试
// ========================================

func TestProtocol_Interface(t *testing.T) {
	// 验证 realtimeProtocol 实现了 Protocol 接口
	var protocol Protocol = NewRealtimeProtocol()

	if protocol == nil {
		t.Error("NewRealtimeProtocol 应该实现 Protocol 接口")
	}

	// 验证所有接口方法可调用
	_ = protocol.GetDefaultHost()
	_ = protocol.GetProtocolName()
	_, _ = protocol.FormatSubscription(nil)
	_, _ = protocol.ParseMessage([]byte("{}"))
}

