package src

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// RealtimeTypedSubscriptionHandler 为不同的消息类型提供类型安全的订阅处理程序
//
// 此处理程序结合了订阅管理和消息路由。它内部维护一个
// RealtimeMessageRouter，将传入的消息路由到注册的回调。
type RealtimeTypedSubscriptionHandler struct {
	client WsClient
	router *RealtimeMessageRouter
}

// NewRealtimeTypedSubscriptionHandler 创建一个新的实时数据类型化订阅处理程序
// client 参数可以为 nil，稍后使用 SetClient() 设置
func NewRealtimeTypedSubscriptionHandler(client WsClient, router *RealtimeMessageRouter) *RealtimeTypedSubscriptionHandler {
	return &RealtimeTypedSubscriptionHandler{
		client: client,
		router: router,
	}
}

// GetRouter 返回内部消息路由器
// 将此与 WithRouter 选项一起使用以自动路由消息
//
// 示例:
//
//	typedSub := NewRealtimeTypedSubscriptionHandler(nil)
//	client := New(
//	    WithRouter(typedSub.GetRouter()),
//	    WithLogger(NewLogger()),
//	)
//	typedSub.SetClient(client)
func (h *RealtimeTypedSubscriptionHandler) GetRouter() *RealtimeMessageRouter {
	return h.router
}

// SetClient 为此处理程序设置客户端
// 当你在客户端之前创建处理程序时，这很有用
func (h *RealtimeTypedSubscriptionHandler) SetClient(client WsClient) {
	h.client = client
}

// Activity 订阅处理程序

// ActivityTradesCallback 是活动交易消息的回调函数
type ActivityTradesCallback func(trade Trade) error

// SubscribeToActivityTrades 使用类型化回调订阅活动交易
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个市场/资产并接收它们的所有数据。
//
// 过滤器格式: {"event_slug":"string"} 或 {"market_slug":"string"}
// filter: 可选的 ActivityFilter，用于按市场或事件 slug 过滤
func (h *RealtimeTypedSubscriptionHandler) SubscribeToActivityTrades(callback ActivityTradesCallback, filter *ActivityFilter) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterActivityTradesHandler(callback)
	}

	filterStr := ""
	if filter != nil {
		var err error
		filterStr, err = filter.ToJSON()
		if err != nil {
			return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
		}
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicActivity,
			Type:    MessageTypeTrades,
			Filters: filterStr,
		},
	})
}

// ActivityOrdersMatchedCallback 是活动订单撮合消息的回调函数
type ActivityOrdersMatchedCallback func(trade Trade) error

// SubscribeToActivityOrdersMatched 使用类型化回调订阅活动订单撮合
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个市场/资产并接收它们的所有数据。
//
// 过滤器格式: {"event_slug":"string"} 或 {"market_slug":"string"}
// filter: 可选的 ActivityFilter，用于按市场或事件 slug 过滤
func (h *RealtimeTypedSubscriptionHandler) SubscribeToActivityOrdersMatched(filter *ActivityFilter, callback ActivityOrdersMatchedCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterActivityOrdersMatchedHandler(callback)
	}

	filterStr := ""
	if filter != nil {
		var err error
		filterStr, err = filter.ToJSON()
		if err != nil {
			return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
		}
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicActivity,
			Type:    MessageTypeOrdersMatched,
			Filters: filterStr,
		},
	})
}

// Comments 订阅处理程序

// CommentCreatedCallback 是评论创建消息的回调函数
type CommentCreatedCallback func(comment Comment) error

// SubscribeToCommentCreated 使用类型化回调订阅评论创建事件
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个事件/系列并接收它们的所有数据。
//
// 过滤器格式: {"event_slug":"string"} 或 {"series_slug":"string"}
// filter: 可选的 CommentFilter，用于按事件或系列 slug 过滤
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCommentCreated(filter *CommentFilter, callback CommentCreatedCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterCommentCreatedHandler(callback)
	}

	filterStr := ""
	if filter != nil {
		var err error
		filterStr, err = filter.ToJSON()
		if err != nil {
			return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
		}
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicComments,
			Type:    MessageTypeCommentCreated,
			Filters: filterStr,
		},
	})
}

// CommentRemovedCallback 是评论移除消息的回调函数
type CommentRemovedCallback func(comment Comment) error

// SubscribeToCommentRemoved 使用类型化回调订阅评论移除事件
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个事件/系列并接收它们的所有数据。
//
// 过滤器格式: {"event_slug":"string"} 或 {"series_slug":"string"}
// filter: 可选的 CommentFilter，用于按事件或系列 slug 过滤
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCommentRemoved(filter *CommentFilter, callback CommentRemovedCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterCommentRemovedHandler(callback)
	}

	filterStr := ""
	if filter != nil {
		var err error
		filterStr, err = filter.ToJSON()
		if err != nil {
			return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
		}
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicComments,
			Type:    MessageTypeCommentRemoved,
			Filters: filterStr,
		},
	})
}

// ReactionCreatedCallback 是反应创建消息的回调函数
type ReactionCreatedCallback func(reaction Reaction) error

// SubscribeToReactionCreated 使用类型化回调订阅反应创建事件
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个事件/系列并接收它们的所有数据。
//
// 过滤器格式: {"event_slug":"string"} 或 {"series_slug":"string"}
// filter: 可选的 CommentFilter，用于按事件或系列 slug 过滤
func (h *RealtimeTypedSubscriptionHandler) SubscribeToReactionCreated(filter *CommentFilter, callback ReactionCreatedCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterReactionCreatedHandler(callback)
	}

	filterStr := ""
	if filter != nil {
		var err error
		filterStr, err = filter.ToJSON()
		if err != nil {
			return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
		}
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicComments,
			Type:    MessageTypeReactionCreated,
			Filters: filterStr,
		},
	})
}

// ReactionRemovedCallback 是反应移除消息的回调函数
type ReactionRemovedCallback func(reaction Reaction) error

// SubscribeToReactionRemoved 使用类型化回调订阅反应移除事件
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个事件/系列并接收它们的所有数据。
//
// 过滤器格式: {"event_slug":"string"} 或 {"series_slug":"string"}
// filter: 可选的 CommentFilter，用于按事件或系列 slug 过滤
func (h *RealtimeTypedSubscriptionHandler) SubscribeToReactionRemoved(filter *CommentFilter, callback ReactionRemovedCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterReactionRemovedHandler(callback)
	}

	filterStr := ""
	if filter != nil {
		var err error
		filterStr, err = filter.ToJSON()
		if err != nil {
			return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
		}
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicComments,
			Type:    MessageTypeReactionRemoved,
			Filters: filterStr,
		},
	})
}

// RFQ 订阅处理程序

// RFQRequestCallback 是 RFQ 请求消息的回调函数
type RFQRequestCallback func(request RFQRequest) error

// SubscribeToRFQRequestCreated 订阅 RFQ 请求创建事件
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个 RFQ 事件并接收它们的所有数据。
//
// 过滤器格式: RFQ 主题不需要过滤器
func (h *RealtimeTypedSubscriptionHandler) SubscribeToRFQRequestCreated(callback RFQRequestCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterRFQRequestHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic: TopicRfq,
			Type:  MessageTypeRequestCreated,
		},
	})
}

// SubscribeToRFQRequestEdited 订阅 RFQ 请求编辑事件
//
// 此主题支持多个订阅。详情请参阅 SubscribeToRFQRequestCreated。
func (h *RealtimeTypedSubscriptionHandler) SubscribeToRFQRequestEdited(callback RFQRequestCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterRFQRequestHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic: TopicRfq,
			Type:  MessageTypeRequestEdited,
		},
	})
}

// SubscribeToRFQRequestCanceled 订阅 RFQ 请求取消事件
//
// 此主题支持多个订阅。详情请参阅 SubscribeToRFQRequestCreated。
func (h *RealtimeTypedSubscriptionHandler) SubscribeToRFQRequestCanceled(callback RFQRequestCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterRFQRequestHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic: TopicRfq,
			Type:  MessageTypeRequestCanceled,
		},
	})
}

// SubscribeToRFQRequestExpired 订阅 RFQ 请求过期事件
//
// 此主题支持多个订阅。详情请参阅 SubscribeToRFQRequestCreated。
func (h *RealtimeTypedSubscriptionHandler) SubscribeToRFQRequestExpired(callback RFQRequestCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterRFQRequestHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic: TopicRfq,
			Type:  MessageTypeRequestExpired,
		},
	})
}

// RFQQuoteCallback 是 RFQ 报价消息的回调函数
type RFQQuoteCallback func(quote RFQQuote) error

// SubscribeToRFQQuoteCreated 订阅 RFQ 报价创建事件
//
// 此主题支持多个订阅。详情请参阅 SubscribeToRFQRequestCreated。
func (h *RealtimeTypedSubscriptionHandler) SubscribeToRFQQuoteCreated(callback RFQQuoteCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterRFQQuoteHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic: TopicRfq,
			Type:  MessageTypeQuoteCreated,
		},
	})
}

// SubscribeToRFQQuoteEdited 订阅 RFQ 报价编辑事件
//
// 此主题支持多个订阅。详情请参阅 SubscribeToRFQRequestCreated。
func (h *RealtimeTypedSubscriptionHandler) SubscribeToRFQQuoteEdited(callback RFQQuoteCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterRFQQuoteHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic: TopicRfq,
			Type:  MessageTypeQuoteEdited,
		},
	})
}

// SubscribeToRFQQuoteCanceled 订阅 RFQ 报价取消事件
//
// 此主题支持多个订阅。详情请参阅 SubscribeToRFQRequestCreated。
func (h *RealtimeTypedSubscriptionHandler) SubscribeToRFQQuoteCanceled(callback RFQQuoteCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterRFQQuoteHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic: TopicRfq,
			Type:  MessageTypeQuoteCanceled,
		},
	})
}

// SubscribeToRFQQuoteExpired 订阅 RFQ 报价过期事件
//
// 此主题支持多个订阅。详情请参阅 SubscribeToRFQRequestCreated。
func (h *RealtimeTypedSubscriptionHandler) SubscribeToRFQQuoteExpired(callback RFQQuoteCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterRFQQuoteHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic: TopicRfq,
			Type:  MessageTypeQuoteExpired,
		},
	})
}

// Crypto Prices 订阅处理程序

// CryptoPriceCallback 是加密货币价格更新消息的回调函数
type CryptoPriceCallback func(price CryptoPrice) error

// SubscribeToCryptoPrices 订阅加密货币价格更新
//
// 重要提示: crypto_prices 主题的 Polymarket WebSocket API 仅支持
// 每个连接一个代码 (ONE symbol)。订阅新代码将替换以前的
// 订阅，而不是添加到其中。如果你需要同时监控多个代码，
// 你必须为每个代码创建单独的客户端连接。
//
// 正确使用多个代码的示例:
//
//	// 为每个代码创建单独的客户端
//	solClient := FKGoPolyTools.New(...)
//	ethClient := FKGoPolyTools.New(...)
//
//	solClient.Connect()
//	ethClient.Connect()
//
//	// 每个客户端订阅一个代码
//	solSub := FKGoPolyTools.NewRealtimeTypedSubscriptionHandler(solClient)
//	ethSub := FKGoPolyTools.NewRealtimeTypedSubscriptionHandler(ethClient)
//
//	solSub.SubscribeToCryptoPrices(nil, FKGoPolyTools.NewSOLPriceFilter())
//	ethSub.SubscribeToCryptoPrices(nil, FKGoPolyTools.NewETHPriceFilter())
//
// filter: 带有代码的 CryptoPriceFilter (例如: "btcusdt")
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCryptoPrices(filter *CryptoPriceFilter, callback CryptoPriceCallback) error {
	if filter == nil {
		return fmt.Errorf("加密货币价格订阅需要过滤器")
	}

	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterCryptoPriceHandler(callback)
	}

	filterStr, err := filter.ToJSON()
	if err != nil {
		return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicCryptoPrices,
			Type:    MessageTypeUpdate,
			Filters: filterStr,
		},
	})
}

// SubscribeToCryptoPricesChainlink 订阅来自 Chainlink 的加密货币价格更新
//
// 重要提示: 与 SubscribeToCryptoPrices 相同的限制 - 每个连接仅限一个代码。
// 有关处理多个代码的详细信息，请参阅 SubscribeToCryptoPrices 文档。
//
// filter: 带有代码的 CryptoPriceFilter (例如: "btcusdt")
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCryptoPricesChainlink(filter *CryptoPriceFilter, callback CryptoPriceCallback) error {
	if filter == nil {
		return fmt.Errorf("加密货币价格 chainlink 订阅需要过滤器")
	}

	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterCryptoPriceHandler(callback)
	}

	filterStr, err := filter.ToJSON()
	if err != nil {
		return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicCryptoPricesChainlink,
			Type:    MessageTypeUpdate,
			Filters: filterStr,
		},
	})
}

// Equity Prices 订阅处理程序

// EquityPriceCallback 是股票价格更新消息的回调函数
type EquityPriceCallback func(price EquityPrice) error

// SubscribeToEquityPrices 订阅股票价格更新
//
// 重要提示: 与 SubscribeToCryptoPrices 相同的限制 - 每个连接仅限一个代码。
// 有关处理多个代码的详细信息，请参阅 SubscribeToCryptoPrices 文档。
//
// filter: 带有代码的 EquityPriceFilter (例如: "AAPL")
func (h *RealtimeTypedSubscriptionHandler) SubscribeToEquityPrices(filter *EquityPriceFilter, callback EquityPriceCallback) error {
	if filter == nil {
		return fmt.Errorf("股票价格订阅需要过滤器")
	}

	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterEquityPriceHandler(callback)
	}

	filterStr, err := filter.ToJSON()
	if err != nil {
		return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicEquityPrices,
			Type:    MessageTypeUpdate,
			Filters: filterStr,
		},
	})
}

// CLOB User 订阅处理程序

// CLOBOrderCallback 是 CLOB 订单消息的回调函数
type CLOBOrderCallback func(order CLOBOrder) error

// SubscribeToCLOBUserOrders 订阅 CLOB 用户订单
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCLOBUserOrders(auth *ClobAuth, callback CLOBOrderCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterCLOBOrderHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:    TopicClobUser,
			Type:     MessageTypeOrder,
			ClobAuth: auth,
		},
	})
}

// CLOBTradeCallback 是 CLOB 交易消息的回调函数
type CLOBTradeCallback func(trade CLOBTrade) error

// SubscribeToCLOBUserTrades 订阅 CLOB 用户交易
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCLOBUserTrades(auth *ClobAuth, callback CLOBTradeCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterCLOBTradeHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:    TopicClobUser,
			Type:     MessageTypeTrade,
			ClobAuth: auth,
		},
	})
}

// SubscribeToCLOBUserAll 订阅所有 CLOB 用户消息 (订单和交易)
//
// 注意: 此方法不接受回调参数。要处理消息，你需要
// 手动在路由器上注册处理程序:
//
//	typedSub.GetRouter().RegisterCLOBOrderHandler(func(order CLOBOrder) error { ... })
//	typedSub.GetRouter().RegisterCLOBTradeHandler(func(trade CLOBTrade) error { ... })
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCLOBUserAll(auth *ClobAuth) error {
	return h.client.Subscribe([]Subscription{
		{
			Topic:    TopicClobUser,
			Type:     MessageTypeAll,
			ClobAuth: auth,
		},
	})
}

// CLOB Market 订阅处理程序

// PriceChangesCallback 是价格变动消息的回调函数
type PriceChangesCallback func(changes PriceChanges) error

// SubscribeToCLOBMarketPriceChanges 订阅 CLOB 市场价格变动
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个代币 ID 并接收它们的所有数据。
//
// 过滤器格式: ["100","200",...] (代币 ID 字符串数组)
// filter: 带有代币 ID 的 CLOBMarketFilter (必需)
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCLOBMarketPriceChanges(filter *CLOBMarketFilter, callback PriceChangesCallback) error {
	if filter == nil {
		return fmt.Errorf("价格变动订阅需要过滤器")
	}

	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterPriceChangesHandler(callback)
	}

	filterStr, err := filter.ToJSON()
	if err != nil {
		return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicClobMarket,
			Type:    MessageTypePriceChange,
			Filters: filterStr,
		},
	})
}

// AggOrderbookCallback 是聚合订单簿消息的回调函数
type AggOrderbookCallback func(orderbook AggOrderbook) error

// SubscribeToCLOBMarketAggOrderbook 订阅 CLOB 市场聚合订单簿
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个代币 ID 并接收它们的所有数据。
//
// 过滤器格式: ["100","200",...] (代币 ID 字符串数组)
// filter: 带有代币 ID 的 CLOBMarketFilter (必需)
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCLOBMarketAggOrderbook(filter *CLOBMarketFilter, callback AggOrderbookCallback) error {
	if filter == nil {
		return fmt.Errorf("聚合订单簿订阅需要过滤器")
	}

	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterAggOrderbookHandler(callback)
	}

	filterStr, err := filter.ToJSON()
	if err != nil {
		return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicClobMarket,
			Type:    MessageTypeAggOrderbook,
			Filters: filterStr,
		},
	})
}

// LastTradePriceCallback 是最新成交价消息的回调函数
type LastTradePriceCallback func(price LastTradePrice) error

// SubscribeToCLOBMarketLastTradePrice 订阅 CLOB 市场最新成交价
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个代币 ID 并接收它们的所有数据。
//
// 过滤器格式: ["100","200",...] (代币 ID 字符串数组)
// filter: 带有代币 ID 的 CLOBMarketFilter (必需)
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCLOBMarketLastTradePrice(filter *CLOBMarketFilter, callback LastTradePriceCallback) error {
	if filter == nil {
		return fmt.Errorf("最新成交价订阅需要过滤器")
	}

	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterLastTradePriceHandler(callback)
	}

	filterStr, err := filter.ToJSON()
	if err != nil {
		return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicClobMarket,
			Type:    MessageTypeLastTradePrice,
			Filters: filterStr,
		},
	})
}

// TickSizeChangeCallback 是最小变动价位更改消息的回调函数
type TickSizeChangeCallback func(change TickSizeChange) error

// SubscribeToCLOBMarketTickSizeChange 订阅 CLOB 市场最小变动价位更改
//
// 此主题支持单个连接中的多个订阅。
// 你可以订阅多个代币 ID 并接收它们的所有数据。
//
// 过滤器格式: ["100","200",...] (代币 ID 字符串数组)
// filter: 带有代币 ID 的 CLOBMarketFilter (必需)
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCLOBMarketTickSizeChange(filter *CLOBMarketFilter, callback TickSizeChangeCallback) error {
	if filter == nil {
		return fmt.Errorf("最小变动价位更改订阅需要过滤器")
	}

	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterTickSizeChangeHandler(callback)
	}

	filterStr, err := filter.ToJSON()
	if err != nil {
		return fmt.Errorf("将过滤器转换为 JSON 失败: %w", err)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic:   TopicClobMarket,
			Type:    MessageTypeTickSizeChange,
			Filters: filterStr,
		},
	})
}

// ClobMarketCallback 是 CLOB 市场消息的回调函数
type ClobMarketCallback func(market ClobMarket) error

// SubscribeToCLOBMarketCreated 订阅 CLOB 市场创建事件
//
// 此主题支持单个连接中的多个订阅。
//
// 过滤器格式: 市场创建不需要过滤器
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCLOBMarketCreated(callback ClobMarketCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterClobMarketHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic: TopicClobMarket,
			Type:  MessageTypeMarketCreated,
		},
	})
}

// SubscribeToCLOBMarketResolved 订阅 CLOB 市场解决事件
//
// 此主题支持单个连接中的多个订阅。
//
// 过滤器格式: 市场解决不需要过滤器
func (h *RealtimeTypedSubscriptionHandler) SubscribeToCLOBMarketResolved(callback ClobMarketCallback) error {
	// 如果提供了回调，则将其注册到内部路由器
	if callback != nil {
		h.router.RegisterClobMarketHandler(callback)
	}

	return h.client.Subscribe([]Subscription{
		{
			Topic: TopicClobMarket,
			Type:  MessageTypeMarketResolved,
		},
	})
}

// RealtimeMessageRouter 将传入消息路由到注册的类型化回调
type RealtimeMessageRouter struct {
	// Activity 处理程序
	activityTradesHandlers        []ActivityTradesCallback
	activityOrdersMatchedHandlers []ActivityOrdersMatchedCallback

	// Comments 处理程序
	commentCreatedHandlers  []CommentCreatedCallback
	commentRemovedHandlers  []CommentRemovedCallback
	reactionCreatedHandlers []ReactionCreatedCallback
	reactionRemovedHandlers []ReactionRemovedCallback

	// RFQ 处理程序
	rfqRequestHandlers []RFQRequestCallback
	rfqQuoteHandlers   []RFQQuoteCallback

	// Price 处理程序
	cryptoPriceHandlers []CryptoPriceCallback
	equityPriceHandlers []EquityPriceCallback

	// CLOB User 处理程序
	clobOrderHandlers []CLOBOrderCallback
	clobTradeHandlers []CLOBTradeCallback

	// CLOB Market 处理程序
	priceChangesHandlers   []PriceChangesCallback
	aggOrderbookHandlers   []AggOrderbookCallback
	lastTradePriceHandlers []LastTradePriceCallback
	tickSizeChangeHandlers []TickSizeChangeCallback
	clobMarketHandlers     []ClobMarketCallback
}

// NewRealtimeMessageRouter 创建一个新的类型化消息路由器
func NewRealtimeMessageRouter() *RealtimeMessageRouter {
	return &RealtimeMessageRouter{}
}

// RegisterActivityTradesHandler 注册活动交易的处理程序
func (r *RealtimeMessageRouter) RegisterActivityTradesHandler(handler ActivityTradesCallback) {
	r.activityTradesHandlers = append(r.activityTradesHandlers, handler)
}

// RegisterActivityOrdersMatchedHandler 注册活动订单撮合的处理程序
func (r *RealtimeMessageRouter) RegisterActivityOrdersMatchedHandler(handler ActivityOrdersMatchedCallback) {
	r.activityOrdersMatchedHandlers = append(r.activityOrdersMatchedHandlers, handler)
}

// RegisterCommentCreatedHandler 注册评论创建事件的处理程序
func (r *RealtimeMessageRouter) RegisterCommentCreatedHandler(handler CommentCreatedCallback) {
	r.commentCreatedHandlers = append(r.commentCreatedHandlers, handler)
}

// RegisterCommentRemovedHandler 注册评论移除事件的处理程序
func (r *RealtimeMessageRouter) RegisterCommentRemovedHandler(handler CommentRemovedCallback) {
	r.commentRemovedHandlers = append(r.commentRemovedHandlers, handler)
}

// RegisterReactionCreatedHandler 注册反应创建事件的处理程序
func (r *RealtimeMessageRouter) RegisterReactionCreatedHandler(handler ReactionCreatedCallback) {
	r.reactionCreatedHandlers = append(r.reactionCreatedHandlers, handler)
}

// RegisterReactionRemovedHandler 注册反应移除事件的处理程序
func (r *RealtimeMessageRouter) RegisterReactionRemovedHandler(handler ReactionRemovedCallback) {
	r.reactionRemovedHandlers = append(r.reactionRemovedHandlers, handler)
}

// RegisterRFQRequestHandler 注册 RFQ 请求消息的处理程序
func (r *RealtimeMessageRouter) RegisterRFQRequestHandler(handler RFQRequestCallback) {
	r.rfqRequestHandlers = append(r.rfqRequestHandlers, handler)
}

// RegisterRFQQuoteHandler 注册 RFQ 报价消息的处理程序
func (r *RealtimeMessageRouter) RegisterRFQQuoteHandler(handler RFQQuoteCallback) {
	r.rfqQuoteHandlers = append(r.rfqQuoteHandlers, handler)
}

// RegisterCryptoPriceHandler 注册加密货币价格更新的处理程序
func (r *RealtimeMessageRouter) RegisterCryptoPriceHandler(handler CryptoPriceCallback) {
	r.cryptoPriceHandlers = append(r.cryptoPriceHandlers, handler)
}

// RegisterEquityPriceHandler 注册股票价格更新的处理程序
func (r *RealtimeMessageRouter) RegisterEquityPriceHandler(handler EquityPriceCallback) {
	r.equityPriceHandlers = append(r.equityPriceHandlers, handler)
}

// RegisterCLOBOrderHandler 注册 CLOB 订单消息的处理程序
func (r *RealtimeMessageRouter) RegisterCLOBOrderHandler(handler CLOBOrderCallback) {
	r.clobOrderHandlers = append(r.clobOrderHandlers, handler)
}

// RegisterCLOBTradeHandler 注册 CLOB 交易消息的处理程序
func (r *RealtimeMessageRouter) RegisterCLOBTradeHandler(handler CLOBTradeCallback) {
	r.clobTradeHandlers = append(r.clobTradeHandlers, handler)
}

// RegisterPriceChangesHandler 注册价格变动的处理程序
func (r *RealtimeMessageRouter) RegisterPriceChangesHandler(handler PriceChangesCallback) {
	r.priceChangesHandlers = append(r.priceChangesHandlers, handler)
}

// RegisterAggOrderbookHandler 注册聚合订单簿的处理程序
func (r *RealtimeMessageRouter) RegisterAggOrderbookHandler(handler AggOrderbookCallback) {
	r.aggOrderbookHandlers = append(r.aggOrderbookHandlers, handler)
}

// RegisterLastTradePriceHandler 注册最新成交价的处理程序
func (r *RealtimeMessageRouter) RegisterLastTradePriceHandler(handler LastTradePriceCallback) {
	r.lastTradePriceHandlers = append(r.lastTradePriceHandlers, handler)
}

// RegisterTickSizeChangeHandler 注册最小变动价位更改的处理程序
func (r *RealtimeMessageRouter) RegisterTickSizeChangeHandler(handler TickSizeChangeCallback) {
	r.tickSizeChangeHandlers = append(r.tickSizeChangeHandlers, handler)
}

// RegisterClobMarketHandler 注册 CLOB 市场事件的处理程序
func (r *RealtimeMessageRouter) RegisterClobMarketHandler(handler ClobMarketCallback) {
	r.clobMarketHandlers = append(r.clobMarketHandlers, handler)
}

// RouteMessage 将原始消息路由到适当的类型化处理程序
func (r *RealtimeMessageRouter) RouteMessage(data []byte) error {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		// 打印带有长度信息的原始数据以便调试
		maxLen := len(data)
		if maxLen > 200 {
			maxLen = 200
		}
		return fmt.Errorf("解析消息失败: %w (长度=%d, 数据=%q)", err, len(data), string(data[:maxLen]))
	}

	switch msg.Topic {
	case TopicActivity:
		return r.routeActivityMessage(msg)
	case TopicComments:
		return r.routeCommentsMessage(msg)
	case TopicRfq:
		return r.routeRFQMessage(msg)
	case TopicCryptoPrices, TopicCryptoPricesChainlink:
		return r.routeCryptoPriceMessage(msg)
	case TopicEquityPrices:
		return r.routeEquityPriceMessage(msg)
	case TopicClobUser:
		return r.routeCLOBUserMessage(msg)
	case TopicClobMarket:
		return r.routeCLOBMarketMessage(msg)
	default:
		return fmt.Errorf("未知主题: %s", msg.Topic)
	}
}

func (r *RealtimeMessageRouter) routeActivityMessage(msg Message) error {
	switch msg.Type {
	case MessageTypeTrades:
		var trade Trade
		if err := json.Unmarshal(msg.Payload, &trade); err != nil {
			return fmt.Errorf("解析交易失败: %w", err)
		}
		for _, handler := range r.activityTradesHandlers {
			if err := handler(trade); err != nil {
				return err
			}
		}
	case MessageTypeOrdersMatched:
		var trade Trade
		if err := json.Unmarshal(msg.Payload, &trade); err != nil {
			return fmt.Errorf("解析订单撮合失败: %w", err)
		}
		for _, handler := range r.activityOrdersMatchedHandlers {
			if err := handler(trade); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RealtimeMessageRouter) routeCommentsMessage(msg Message) error {
	switch msg.Type {
	case MessageTypeCommentCreated:
		var comment Comment
		if err := json.Unmarshal(msg.Payload, &comment); err != nil {
			return fmt.Errorf("解析评论失败: %w", err)
		}
		for _, handler := range r.commentCreatedHandlers {
			if err := handler(comment); err != nil {
				return err
			}
		}
	case MessageTypeCommentRemoved:
		var comment Comment
		if err := json.Unmarshal(msg.Payload, &comment); err != nil {
			return fmt.Errorf("解析评论失败: %w", err)
		}
		for _, handler := range r.commentRemovedHandlers {
			if err := handler(comment); err != nil {
				return err
			}
		}
	case MessageTypeReactionCreated:
		var reaction Reaction
		if err := json.Unmarshal(msg.Payload, &reaction); err != nil {
			return fmt.Errorf("解析反应失败: %w", err)
		}
		for _, handler := range r.reactionCreatedHandlers {
			if err := handler(reaction); err != nil {
				return err
			}
		}
	case MessageTypeReactionRemoved:
		var reaction Reaction
		if err := json.Unmarshal(msg.Payload, &reaction); err != nil {
			return fmt.Errorf("解析反应失败: %w", err)
		}
		for _, handler := range r.reactionRemovedHandlers {
			if err := handler(reaction); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RealtimeMessageRouter) routeRFQMessage(msg Message) error {
	switch msg.Type {
	case MessageTypeRequestCreated, MessageTypeRequestEdited, MessageTypeRequestCanceled, MessageTypeRequestExpired:
		var request RFQRequest
		if err := json.Unmarshal(msg.Payload, &request); err != nil {
			return fmt.Errorf("解析 RFQ 请求失败: %w", err)
		}
		for _, handler := range r.rfqRequestHandlers {
			if err := handler(request); err != nil {
				return err
			}
		}
	case MessageTypeQuoteCreated, MessageTypeQuoteEdited, MessageTypeQuoteCanceled, MessageTypeQuoteExpired:
		var quote RFQQuote
		if err := json.Unmarshal(msg.Payload, &quote); err != nil {
			return fmt.Errorf("解析 RFQ 报价失败: %w", err)
		}
		for _, handler := range r.rfqQuoteHandlers {
			if err := handler(quote); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RealtimeMessageRouter) routeCryptoPriceMessage(msg Message) error {
	if msg.Type == MessageTypeUpdate {
		var price CryptoPrice
		if err := json.Unmarshal(msg.Payload, &price); err != nil {
			return fmt.Errorf("解析加密货币价格失败: %w", err)
		}
		for _, handler := range r.cryptoPriceHandlers {
			if err := handler(price); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RealtimeMessageRouter) routeEquityPriceMessage(msg Message) error {
	if msg.Type == MessageTypeUpdate {
		var price EquityPrice
		if err := json.Unmarshal(msg.Payload, &price); err != nil {
			return fmt.Errorf("解析股票价格失败: %w", err)
		}
		for _, handler := range r.equityPriceHandlers {
			if err := handler(price); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RealtimeMessageRouter) routeCLOBUserMessage(msg Message) error {
	switch msg.Type {
	case MessageTypeOrder:
		var order CLOBOrder
		if err := json.Unmarshal(msg.Payload, &order); err != nil {
			return fmt.Errorf("解析 CLOB 订单失败: %w", err)
		}
		for _, handler := range r.clobOrderHandlers {
			if err := handler(order); err != nil {
				return err
			}
		}
	case MessageTypeTrade:
		var trade CLOBTrade
		if err := json.Unmarshal(msg.Payload, &trade); err != nil {
			return fmt.Errorf("解析 CLOB 交易失败: %w", err)
		}
		for _, handler := range r.clobTradeHandlers {
			if err := handler(trade); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RealtimeMessageRouter) routeCLOBMarketMessage(msg Message) error {
	switch msg.Type {
	case MessageTypePriceChange:
		var changes PriceChanges
		if err := json.Unmarshal(msg.Payload, &changes); err != nil {
			return fmt.Errorf("解析价格变动失败: %w", err)
		}
		for _, handler := range r.priceChangesHandlers {
			if err := handler(changes); err != nil {
				return err
			}
		}
	case MessageTypeAggOrderbook:
		var orderbook AggOrderbook
		if err := json.Unmarshal(msg.Payload, &orderbook); err != nil {
			return fmt.Errorf("解析聚合订单簿失败: %w", err)
		}
		for _, handler := range r.aggOrderbookHandlers {
			if err := handler(orderbook); err != nil {
				return err
			}
		}
	case MessageTypeLastTradePrice:
		var lastPrice LastTradePrice
		if err := json.Unmarshal(msg.Payload, &lastPrice); err != nil {
			return fmt.Errorf("解析最新成交价失败: %w", err)
		}
		for _, handler := range r.lastTradePriceHandlers {
			if err := handler(lastPrice); err != nil {
				return err
			}
		}
	case MessageTypeTickSizeChange:
		var change TickSizeChange
		if err := json.Unmarshal(msg.Payload, &change); err != nil {
			return fmt.Errorf("解析最小变动价位更改失败: %w", err)
		}
		for _, handler := range r.tickSizeChangeHandlers {
			if err := handler(change); err != nil {
				return err
			}
		}
	case MessageTypeMarketCreated, MessageTypeMarketResolved:
		var market ClobMarket
		if err := json.Unmarshal(msg.Payload, &market); err != nil {
			return fmt.Errorf("解析 CLOB 市场失败: %w", err)
		}
		for _, handler := range r.clobMarketHandlers {
			if err := handler(market); err != nil {
				return err
			}
		}
	}
	return nil
}

// NormalizedTime 是一个自定义时间类型，可以解析多种时间格式
type NormalizedTime time.Time

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (ct *NormalizedTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		*ct = NormalizedTime(time.Time{})
		return nil
	}

	// 标准化时区偏移格式: 将 "+00" 转换为 "+00:00" 以供 Go 的 time.Parse 使用
	// API 有时返回 "2020-11-02 16:31:01+00" 而不是 "2020-11-02 16:31:01+00:00"
	// 仅当其以 +XX 或 -XX (2 位数字) 结尾且尚未采用 +XX:00 格式时才标准化
	if len(s) >= 3 && strings.Contains(s, ":") {
		last3 := s[len(s)-3:]
		// 检查它是否以 +XX 或 -XX (2 位数字) 结尾
		if (last3[0] == '+' || last3[0] == '-') && last3[1] >= '0' && last3[1] <= '9' && last3[2] >= '0' && last3[2] <= '9' {
			// 检查它是否尚未采用 +XX:00 格式 (检查最后 6 个字符)
			if len(s) < 6 || s[len(s)-6:] != last3+":00" {
				// 不是 +XX:00 格式，将 "+00" 转换为 "+00:00"
				s = s[:len(s)-3] + last3 + ":00"
			}
		}
	}

	// 尝试不同的时间格式
	// 注意: 应首先尝试 RFC3339 格式 (例如: "2024-11-06T15:17:41Z")
	formats := []string{
		time.RFC3339,                     // "2006-01-02T15:04:05Z07:00" 或 "2006-01-02T15:04:05Z"
		time.RFC3339Nano,                 // "2006-01-02T15:04:05.999999999Z07:00"
		"2006-01-02T15:04:05Z",           // 显式 Z 格式 (例如: "2024-11-06T15:17:41Z")
		"2006-01-02T15:04:05.999999999Z", // 带有纳秒和 Z
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999+00:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05+00:00", // 类似于 "2020-11-02 16:31:01+00:00" 的格式 (已标准化)
		"2006-01-02",                // 简单日期格式 (YYYY-MM-DD)
		"January 2, 2006",           // 长月份名称格式 (例如: "November 1, 2022")
	}

	var err error
	var t time.Time
	for _, format := range formats {
		t, err = time.Parse(format, s)
		if err == nil {
			*ct = NormalizedTime(t)
			return nil
		}
	}

	// 如果所有格式都失败，则返回最后一个错误
	return err
}

// MarshalJSON 实现 json.Marshaler 接口
func (ct NormalizedTime) MarshalJSON() ([]byte, error) {
	t := time.Time(ct)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Format(time.RFC3339))
}

// Time type 返回底层的 time.Time 值
func (ct NormalizedTime) Time() time.Time {
	return time.Time(ct)
}

// IsZero 报告 ct 是否表示零时刻
func (ct NormalizedTime) IsZero() bool {
	return time.Time(ct).IsZero()
}

// Format 返回时间值的文本表示形式
func (ct NormalizedTime) Format(layout string) string {
	return time.Time(ct).Format(layout)
}

func (ct NormalizedTime) String() string {
	return ct.Time().String()
}

// StringOrArray 是一个自定义类型，可以从 JSON 解析字符串和数组
// API 有时将这些字段作为字符串返回，有时作为数组返回
type StringOrArray []string

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (sa *StringOrArray) UnmarshalJSON(b []byte) error {
	// 显式处理 null
	if string(b) == "null" {
		*sa = StringOrArray([]string{})
		return nil
	}

	// 首先尝试解析为一维数组
	var arr []string
	if err := json.Unmarshal(b, &arr); err == nil {
		// 确保非 nil 切片 (在 null 情况下)
		if arr == nil {
			*sa = StringOrArray([]string{})
			return nil
		}

		// 检查是否有任何元素是 JSON 数组字符串 (例如: "[\"Up\", \"Down\"]")
		// 这处理 API 返回包含 JSON 编码数组字符串的数组的情况
		var flattened []string
		needsFlattening := false
		for _, s := range arr {
			// 检查字符串是否看起来像 JSON 数组
			if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
				// 尝试把字符串解析为 JSON 数组
				var innerArr []string
				if err := json.Unmarshal([]byte(s), &innerArr); err == nil {
					flattened = append(flattened, innerArr...)
					needsFlattening = true
					continue
				}

				// 尝试解析为二维数组并展平
				var innerArr2D [][]string
				if err := json.Unmarshal([]byte(s), &innerArr2D); err == nil {
					for _, innerArr := range innerArr2D {
						flattened = append(flattened, innerArr...)
					}
					needsFlattening = true
					continue
				}
			}

			// 不是 JSON 数组字符串，保持不变
			flattened = append(flattened, s)
		}

		if needsFlattening {
			*sa = StringOrArray(flattened)
		} else {
			*sa = StringOrArray(arr)
		}
		return nil
	}

	// 尝试解析为二维数组并将其展平为一维
	var arr2D [][]string
	if err := json.Unmarshal(b, &arr2D); err == nil {
		// 将二维数组展平为一维
		var flattened []string
		for _, innerArr := range arr2D {
			flattened = append(flattened, innerArr...)
		}
		*sa = StringOrArray(flattened)
		return nil
	}

	// 如果失败，尝试作为字符串解析
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if s == "" {
			*sa = StringOrArray([]string{})
			return nil
		}

		// 检查字符串是否是 JSON 数组 (例如: "[\"Yes\", \"No\"]")
		// 这处理 API 将 JSON 编码的数组作为字符串返回的情况
		if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
			// 尝试把字符串解析为 JSON 数组
			var arr []string
			if err := json.Unmarshal([]byte(s), &arr); err == nil {
				*sa = StringOrArray(arr)
				return nil
			}

			// 尝试解析为二维数组并展平
			var arr2D [][]string
			if err := json.Unmarshal([]byte(s), &arr2D); err == nil {
				var flattened []string
				for _, innerArr := range arr2D {
					flattened = append(flattened, innerArr...)
				}
				*sa = StringOrArray(flattened)
				return nil
			}
		}

		// 如果不是 JSON 数组，则视为单个字符串值
		*sa = StringOrArray([]string{s})
		return nil
	}

	// 默认为空数组
	*sa = StringOrArray([]string{})
	return nil
}

// MarshalJSON 实现 json.Marshaler 接口
func (sa StringOrArray) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string(sa))
}

// HealthResponse 表示健康检查端点的响应
type HealthResponse struct {
	Data string `json:"data"`
}

// GetEventsParams 表示获取事件列表的参数
type GetEventsParams struct {
	Limit           int             `json:"limit,omitempty"`  // 要返回的最大事件数
	Offset          int             `json:"offset,omitempty"` // 分页偏移量
	Order           string          `json:"order,omitempty"`  // 按逗号分隔的字段列表排序
	Ascending       *bool           `json:"ascending,omitempty"`
	ID              []int           `json:"id,omitempty"`
	Slug            []string        `json:"slug,omitempty"`
	TagID           *int            `json:"tag_id,omitempty"`
	ExcludeTagID    []int           `json:"exclude_tag_id,omitempty"`
	RelatedTags     *bool           `json:"related_tags,omitempty"`
	Featured        *bool           `json:"featured,omitempty"`
	CYOM            *bool           `json:"cyom,omitempty"`
	IncludeChat     *bool           `json:"include_chat,omitempty"`
	IncludeTemplate *bool           `json:"include_template,omitempty"`
	Recurrence      string          `json:"recurrence,omitempty"`
	Closed          *bool           `json:"closed,omitempty"`
	StartDateMin    *NormalizedTime `json:"start_date_min,omitempty"` // ISO 8601 日期时间
	StartDateMax    *NormalizedTime `json:"start_date_max,omitempty"` // ISO 8601 日期时间
	EndDateMin      *NormalizedTime `json:"end_date_min,omitempty"`   // ISO 8601 日期时间
	EndDateMax      *NormalizedTime `json:"end_date_max,omitempty"`   // ISO 8601 日期时间
}

// GetMarketsParams 表示获取市场列表的参数
type GetMarketsParams struct {
	Limit               int             `json:"limit,omitempty"`
	Offset              int             `json:"offset,omitempty"`
	Order               string          `json:"order,omitempty"`          // 按逗号分隔的字段列表排序
	Ascending           *bool           `json:"ascending,omitempty"`      // 使用指针区分 false 和未设置
	ID                  []int           `json:"id,omitempty"`             // 市场 ID
	Slug                []string        `json:"slug,omitempty"`           // 市场 slug
	ClobTokenIDs        []string        `json:"clob_token_ids,omitempty"` // CLOB 代币 ID
	ConditionIDs        []string        `json:"condition_ids,omitempty"`  // 条件 ID
	MarketMakerAddress  []string        `json:"market_maker_address,omitempty"`
	LiquidityNumMin     *float64        `json:"liquidity_num_min,omitempty"`
	LiquidityNumMax     *float64        `json:"liquidity_num_max,omitempty"`
	VolumeNumMin        *float64        `json:"volume_num_min,omitempty"`
	VolumeNumMax        *float64        `json:"volume_num_max,omitempty"`
	StartDateMin        *NormalizedTime `json:"start_date_min,omitempty"` // ISO 8601 日期时间
	StartDateMax        *NormalizedTime `json:"start_date_max,omitempty"` // ISO 8601 日期时间
	EndDateMin          *NormalizedTime `json:"end_date_min,omitempty"`   // ISO 8601 日期时间
	EndDateMax          *NormalizedTime `json:"end_date_max,omitempty"`   // ISO 8601 日期时间
	TagID               *int            `json:"tag_id,omitempty"`
	RelatedTags         *bool           `json:"related_tags,omitempty"`
	CYOM                *bool           `json:"cyom,omitempty"`
	UMAResolutionStatus string          `json:"uma_resolution_status,omitempty"`
	GameID              string          `json:"game_id,omitempty"`
	SportsMarketTypes   []string        `json:"sports_market_types,omitempty"`
	RewardsMinSize      *float64        `json:"rewards_min_size,omitempty"`
	QuestionIDs         []string        `json:"question_ids,omitempty"`
	IncludeTag          *bool           `json:"include_tag,omitempty"`
	Closed              *bool           `json:"closed,omitempty"`
}

// GetMarketByIDQueryParams 表示通过 ID 获取单个市场的查询参数
type GetMarketByIDQueryParams struct {
	IncludeTag *bool `json:"include_tag,omitempty"` // 在响应中包含标签信息
}

// GetSeriesParams 表示获取系列列表的参数
type GetSeriesParams struct {
	Limit            int      `json:"limit,omitempty"`  // 要返回的最大系列数
	Offset           int      `json:"offset,omitempty"` // 分页偏移量
	Order            string   `json:"order,omitempty"`  // 按逗号分隔的字段列表排序
	Ascending        *bool    `json:"ascending,omitempty"`
	Slug             []string `json:"slug,omitempty"`
	CategoriesIDs    []int    `json:"categories_ids,omitempty"`
	CategoriesLabels []string `json:"categories_labels,omitempty"`
	Closed           *bool    `json:"closed,omitempty"`
	IncludeChat      *bool    `json:"include_chat,omitempty"`
	Recurrence       string   `json:"recurrence,omitempty"`
}

// GetEventByIDQueryParams 表示通过 ID 获取单个事件的查询参数
type GetEventByIDQueryParams struct {
	IncludeChat     *bool `json:"include_chat,omitempty"`     // 在响应中包含聊天信息
	IncludeTemplate *bool `json:"include_template,omitempty"` // 在响应中包含模板信息
}

// GetEventBySlugQueryParams 表示通过 slug 获取单个事件的查询参数
type GetEventBySlugQueryParams struct {
	IncludeChat     *bool `json:"include_chat,omitempty"`     // 在响应中包含聊天信息
	IncludeTemplate *bool `json:"include_template,omitempty"` // 在响应中包含模板信息
}

// GetSeriesByIDQueryParams 表示通过 ID 获取单个系列的查询参数
type GetSeriesByIDQueryParams struct {
	IncludeChat *bool `json:"include_chat,omitempty"` // 在响应中包含聊天信息
}

// GetTeamsParams 表示获取团队的查询参数
type GetTeamsParams struct {
	Limit        int      `json:"limit,omitempty"`        // 要返回的结果数 (x >= 0)
	Offset       int      `json:"offset,omitempty"`       // 要跳过的结果数 (x >= 0)
	Order        string   `json:"order,omitempty"`        // 按逗号分隔的字段列表排序
	Ascending    *bool    `json:"ascending,omitempty"`    // 排序顺序
	League       []string `json:"league,omitempty"`       // 按联盟过滤
	Name         []string `json:"name,omitempty"`         // 按名称过滤
	Abbreviation []string `json:"abbreviation,omitempty"` // 按缩写过滤
}

// GetTagsParams 表示获取标签的查询参数
type GetTagsParams struct {
	Limit           int    `json:"limit,omitempty"`            // 要返回的结果数 (x >= 0)
	Offset          int    `json:"offset,omitempty"`           // 要跳过的结果数 (x >= 0)
	Order           string `json:"order,omitempty"`            // 按逗号分隔的字段列表排序
	Ascending       *bool  `json:"ascending,omitempty"`        // 排序顺序
	IncludeTemplate *bool  `json:"include_template,omitempty"` // 包含模板信息
	IsCarousel      *bool  `json:"is_carousel,omitempty"`      // 按轮播状态过滤
}

// GetTagByIDQueryParams 表示通过 ID 获取单个标签的查询参数
type GetTagByIDQueryParams struct {
	IncludeTemplate *bool `json:"include_template,omitempty"` // 包含模板信息
}

// GetTagBySlugQueryParams 表示通过 slug 获取单个标签的查询参数
type GetTagBySlugQueryParams struct {
	IncludeTemplate *bool `json:"include_template,omitempty"` // 包含模板信息
}

// GetRelatedTagsParams 表示获取相关标签的查询参数
type GetRelatedTagsParams struct {
	OmitEmpty *bool     `json:"omit_empty,omitempty"` // 省略空关系
	Status    TagStatus `json:"status,omitempty"`     // 状态过滤器 (active, closed, all)
}

// SearchParams 表示搜索市场、事件和个人资料的参数
type SearchParams struct {
	Q                 string   `json:"q"`                             // 搜索查询 (必需)
	Cache             *bool    `json:"cache,omitempty"`               // 使用缓存
	EventsStatus      string   `json:"events_status,omitempty"`       // 事件状态过滤器
	LimitPerType      *int     `json:"limit_per_type,omitempty"`      // 每种类型的限制结果
	Page              *int     `json:"page,omitempty"`                // 分页页码
	EventsTag         []string `json:"events_tag,omitempty"`          // 按事件标签过滤
	KeepClosedMarkets *int     `json:"keep_closed_markets,omitempty"` // 保留已关闭的市场 (0 或 1)
	Sort              string   `json:"sort,omitempty"`                // 排序字段
	Ascending         *bool    `json:"ascending,omitempty"`           // 排序顺序
	SearchTags        *bool    `json:"search_tags,omitempty"`         // 在搜索中包含标签
	SearchProfiles    *bool    `json:"search_profiles,omitempty"`     // 在搜索中包含个人资料
	Recurrence        string   `json:"recurrence,omitempty"`          // 循环过滤器
	ExcludeTagID      []int    `json:"exclude_tag_id,omitempty"`      // 排除标签 ID
	Optimized         *bool    `json:"optimized,omitempty"`           // 返回优化后的图像
}

// SearchResponse 表示搜索端点的响应
type SearchResponse struct {
	Events     []Event     `json:"events"`
	Tags       []SearchTag `json:"tags"`
	Profiles   []Profile   `json:"profiles"`
	Pagination Pagination  `json:"pagination"`
}

// SearchTag 表示搜索结果中的标签
type SearchTag struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Slug       string `json:"slug"`
	EventCount int    `json:"event_count"`
}

// Profile 表示用户个人资料
type Profile struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	User                  int             `json:"user"`
	Referral              string          `json:"referral"`
	CreatedBy             int             `json:"createdBy"`
	UpdatedBy             int             `json:"updatedBy"`
	CreatedAt             NormalizedTime  `json:"createdAt"`
	UpdatedAt             NormalizedTime  `json:"updatedAt"`
	UTMSource             string          `json:"utmSource"`
	UTMMedium             string          `json:"utmMedium"`
	UTMCampaign           string          `json:"utmCampaign"`
	UTMContent            string          `json:"utmContent"`
	UTMTerm               string          `json:"utmTerm"`
	WalletActivated       bool            `json:"walletActivated"`
	Pseudonym             string          `json:"pseudonym"`
	DisplayUsernamePublic bool            `json:"displayUsernamePublic"`
	ProfileImage          string          `json:"profileImage"`
	Bio                   string          `json:"bio"`
	ProxyWallet           string          `json:"proxyWallet"`
	ProfileImageOptimized *ImageOptimized `json:"profileImageOptimized,omitempty"`
	IsCloseOnly           bool            `json:"isCloseOnly"`
	IsCertReq             bool            `json:"isCertReq"`
	CertReqDate           NormalizedTime  `json:"certReqDate"`
}

// Pagination 表示分页信息
type Pagination struct {
	HasMore      bool `json:"hasMore"`
	TotalResults int  `json:"totalResults"`
}

// Market 表示来自 Gamma API /markets 端点的市场
// 这包含超出 CLOB API Market 类型的大量元数据
type Market struct {
	// 核心市场标识符
	ID          string `json:"id"`
	Question    string `json:"question"`
	ConditionID string `json:"conditionId"`
	Slug        string `json:"slug"`
	QuestionID  string `json:"questionID"`

	// 显示和品牌
	TwitterCardImage string `json:"twitterCardImage"`
	Image            string `json:"image"`
	Icon             string `json:"icon"`
	Description      string `json:"description"`

	// 解决和时间
	ResolutionSource string         `json:"resolutionSource"`
	EndDate          NormalizedTime `json:"endDate"`
	StartDate        NormalizedTime `json:"startDate"`
	EndDateISO       string         `json:"endDateIso"`
	StartDateISO     string         `json:"startDateIso"`
	UMAEndDate       NormalizedTime `json:"umaEndDate"`
	UMAEndDateISO    string         `json:"umaEndDateIso"`
	ClosedTime       NormalizedTime `json:"closedTime"`

	// 市场机制
	Category          string  `json:"category"`
	AmmType           string  `json:"ammType"`
	Liquidity         string  `json:"liquidity"`
	LiquidityNum      float64 `json:"liquidityNum"`
	Volume            string  `json:"volume"`
	VolumeNum         float64 `json:"volumeNum"`
	Fee               string  `json:"fee"`
	DenominationToken string  `json:"denominationToken"`

	// 赞助商信息
	SponsorName  string `json:"sponsorName"`
	SponsorImage string `json:"sponsorImage"`

	// 图表配置
	XAxisValue string `json:"xAxisValue"`
	YAxisValue string `json:"yAxisValue"`
	LowerBound string `json:"lowerBound"`
	UpperBound string `json:"upperBound"`

	// 结果和定价
	// Outcomes 表示市场的可能结果。
	// API 将其作为 JSON 编码的字符串返回 (例如: "[\"Yes\", \"No\"]")，该字符串会自动解析为字符串数组。
	// 示例: 二元市场为 ["Yes", "No"]，分类市场为 ["Option A", "Option B", "Option C"]。
	Outcomes StringOrArray `json:"outcomes"`
	// OutcomePrices 表示每个结果的当前价格，通常为十进制字符串。
	// API 将其作为 JSON 编码的字符串返回 (例如: "[\"0.52\", \"0.48\"]")，该字符串会自动解析为字符串数组。
	// 示例: ["0.52", "0.48"] 用于二元市场，其中价格总和为 1.0。
	OutcomePrices StringOrArray `json:"outcomePrices"`
	ShortOutcomes StringOrArray `json:"shortOutcomes"`

	// 状态标志
	Active     bool `json:"active"`
	Closed     bool `json:"closed"`
	Archived   bool `json:"archived"`
	New        bool `json:"new"`
	Featured   bool `json:"featured"`
	Restricted bool `json:"restricted"`
	WideFormat bool `json:"wideFormat"`
	Ready      bool `json:"ready"`
	Funded     bool `json:"funded"`

	// 市场类型和格式
	MarketType string `json:"marketType"`
	FormatType string `json:"formatType"`

	// 日期边界
	LowerBoundDate NormalizedTime `json:"lowerBoundDate"`
	UpperBoundDate NormalizedTime `json:"upperBoundDate"`

	// 合约和交换
	MarketMakerAddress string `json:"marketMakerAddress"`

	// 元数据
	CreatedBy int            `json:"createdBy"`
	UpdatedBy int            `json:"updatedBy"`
	CreatedAt NormalizedTime `json:"createdAt"`
	UpdatedAt NormalizedTime `json:"updatedAt"`

	// 营销
	MailchimpTag string `json:"mailchimpTag"`
	ResolvedBy   string `json:"resolvedBy"`

	// 分组
	MarketGroup        int    `json:"marketGroup"`
	GroupItemTitle     string `json:"groupItemTitle"`
	GroupItemThreshold string `json:"groupItemThreshold"`
	GroupItemRange     string `json:"groupItemRange"`

	// UMA 解决
	UMAResolutionStatus   string `json:"umaResolutionStatus"`
	UMAResolutionStatuses string `json:"umaResolutionStatuses"`
	UMABond               string `json:"umaBond"`
	UMAReward             string `json:"umaReward"`

	// 订单簿配置
	EnableOrderBook       bool    `json:"enableOrderBook"`
	OrderPriceMinTickSize float64 `json:"orderPriceMinTickSize"`
	OrderMinSize          float64 `json:"orderMinSize"`
	MakerBaseFee          int     `json:"makerBaseFee"`
	TakerBaseFee          int     `json:"takerBaseFee"`
	AcceptingOrders       bool    `json:"acceptingOrders"`
	NotificationsEnabled  bool    `json:"notificationsEnabled"`

	// 策展和评分
	CurationOrder int     `json:"curationOrder"`
	Score         float64 `json:"score"`

	// 审查状态
	HasReviewedDates bool `json:"hasReviewedDates"`
	ReadyForCron     bool `json:"readyForCron"`
	CommentsEnabled  bool `json:"commentsEnabled"`

	// 交易量指标
	Volume24hr     float64 `json:"volume24hr"`
	Volume1wk      float64 `json:"volume1wk"`
	Volume1mo      float64 `json:"volume1mo"`
	Volume1yr      float64 `json:"volume1yr"`
	Volume24hrAmm  float64 `json:"volume24hrAmm"`
	Volume1wkAmm   float64 `json:"volume1wkAmm"`
	Volume1moAmm   float64 `json:"volume1moAmm"`
	Volume1yrAmm   float64 `json:"volume1yrAmm"`
	Volume24hrClob float64 `json:"volume24hrClob"`
	Volume1wkClob  float64 `json:"volume1wkClob"`
	Volume1moClob  float64 `json:"volume1moClob"`
	Volume1yrClob  float64 `json:"volume1yrClob"`
	VolumeAmm      float64 `json:"volumeAmm"`
	VolumeClob     float64 `json:"volumeClob"`

	// 流动性细分
	LiquidityAmm  float64 `json:"liquidityAmm"`
	LiquidityClob float64 `json:"liquidityClob"`

	// 游戏/体育特定
	GameStartTime    NormalizedTime `json:"gameStartTime"`
	SecondsDelay     int            `json:"secondsDelay"`
	ClobTokenIDs     StringOrArray  `json:"clobTokenIds"`
	TeamAID          string         `json:"teamAID"`
	TeamBID          string         `json:"teamBID"`
	GameID           string         `json:"gameId"`
	SportsMarketType string         `json:"sportsMarketType"`
	Line             float64        `json:"line"`

	// 讨论
	DisqusThread string `json:"disqusThread"`

	// AMM 状态
	FPMMLive bool `json:"fpmmLive"`

	// 自定义设置
	CustomLiveness int `json:"customLiveness"`

	// 奖励
	RewardsMinSize   float64 `json:"rewardsMinSize"`
	RewardsMaxSpread float64 `json:"rewardsMaxSpread"`

	// 图像优化
	ImageOptimized *ImageOptimized `json:"imageOptimized,omitempty"`
	IconOptimized  *ImageOptimized `json:"iconOptimized,omitempty"`

	// 相关实体
	Events     []Event    `json:"events,omitempty"`
	Categories []Category `json:"categories,omitempty"`
	Tags       []Tag      `json:"tags,omitempty"`

	// 创建者和元数据
	Creator                  string         `json:"creator"`
	PastSlugs                string         `json:"pastSlugs"`
	ReadyTimestamp           NormalizedTime `json:"readyTimestamp"`
	FundedTimestamp          NormalizedTime `json:"fundedTimestamp"`
	AcceptingOrdersTimestamp NormalizedTime `json:"acceptingOrdersTimestamp"`

	// 竞争
	Competitive float64 `json:"competitive"`

	// 点差信息
	Spread float64 `json:"spread"`

	// 解决标志
	AutomaticallyResolved bool `json:"automaticallyResolved"`

	// 价格变动
	OneDayPriceChange   float64 `json:"oneDayPriceChange"`
	OneHourPriceChange  float64 `json:"oneHourPriceChange"`
	OneWeekPriceChange  float64 `json:"oneWeekPriceChange"`
	OneMonthPriceChange float64 `json:"oneMonthPriceChange"`
	OneYearPriceChange  float64 `json:"oneYearPriceChange"`

	// 当前价格
	LastTradePrice float64 `json:"lastTradePrice"`
	BestBid        float64 `json:"bestBid"`
	BestAsk        float64 `json:"bestAsk"`

	// 激活
	AutomaticallyActive bool `json:"automaticallyActive"`
	ClearBookOnStart    bool `json:"clearBookOnStart"`
	ManualActivation    bool `json:"manualActivation"`

	// 图表样式
	ChartColor     string `json:"chartColor"`
	SeriesColor    string `json:"seriesColor"`
	ShowGmpSeries  bool   `json:"showGmpSeries"`
	ShowGmpOutcome bool   `json:"showGmpOutcome"`

	// 负风险
	NegRiskOther bool `json:"negRiskOther"`

	// 部署状态
	PendingDeployment            bool           `json:"pendingDeployment"`
	Deploying                    bool           `json:"deploying"`
	DeployingTimestamp           NormalizedTime `json:"deployingTimestamp"`
	ScheduledDeploymentTimestamp NormalizedTime `json:"scheduledDeploymentTimestamp"`

	// RFQ
	RFQEnabled bool `json:"rfqEnabled"`

	// 事件时间 (为了向后兼容，可能为空/null)
	EventStartTime NormalizedTime `json:"eventStartTime"`
}

// ImageOptimized 表示优化的图像资源
type ImageOptimized struct {
	ID                        string         `json:"id"`
	ImageURLSource            string         `json:"imageUrlSource"`
	ImageURLOptimized         string         `json:"imageUrlOptimized"`
	ImageSizeKBSource         int            `json:"imageSizeKbSource"`
	ImageSizeKBOptimized      int            `json:"imageSizeKbOptimized"`
	ImageOptimizedComplete    bool           `json:"imageOptimizedComplete"`
	ImageOptimizedLastUpdated NormalizedTime `json:"imageOptimizedLastUpdated"`
	RelID                     int            `json:"relID"`
	Field                     string         `json:"field"`
	Relname                   string         `json:"relname"`
}

// Event 表示来自 Gamma API 的事件
type Event struct {
	ID                string         `json:"id"`
	Ticker            string         `json:"ticker"`
	Slug              string         `json:"slug"`
	Title             string         `json:"title"`
	Subtitle          string         `json:"subtitle"`
	Description       string         `json:"description"`
	ResolutionSource  string         `json:"resolutionSource"`
	StartDate         NormalizedTime `json:"startDate"`
	CreationDate      NormalizedTime `json:"creationDate"`
	EndDate           NormalizedTime `json:"endDate"`
	Image             string         `json:"image"`
	Icon              string         `json:"icon"`
	Active            bool           `json:"active"`
	Closed            bool           `json:"closed"`
	Archived          bool           `json:"archived"`
	New               bool           `json:"new"`
	Featured          bool           `json:"featured"`
	Restricted        bool           `json:"restricted"`
	Liquidity         float64        `json:"liquidity"`
	Volume            float64        `json:"volume"`
	OpenInterest      float64        `json:"openInterest"`
	SortBy            string         `json:"sortBy"`
	Category          string         `json:"category"`
	Subcategory       string         `json:"subcategory"`
	IsTemplate        bool           `json:"isTemplate"`
	TemplateVariables string         `json:"templateVariables"`
	PublishedAt       NormalizedTime `json:"published_at"`
	CreatedBy         string         `json:"createdBy"`
	UpdatedBy         string         `json:"updatedBy"`
	CreatedAt         NormalizedTime `json:"createdAt"`
	UpdatedAt         NormalizedTime `json:"updatedAt"`
	CommentsEnabled   bool           `json:"commentsEnabled"`
	Competitive       float64        `json:"competitive"`
	Volume24hr        float64        `json:"volume24hr"`
	Volume1wk         float64        `json:"volume1wk"`
	Volume1mo         float64        `json:"volume1mo"`
	Volume1yr         float64        `json:"volume1yr"`
	FeaturedImage     string         `json:"featuredImage"`
	DisqusThread      string         `json:"disqusThread"`
	ParentEvent       string         `json:"parentEvent"`
	EnableOrderBook   bool           `json:"enableOrderBook"`
	LiquidityAmm      float64        `json:"liquidityAmm"`
	LiquidityClob     float64        `json:"liquidityClob"`
	NegRisk           bool           `json:"negRisk"`
	NegRiskMarketID   string         `json:"negRiskMarketID"`
	NegRiskFeeBips    int            `json:"negRiskFeeBips"`
	CommentCount      int            `json:"commentCount"`

	// 优化图像
	ImageOptimized         *ImageOptimized `json:"imageOptimized,omitempty"`
	IconOptimized          *ImageOptimized `json:"iconOptimized,omitempty"`
	FeaturedImageOptimized *ImageOptimized `json:"featuredImageOptimized,omitempty"`

	// 相关实体
	SubEvents   []string     `json:"subEvents,omitempty"`
	Markets     []Market     `json:"markets,omitempty"`
	Series      []Series     `json:"series,omitempty"`
	Categories  []Category   `json:"categories,omitempty"`
	Collections []Collection `json:"collections,omitempty"`
	Tags        []Tag        `json:"tags,omitempty"`

	// 附加字段
	CYOM                         bool           `json:"cyom"`
	ClosedTime                   NormalizedTime `json:"closedTime"`
	ShowAllOutcomes              bool           `json:"showAllOutcomes"`
	ShowMarketImages             bool           `json:"showMarketImages"`
	AutomaticallyResolved        bool           `json:"automaticallyResolved"`
	EnableNegRisk                bool           `json:"enableNegRisk"`
	AutomaticallyActive          bool           `json:"automaticallyActive"`
	EventDate                    NormalizedTime `json:"eventDate"`
	StartTime                    NormalizedTime `json:"startTime"`
	EventWeek                    int            `json:"eventWeek"`
	SeriesSlug                   string         `json:"seriesSlug"`
	Score                        string         `json:"score"`
	Elapsed                      string         `json:"elapsed"`
	Period                       string         `json:"period"`
	Live                         bool           `json:"live"`
	Ended                        bool           `json:"ended"`
	FinishedTimestamp            NormalizedTime `json:"finishedTimestamp"`
	GMPChartMode                 string         `json:"gmpChartMode"`
	EventCreators                []EventCreator `json:"eventCreators,omitempty"`
	TweetCount                   int            `json:"tweetCount"`
	Chats                        []Chat         `json:"chats,omitempty"`
	FeaturedOrder                int            `json:"featuredOrder"`
	EstimateValue                bool           `json:"estimateValue"`
	CantEstimate                 bool           `json:"cantEstimate"`
	EstimatedValue               string         `json:"estimatedValue"`
	Templates                    []Template     `json:"templates,omitempty"`
	SpreadsMainLine              float64        `json:"spreadsMainLine"`
	TotalsMainLine               float64        `json:"totalsMainLine"`
	CarouselMap                  string         `json:"carouselMap"`
	PendingDeployment            bool           `json:"pendingDeployment"`
	Deploying                    bool           `json:"deploying"`
	DeployingTimestamp           NormalizedTime `json:"deployingTimestamp"`
	ScheduledDeploymentTimestamp NormalizedTime `json:"scheduledDeploymentTimestamp"`
	GameStatus                   string         `json:"gameStatus"`
}

// Category 表示类别或子类别
type Category struct {
	ID             string         `json:"id"`
	Label          string         `json:"label"`
	ParentCategory string         `json:"parentCategory"`
	Slug           string         `json:"slug"`
	PublishedAt    NormalizedTime `json:"publishedAt"`
	CreatedBy      string         `json:"createdBy"`
	UpdatedBy      string         `json:"updatedBy"`
	CreatedAt      NormalizedTime `json:"createdAt"`
	UpdatedAt      NormalizedTime `json:"updatedAt"`
}

// Tag 表示与市场或事件关联的标签
type Tag struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Slug        string         `json:"slug"`
	ForceShow   bool           `json:"forceShow"`
	PublishedAt NormalizedTime `json:"publishedAt"`
	CreatedBy   int            `json:"createdBy"`
	UpdatedBy   int            `json:"updatedBy"`
	CreatedAt   NormalizedTime `json:"createdAt"`
	UpdatedAt   NormalizedTime `json:"updatedAt"`
	ForceHide   bool           `json:"forceHide"`
	IsCarousel  bool           `json:"isCarousel"`
}

// Series 表示一系列事件
type Series struct {
	ID                string         `json:"id"`
	Ticker            string         `json:"ticker"`
	Slug              string         `json:"slug"`
	Title             string         `json:"title"`
	Subtitle          string         `json:"subtitle"`
	SeriesType        string         `json:"seriesType"`
	Recurrence        string         `json:"recurrence"`
	Description       string         `json:"description"`
	Image             string         `json:"image"`
	Icon              string         `json:"icon"`
	Layout            string         `json:"layout"`
	Active            bool           `json:"active"`
	Closed            bool           `json:"closed"`
	Archived          bool           `json:"archived"`
	New               bool           `json:"new"`
	Featured          bool           `json:"featured"`
	Restricted        bool           `json:"restricted"`
	IsTemplate        bool           `json:"isTemplate"`
	TemplateVariables bool           `json:"templateVariables"`
	PublishedAt       NormalizedTime `json:"publishedAt"`
	CreatedBy         string         `json:"createdBy"`
	UpdatedBy         string         `json:"updatedBy"`
	CreatedAt         NormalizedTime `json:"createdAt"`
	UpdatedAt         NormalizedTime `json:"updatedAt"`
	CommentsEnabled   bool           `json:"commentsEnabled"`
	Competitive       string         `json:"competitive"`
	Volume24hr        float64        `json:"volume24hr"`
	Volume            float64        `json:"volume"`
	Liquidity         float64        `json:"liquidity"`
	StartDate         NormalizedTime `json:"startDate"`
	PythTokenID       string         `json:"pythTokenID"`
	CGAssetName       string         `json:"cgAssetName"`
	Score             float64        `json:"score"`
	Events            []Event        `json:"events,omitempty"`
	Collections       []Collection   `json:"collections,omitempty"`
	Categories        []Category     `json:"categories,omitempty"`
	Tags              []Tag          `json:"tags,omitempty"`
	CommentCount      int            `json:"commentCount"`
	Chats             []Chat         `json:"chats,omitempty"`
}

// Collection 表示事件的集合
type Collection struct {
	ID                   string          `json:"id"`
	Ticker               string          `json:"ticker"`
	Slug                 string          `json:"slug"`
	Title                string          `json:"title"`
	Subtitle             string          `json:"subtitle"`
	CollectionType       string          `json:"collectionType"`
	Description          string          `json:"description"`
	Tags                 string          `json:"tags"`
	Image                string          `json:"image"`
	Icon                 string          `json:"icon"`
	HeaderImage          string          `json:"headerImage"`
	Layout               string          `json:"layout"`
	Active               bool            `json:"active"`
	Closed               bool            `json:"closed"`
	Archived             bool            `json:"archived"`
	New                  bool            `json:"new"`
	Featured             bool            `json:"featured"`
	Restricted           bool            `json:"restricted"`
	IsTemplate           bool            `json:"isTemplate"`
	TemplateVariables    string          `json:"templateVariables"`
	PublishedAt          NormalizedTime  `json:"publishedAt"`
	CreatedBy            string          `json:"createdBy"`
	UpdatedBy            string          `json:"updatedBy"`
	CreatedAt            NormalizedTime  `json:"createdAt"`
	UpdatedAt            NormalizedTime  `json:"updatedAt"`
	CommentsEnabled      bool            `json:"commentsEnabled"`
	ImageOptimized       *ImageOptimized `json:"imageOptimized,omitempty"`
	IconOptimized        *ImageOptimized `json:"iconOptimized,omitempty"`
	HeaderImageOptimized *ImageOptimized `json:"headerImageOptimized,omitempty"`
}

// EventCreator 表示事件的创建者
type EventCreator struct {
	ID            string         `json:"id"`
	CreatorName   string         `json:"creatorName"`
	CreatorHandle string         `json:"creatorHandle"`
	CreatorURL    string         `json:"creatorUrl"`
	CreatorImage  string         `json:"creatorImage"`
	CreatedAt     NormalizedTime `json:"createdAt"`
	UpdatedAt     NormalizedTime `json:"updatedAt"`
}

// Chat 表示与事件关联的聊天频道
type Chat struct {
	ID           string         `json:"id"`
	ChannelID    string         `json:"channelId"`
	ChannelName  string         `json:"channelName"`
	ChannelImage string         `json:"channelImage"`
	Live         bool           `json:"live"`
	StartTime    NormalizedTime `json:"startTime"`
	EndTime      NormalizedTime `json:"endTime"`
}

// Template 表示事件模板
type Template struct {
	ID               string `json:"id"`
	EventTitle       string `json:"eventTitle"`
	EventSlug        string `json:"eventSlug"`
	EventImage       string `json:"eventImage"`
	MarketTitle      string `json:"marketTitle"`
	Description      string `json:"description"`
	ResolutionSource string `json:"resolutionSource"`
	NegRisk          bool   `json:"negRisk"`
	SortBy           string `json:"sortBy"`
	ShowMarketImages bool   `json:"showMarketImages"`
	SeriesSlug       string `json:"seriesSlug"`
	Outcomes         string `json:"outcomes"`
}

// Team 表示运动队
type Team struct {
	ID           int            `json:"id"`
	Name         string         `json:"name"`
	League       string         `json:"league"`
	Record       string         `json:"record"`
	Logo         string         `json:"logo"`
	Abbreviation string         `json:"abbreviation"`
	Alias        string         `json:"alias"`
	CreatedAt    NormalizedTime `json:"createdAt"`
	UpdatedAt    NormalizedTime `json:"updatedAt"`
}

// SportMetadata 表示体育运动的元数据信息
type SportMetadata struct {
	Sport      string `json:"sport"`
	Image      string `json:"image"`
	Resolution string `json:"resolution"`
	Ordering   string `json:"ordering"`
	Tags       string `json:"tags"`
	Series     string `json:"series"`
}

// TagRelationship 表示两个标签之间的关系
type TagRelationship struct {
	ID           string `json:"id"`
	TagID        int    `json:"tagID"`
	RelatedTagID int    `json:"relatedTagID"`
	Rank         int    `json:"rank"`
}

// TagStatus 表示相关标签的状态过滤器
type TagStatus string

const (
	TagStatusActive TagStatus = "active"
	TagStatusClosed TagStatus = "closed"
	TagStatusAll    TagStatus = "all"
)

// ErrorResponse 表示来自 API 的标准错误响应
type ErrorResponse struct {
	Error string `json:"error"`
}

// SortBy 表示持仓的排序字段
type SortBy string

const (
	SortByCurrent    SortBy = "CURRENT"
	SortByInitial    SortBy = "INITIAL"
	SortByTokens     SortBy = "TOKENS"
	SortByCashPnl    SortBy = "CASHPNL"
	SortByPercentPnl SortBy = "PERCENTPNL"
	SortByTitle      SortBy = "TITLE"
	SortByResolving  SortBy = "RESOLVING"
	SortByPrice      SortBy = "PRICE"
	SortByAvgPrice   SortBy = "AVGPRICE"
)

// SortDirection 表示排序方向
type SortDirection string

const (
	SortDirectionAsc  SortDirection = "ASC"
	SortDirectionDesc SortDirection = "DESC"
)

// Position 表示用户在市场中的持仓
type Position struct {
	ProxyWallet        string          `json:"proxyWallet"`
	Asset              string          `json:"asset"`
	ConditionId        string          `json:"conditionId"`
	Size               decimal.Decimal `json:"size"`
	AvgPrice           decimal.Decimal `json:"avgPrice"`
	InitialValue       decimal.Decimal `json:"initialValue"`
	CurrentValue       decimal.Decimal `json:"currentValue"`
	CashPnl            decimal.Decimal `json:"cashPnl"`
	PercentPnl         decimal.Decimal `json:"percentPnl"`
	TotalBought        decimal.Decimal `json:"totalBought"`
	RealizedPnl        decimal.Decimal `json:"realizedPnl"`
	PercentRealizedPnl decimal.Decimal `json:"percentRealizedPnl"`
	CurPrice           decimal.Decimal `json:"curPrice"`
	Redeemable         bool            `json:"redeemable"`
	Mergeable          bool            `json:"mergeable"`
	Title              string          `json:"title"`
	Slug               string          `json:"slug"`
	Icon               string          `json:"icon"`
	EventSlug          string          `json:"eventSlug"`
	Outcome            string          `json:"outcome"`
	OutcomeIndex       int             `json:"outcomeIndex"`
	OppositeOutcome    string          `json:"oppositeOutcome"`
	OppositeAsset      string          `json:"oppositeAsset"`
	EndDate            string          `json:"endDate"`
	NegativeRisk       bool            `json:"negativeRisk"`
}

// GetPositionsParams 表示获取用户持仓的参数
type GetPositionsParams struct {
	User          string           // 必需: 用户地址。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
	Market        []string         // 可选: 条件 ID 的逗号分隔列表。0x 前缀的 64 位十六进制字符串。与 EventId 互斥。
	EventId       []int            // 可选: 事件 ID 的逗号分隔列表。与 Market 互斥。
	SizeThreshold *decimal.Decimal // 可选: 默认为 1，所需范围: x >= 0
	Redeemable    *bool            // 可选: 默认为 false
	Mergeable     *bool            // 可选: 默认为 false
	Limit         int              // 可选: 默认为 100，所需范围: 0 <= x <= 500。0 表示未设置。
	Offset        int              // 可选: 默认为 0，所需范围: 0 <= x <= 10000。0 表示未设置。
	SortBy        SortBy           // 可选: 默认为 TOKENS。可用选项: CURRENT, INITIAL, TOKENS, CASHPNL, PERCENTPNL, TITLE, RESOLVING, PRICE, AVGPRICE
	SortDirection SortDirection    // 可选: 默认为 DESC。可用选项: ASC, DESC
	Title         string           // 可选: 最大长度: 100
}

// FilterType 表示交易的过滤器类型
type FilterType string

const (
	FilterTypeCash   FilterType = "CASH"
	FilterTypeTokens FilterType = "TOKENS"
)

// Trade 表示交易记录

// Trade 表示用户在条件代币市场上的交易。
type Trade struct {
	Asset                 string          `json:"asset"`           // 被交易的条件代币的 ERC1155 代币 ID
	Bio                   string          `json:"bio"`             // 交易用户的个人简介
	ConditionID           string          `json:"conditionId"`     // 市场的 ID，也是 CTF 条件 ID
	EventSlug             string          `json:"eventSlug"`       // 事件的 slug
	Icon                  string          `json:"icon"`            // 市场图标图像的 URL
	Name                  string          `json:"name"`            // 交易用户的名称
	Outcome               string          `json:"outcome"`         // 市场的人类可读结果
	OutcomeIndex          int             `json:"outcomeIndex"`    // 结果的索引
	Price                 decimal.Decimal `json:"price"`           // 交易价格
	ProfileImage          string          `json:"profileImage"`    // 用户个人资料图像的 URL
	ProxyWallet           string          `json:"proxyWallet"`     // 用户代理钱包的地址
	Pseudonym             string          `json:"pseudonym"`       // 用户的假名
	Side                  Side            `json:"side"`            // 交易方向 (BUY/SELL)
	Size                  decimal.Decimal `json:"size"`            // 交易规模
	Slug                  string          `json:"slug"`            // 市场的 slug
	Timestamp             int64           `json:"timestamp"`       // 交易的时间戳 (毫秒)
	Title                 string          `json:"title"`           // 事件的标题
	TransactionHash       string          `json:"transactionHash"` // 交易哈希
	Time                  time.Time       `json:"-"`               // 从时间戳解析的时间
	Status                TradeStatus     `json:"status"`          // 匹配状态: 例如 MINED
	ProfileImageOptimized string          `json:"profileImageOptimized"`
}

// GetTradesParams 表示获取交易的参数
type GetTradesParams struct {
	Limit        int              // 可选: 默认为 100，所需范围: 0 <= x <= 10000。0 表示未设置。
	Offset       int              // 可选: 默认为 0，所需范围: 0 <= x <= 10000。0 表示未设置。
	TakerOnly    *bool            // 可选: 默认为 true
	FilterType   FilterType       // 可选: 必须与 FilterAmount 一起提供。可用选项: CASH, TOKENS
	FilterAmount *decimal.Decimal // 可选: 必须与 FilterType 一起提供。所需范围: x >= 0
	Market       []string         // 可选: 条件 ID 的逗号分隔列表 (0x 前缀的 64 位十六进制字符串)。与 EventId 互斥。
	EventId      []int            // 可选: 事件 ID 的逗号分隔列表。与 Market 互斥。
	User         string           // 可选: 用户个人资料地址 (0x 前缀，40 个十六进制字符)。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
	Side         Side             // 可选: 可用选项: BUY, SELL
}

// TradedMarketsCount 表示用户交易的市场总数
type TradedMarketsCount struct {
	User   string `json:"user"`   // 用户个人资料地址 (0x 前缀，40 个十六进制字符)。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
	Traded int    `json:"traded"` // 交易的市场总数
}

// GetTradedMarketsCountParams 表示获取交易市场计数的参数
type GetTradedMarketsCountParams struct {
	User string // 必需: 用户个人资料地址 (0x 前缀，40 个十六进制字符)。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
}

// ActivityType 表示活动类型
type ActivityType string

const (
	ActivityTypeTrade      ActivityType = "TRADE"
	ActivityTypeSplit      ActivityType = "SPLIT"
	ActivityTypeMerge      ActivityType = "MERGE"
	ActivityTypeRedeem     ActivityType = "REDEEM"
	ActivityTypeReward     ActivityType = "REWARD"
	ActivityTypeConversion ActivityType = "CONVERSION"
)

// ActivitySortBy 表示活动的排序字段
type ActivitySortBy string

const (
	ActivitySortByTimestamp ActivitySortBy = "TIMESTAMP"
	ActivitySortByTokens    ActivitySortBy = "TOKENS"
	ActivitySortByCash      ActivitySortBy = "CASH"
)

// Activity 表示用户的链上活动
type Activity struct {
	ProxyWallet           string          `json:"proxyWallet"` // 用户个人资料地址 (0x 前缀，40 个十六进制字符)。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
	Timestamp             int64           `json:"timestamp"`
	ConditionId           string          `json:"conditionId"` // 0x 前缀的 64 位十六进制字符串。示例: "0xdd22472e552920b8438158ea7238bfadfa4f736aa4cee91a6b86c39ead110917"
	Type                  ActivityType    `json:"type"`        // 可用选项: TRADE, SPLIT, MERGE, REDEEM, REWARD, CONVERSION
	Size                  decimal.Decimal `json:"size"`
	UsdcSize              decimal.Decimal `json:"usdcSize"`
	TransactionHash       string          `json:"transactionHash"`
	Price                 decimal.Decimal `json:"price"`
	Asset                 string          `json:"asset"`
	Side                  Side            `json:"side"` // 可用选项: BUY, SELL
	OutcomeIndex          int             `json:"outcomeIndex"`
	Title                 string          `json:"title"`
	Slug                  string          `json:"slug"`
	Icon                  string          `json:"icon"`
	EventSlug             string          `json:"eventSlug"`
	Outcome               string          `json:"outcome"`
	Name                  string          `json:"name"`
	Pseudonym             string          `json:"pseudonym"`
	Bio                   string          `json:"bio"`
	ProfileImage          string          `json:"profileImage"`
	ProfileImageOptimized string          `json:"profileImageOptimized"`
}

// GetActivityParams 表示获取用户活动的参数
type GetActivityParams struct {
	Limit         int            // 可选: 默认为 100，所需范围: 0 <= x <= 500。0 表示未设置。
	Offset        int            // 可选: 默认为 0，所需范围: 0 <= x <= 10000。0 表示未设置。
	User          string         // 必需: 用户个人资料地址 (0x 前缀，40 个十六进制字符)。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
	Market        []string       // 可选: 条件 ID 的逗号分隔列表 (0x 前缀的 64 位十六进制字符串)。与 EventId 互斥。
	EventId       []int          // 可选: 事件 ID 的逗号分隔列表。与 Market 互斥。
	Type          []ActivityType // 可选: 活动类型过滤器
	Start         int64          // 可选: 开始时间戳，所需范围: x >= 0。0 表示未设置。
	End           int64          // 可选: 结束时间戳，所需范围: x >= 0。0 表示未设置。
	SortBy        ActivitySortBy // 可选: 默认为 TIMESTAMP。可用选项: TIMESTAMP, TOKENS, CASH
	SortDirection SortDirection  // 可选: 默认为 DESC。可用选项: ASC, DESC
	Side          Side           // 可选: 可用选项: BUY, SELL
}

// Holder 表示市场代币的持有者
type Holder struct {
	ProxyWallet           string          `json:"proxyWallet"`           // 用户个人资料地址 (0x 前缀，40 个十六进制字符)。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
	Bio                   string          `json:"bio"`                   // 用户简介
	Asset                 string          `json:"asset"`                 // 资产地址
	Pseudonym             string          `json:"pseudonym"`             // 用户假名
	Amount                decimal.Decimal `json:"amount"`                // 持有数量
	DisplayUsernamePublic bool            `json:"displayUsernamePublic"` // 用户名是否公开
	OutcomeIndex          int             `json:"outcomeIndex"`          // 结果索引
	Name                  string          `json:"name"`                  // 用户名称
	ProfileImage          string          `json:"profileImage"`          // 个人资料图像 URL
	ProfileImageOptimized string          `json:"profileImageOptimized"` // 优化的个人资料图像 URL
}

// MarketHolders 表示特定市场代币的持有者
type MarketHolders struct {
	Token   string   `json:"token"`   // 代币地址
	Holders []Holder `json:"holders"` // 持有者列表
}

// GetHoldersParams 表示获取顶级持有者的参数
type GetHoldersParams struct {
	Limit      int      // 可选: 默认为 100，所需范围: 0 <= x <= 500。0 表示未设置。
	Market     []string // 必需: 条件 ID 的逗号分隔列表 (0x 前缀的 64 位十六进制字符串)
	MinBalance int      // 可选: 默认为 1，所需范围: 0 <= x <= 999999。0 表示未设置。
}

// UserValue 表示用户持仓的总价值
type UserValue struct {
	User  string          `json:"user"`  // 用户个人资料地址 (0x 前缀，40 个十六进制字符)。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
	Value decimal.Decimal `json:"value"` // 总价值
}

// GetValueParams 表示获取用户总持仓价值的参数
type GetValueParams struct {
	User   string   // 必需: 用户个人资料地址 (0x 前缀，40 个十六进制字符)。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
	Market []string // 可选: 条件 ID 的列表 (0x 前缀的 64 位十六进制字符串)
}

// ClosedPositionSortBy 表示已平仓持仓的排序字段
type ClosedPositionSortBy string

const (
	ClosedPositionSortByRealizedPnl ClosedPositionSortBy = "REALIZEDPNL"
	ClosedPositionSortByTimestamp   ClosedPositionSortBy = "TIMESTAMP"
	ClosedPositionSortByTitle       ClosedPositionSortBy = "TITLE"
	ClosedPositionSortByPrice       ClosedPositionSortBy = "PRICE"
	ClosedPositionSortByAvgPrice    ClosedPositionSortBy = "AVGPRICE"
)

// ClosedPosition 表示用户的已平仓持仓
type ClosedPosition struct {
	ProxyWallet     string          `json:"proxyWallet"` // 用户个人资料地址 (0x 前缀，40 个十六进制字符)。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
	Asset           string          `json:"asset"`
	ConditionId     string          `json:"conditionId"` // 0x 前缀的 64 位十六进制字符串。示例: "0xdd22472e552920b8438158ea7238bfadfa4f736aa4cee91a6b86c39ead110917"
	AvgPrice        decimal.Decimal `json:"avgPrice"`
	TotalBought     decimal.Decimal `json:"totalBought"`
	RealizedPnl     decimal.Decimal `json:"realizedPnl"`
	CurPrice        decimal.Decimal `json:"curPrice"`
	Title           string          `json:"title"`
	Slug            string          `json:"slug"`
	Icon            string          `json:"icon"`
	EventSlug       string          `json:"eventSlug"`
	Outcome         string          `json:"outcome"`
	OutcomeIndex    int             `json:"outcomeIndex"`
	OppositeOutcome string          `json:"oppositeOutcome"`
	OppositeAsset   string          `json:"oppositeAsset"`
	EndDate         string          `json:"endDate"`
}

// GetClosedPositionsParams 表示获取已平仓持仓的参数
type GetClosedPositionsParams struct {
	User          string               // 必需: 用户的地址。示例: "0x56687bf447db6ffa42ffe2204a05edaa20f55839"
	Market        []string             // 可选: 市场的 conditionId (0x 前缀的 64 位十六进制字符串)。不能与 EventId 一起使用。
	Title         string               // 可选: 按市场标题过滤。最大长度: 100
	EventId       []int                // 可选: 事件 ID。返回这些事件 ID 的所有市场的持仓。不能与 Market 一起使用。
	Limit         int                  // 可选: 默认为 50，所需范围: 0 <= x <= 500。0 表示未设置。
	Offset        int                  // 可选: 默认为 0，所需范围: 0 <= x <= 10000。0 表示未设置。
	SortBy        ClosedPositionSortBy // 可选: 默认为 REALIZEDPNL。可用选项: REALIZEDPNL, TITLE, PRICE, AVGPRICE
	SortDirection SortDirection        // 可选: 默认为 DESC。可用选项: ASC, DESC
}

// OpenInterest 表示市场的未平仓合约
type OpenInterest struct {
	Market string          `json:"market"` // 0x 前缀的 64 位十六进制字符串。示例: "0xdd22472e552920b8438158ea7238bfadfa4f736aa4cee91a6b86c39ead110917"
	Value  decimal.Decimal `json:"value"`  // 未平仓合约价值
}

// GetOpenInterestParams 表示获取未平仓合约的参数
type GetOpenInterestParams struct {
	Market []string // 可选: 市场条件 ID 的列表 (0x 前缀的 64 位十六进制字符串)
}

// LiveVolumeMarket 表示特定市场的交易量
type LiveVolumeMarket struct {
	Market string          `json:"market"` // 0x 前缀的 64 位十六进制字符串。示例: "0xdd22472e552920b8438158ea7238bfadfa4f736aa4cee91a6b86c39ead110917"
	Value  decimal.Decimal `json:"value"`  // 交易量价值
}

// LiveVolume 表示事件的实时交易量
type LiveVolume struct {
	Total   decimal.Decimal    `json:"total"`   // 总交易量
	Markets []LiveVolumeMarket `json:"markets"` // 市场交易量列表
}

// GetLiveVolumeParams 表示获取实时交易量的参数
type GetLiveVolumeParams struct {
	Id int // 必需: 事件 ID，所需范围: x >= 1
}


// Token 表示市场的代币信息
type Token struct {
TokenID string
Outcome string
Winner  bool
}

// GetTokens 从市场获取代币列表 (从 ClobTokenIDs 和 Outcomes 构建)
func (m *Market) GetTokens() []Token {
var tokens []Token
ids := []string(m.ClobTokenIDs)
outcomes := []string(m.Outcomes)

for i := 0; i < len(ids) && i < len(outcomes); i++ {
tokens = append(tokens, Token{
TokenID: ids[i],
Outcome: outcomes[i],
})
}
return tokens
}
