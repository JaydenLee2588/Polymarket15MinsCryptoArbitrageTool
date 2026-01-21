package src

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// "connection_id": "",
// "timestamp":1762928533586,
//    "topic":"activity",
//    "type":"orders_matched"

type Message struct {
	ConnectionID string          `json:"connection_id"`
	Timestamp    json.RawMessage `json:"timestamp"`
	Time         time.Time       `json:"-"` // 解析自 timestamp
	Topic        Topic           `json:"topic"`
	Type         MessageType     `json:"type"`
	Payload      json.RawMessage `json:"payload"`
}

// MarshalJSON 实现 Message 的 json.Marshaler 接口
func (m Message) MarshalJSON() ([]byte, error) {
	type Alias Message
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(&m),
	})
}

// UnmarshalJSON 实现 Message 的 json.Unmarshaler 接口
func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias Message
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 解析 Timestamp (支持 int64 数字或 string)
	if len(aux.Timestamp) > 0 {
		// 尝试直接解析为 int64
		var tsInt int64
		if err := json.Unmarshal(aux.Timestamp, &tsInt); err == nil {
			m.Timestamp = json.RawMessage(fmt.Sprintf("%d", tsInt))
			m.Time = time.Unix(0, tsInt*int64(time.Millisecond))
		} else {
			// 尝试解析为 string 然后转 int64
			var tsStr string
			if err := json.Unmarshal(aux.Timestamp, &tsStr); err == nil {
				if tsInt, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
					m.Timestamp = json.RawMessage(fmt.Sprintf("%d", tsInt))
					m.Time = time.Unix(0, tsInt*int64(time.Millisecond))
				}
			}
		}
	}

	// 自动推断 Payload 和 Type/Topic (如果缺失)
	// CLOB WS 消息格式: {"market": "...", "price_changes": [...], "timestamp": "..."}
	// 没有 topic 和 type 字段，也没有 payload 字段
	if len(m.Payload) == 0 {
		// 检查是否包含 price_changes
		if strings.Contains(string(data), `"price_changes"`) {
			m.Topic = TopicClobMarket
			m.Type = MessageTypePriceChange
			m.Payload = data // 整个消息作为 payload
		} else if strings.Contains(string(data), `"market"`) && strings.Contains(string(data), `"price"`) {
			// 可能是最新成交价 (虽然 CLOB 文档说是 price_change)
			m.Topic = TopicClobMarket
			m.Type = MessageTypeLastTradePrice
			m.Payload = data
		}
	}

	return nil
}

type MessageType string

const (
	// MessageTypeAll 是所有消息的类型
	// 这意味着客户端将接收该主题的所有消息，例如 "requests" 或 "quotes"
	MessageTypeAll MessageType = "*"

	// MessageTypeTrades 是交易类型
	MessageTypeTrades MessageType = "trades"

	// MessageTypeOrdersMatched 是订单撮合类型
	MessageTypeOrdersMatched MessageType = "orders_matched"

	// MessageTypeCommentCreated 是评论创建类型
	MessageTypeCommentCreated MessageType = "comment_created"

	// MessageTypeCommentRemoved 是评论移除类型
	MessageTypeCommentRemoved MessageType = "comment_removed"

	// MessageTypeReactionCreated 是反应创建类型
	MessageTypeReactionCreated MessageType = "reaction_created"

	// MessageTypeReactionRemoved 是反应移除类型
	MessageTypeReactionRemoved MessageType = "reaction_removed"

	// MessageTypeRequestCreated 是请求创建类型
	MessageTypeRequestCreated MessageType = "request_created"

	// MessageTypeRequestEdited 是请求编辑类型
	MessageTypeRequestEdited MessageType = "request_edited"

	// MessageTypeRequestCanceled 是请求取消类型
	MessageTypeRequestCanceled MessageType = "request_canceled"

	// MessageTypeRequestExpired 是请求过期类型
	MessageTypeRequestExpired MessageType = "request_expired"

	// MessageTypeQuoteCreated 是报价创建类型
	MessageTypeQuoteCreated MessageType = "quote_created"

	// MessageTypeQuoteEdited 是报价编辑类型
	MessageTypeQuoteEdited MessageType = "quote_edited"

	// MessageTypeQuoteCanceled 是报价取消类型
	MessageTypeQuoteCanceled MessageType = "quote_canceled"

	// MessageTypeQuoteExpired 是报价过期类型
	MessageTypeQuoteExpired MessageType = "quote_expired"

	// MessageTypeUpdate 是价格更新类型
	MessageTypeUpdate MessageType = "update"

	// MessageTypeOrder 是 CLOB 用户订单类型
	MessageTypeOrder MessageType = "order"

	// MessageTypeTrade 是 CLOB 用户交易类型
	MessageTypeTrade MessageType = "trade"

	// MessageTypePriceChange 是 CLOB 市场价格变动类型
	MessageTypePriceChange MessageType = "price_change"

	// MessageTypeAggOrderbook 是 CLOB 市场聚合订单簿类型
	MessageTypeAggOrderbook MessageType = "agg_orderbook"

	// MessageTypeLastTradePrice 是 CLOB 市场最新成交价类型
	MessageTypeLastTradePrice MessageType = "last_trade_price"

	// MessageTypeTickSizeChange 是 CLOB 市场 Tick 大小变动类型
	MessageTypeTickSizeChange MessageType = "tick_size_change"

	// MessageTypeMarketCreated 是 CLOB 市场创建类型
	MessageTypeMarketCreated MessageType = "market_created"

	// MessageTypeMarketResolved 是 CLOB 市场解析类型
	MessageTypeMarketResolved MessageType = "market_resolved"

	// 用于向后兼容和清晰的别名
	MessageTypeCLOBOrder          = MessageTypeOrder
	MessageTypeCLOBTrade          = MessageTypeTrade
	MessageTypeCLOBPriceChanges   = MessageTypePriceChange
	MessageTypeCLOBAggOrderbook   = MessageTypeAggOrderbook
	MessageTypeCLOBLastTradePrice = MessageTypeLastTradePrice
)
