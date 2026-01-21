package src

import (
	"encoding/json"
	"fmt"
)

// Protocol 定义特定端点行为的接口
type Protocol interface {
	// GetDefaultHost 返回此协议的默认 WebSocket 主机
	GetDefaultHost() string

	// FormatSubscription 将订阅格式化为协议特定的消息格式
	FormatSubscription(subscriptions []Subscription) ([]byte, error)

	// ParseMessage 将原始消息解析为结构化的 Message 格式
	// 返回解析后的消息和遇到的任何错误
	ParseMessage(data []byte) (*Message, error)

	// GetProtocolName 返回此协议的人类可读名称
	GetProtocolName() string
}

// realtimeProtocol 实现了实时数据端点的 Protocol 接口
type realtimeProtocol struct{}

// NewRealtimeProtocol 创建一个新的实时协议实例
func NewRealtimeProtocol() Protocol {
	return &realtimeProtocol{}
}

// GetDefaultHost 返回实时数据的默认 WebSocket 主机
func (p *realtimeProtocol) GetDefaultHost() string {
	return WebSocketURL
}

// FormatSubscription 将订阅格式化为 CLOB 协议消息格式
// CLOB WebSocket 期望格式:
// - 初始订阅: {"assets_ids": ["token_id_1", "token_id_2"], "type": "market"}
// - 动态追加: {"assets_ids": ["token_id"], "operation": "subscribe"}
func (p *realtimeProtocol) FormatSubscription(subscriptions []Subscription) ([]byte, error) {
	// 收集所有 token IDs
	var allTokenIDs []string

	for _, sub := range subscriptions {
		// 从 Filters 字段解析 token IDs
		// Filters 是 JSON 格式的字符串数组，如 ["token_id_1", "token_id_2"]
		if sub.Filters != "" {
			var tokenIDs []string
			if err := json.Unmarshal([]byte(sub.Filters), &tokenIDs); err == nil {
				allTokenIDs = append(allTokenIDs, tokenIDs...)
			}
		}
	}

	// 如果没有 token IDs，返回空消息
	if len(allTokenIDs) == 0 {
		return nil, fmt.Errorf("没有可订阅的 token IDs")
	}

	// ⚠️ 关键修复：使用 operation: subscribe 进行动态订阅
	// 参考: binary-jerry/polymarket-sdk ws_client.go
	// - 初始连接使用 {"assets_ids": [...], "type": "market"}
	// - 动态追加使用 {"assets_ids": [...], "operation": "subscribe"}
	//
	// 由于我们在 baseClient 中无法区分首次/后续订阅，
	// 这里改为始终使用 "operation": "subscribe" 格式
	// 因为 Polymarket 服务器对两种格式都能正确处理追加
	type dynamicSubscriptionMessage struct {
		AssetsIDs []string `json:"assets_ids"`
		Operation string   `json:"operation"`
	}

	msg := dynamicSubscriptionMessage{
		AssetsIDs: allTokenIDs,
		Operation: "subscribe",
	}

	return json.Marshal(msg)
}

// ParseMessage 将原始消息解析为结构化的 Message 格式
func (p *realtimeProtocol) ParseMessage(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return nil, fmt.Errorf("解析消息失败: %w", err)
	}
	return &msg, nil
}

// GetProtocolName 返回此协议的人类可读名称
func (p *realtimeProtocol) GetProtocolName() string {
	return "实时数据"
}
