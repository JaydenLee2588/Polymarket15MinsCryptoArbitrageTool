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
// ⚠️ 重要: Polymarket 使用"覆盖性"订阅机制
// 发送 {"type": "market", "assets_ids": ["A"]} 会清空旧订阅，只保留 A
// 因此调用方必须每次发送完整的订阅列表！
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

	// ⚠️ 使用 type: market 格式（服务器会用这个列表完全替换当前订阅）
	// 调用方必须确保传入的是累积的完整列表，而不是增量！
	type subscriptionMessage struct {
		AssetsIDs []string `json:"assets_ids"`
		Type      string   `json:"type"`
	}

	msg := subscriptionMessage{
		AssetsIDs: allTokenIDs,
		Type:      "market",
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
