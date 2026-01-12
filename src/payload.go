package src

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// NOTE: 这份文件用于服务器发送的 payloads。
// 更多信息请参阅: https://github.com/Polymarket/real-time-data-client

// UnmarshalJSON 实现 Trade 的自定义 JSON 反序列化
func (t *Trade) UnmarshalJSON(data []byte) error {
	type Alias Trade
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 将时间戳解析为 time.Time (毫秒)
	if t.Timestamp != 0 {
		t.Time = time.UnixMilli(t.Timestamp)
	}

	return nil
}

type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

type TradeStatus string

const (
	TradeStatusMatched                = "MATCHED"
	TradeStatusMined                  = "MINED"
	TradeStatusConfirmed              = "CONFIRMED"
	TradeStatusCanceledMarketResolved = "CANCELED_MARKET_RESOLVED"
)

// Comment 代表事件或系列上的评论。
type Comment struct {
	ID               string `json:"id"`               // 评论的唯一标识符
	Body             string `json:"body"`             // 评论内容
	ParentEntityType string `json:"parentEntityType"` // 父实体的类型 (Event 或 Series)
	ParentEntityID   int64  `json:"parentEntityID"`   // 父实体的 ID (从 float64 改为 int64)
	ParentCommentID  string `json:"parentCommentID"`  // 父评论的 ID
	UserAddress      string `json:"userAddress"`      // 用户的地址
	ReplyAddress     string `json:"replyAddress"`     // 回复用户的地址
	CreatedAt        string `json:"createdAt"`        // 创建时间戳
	UpdatedAt        string `json:"updatedAt"`        // 最后更新时间戳
}

// Reaction 代表对评论的反应。
type Reaction struct {
	ID           string `json:"id"`           // 反应的唯一标识符
	CommentID    int64  `json:"commentID"`    // 评论的 ID (从 float64 改为 int64)
	ReactionType string `json:"reactionType"` // 反应类型
	Icon         string `json:"icon"`         // 代表反应的图标
	UserAddress  string `json:"userAddress"`  // 用户的地址
	CreatedAt    string `json:"createdAt"`    // 创建时间戳
}

// RFQRequest 代表交易条件代币的请求。
type RFQRequest struct {
	RequestID    string          `json:"requestId"`    // 请求的唯一标识符
	ProxyAddress string          `json:"proxyAddress"` // 用户代理地址
	Market       string          `json:"market"`       // 市场 ID，也是 CTF 条件 ID
	Token        string          `json:"token"`        // 正在交易的条件代币的 ERC1155 代币 ID
	Complement   string          `json:"complement"`   // 正在交易的条件代币的互补 ERC1155 代币 ID
	State        string          `json:"state"`        // 请求的当前状态
	Side         Side            `json:"side"`         // 指示买方或卖方
	SizeIn       decimal.Decimal `json:"sizeIn"`       // 请求的输入数量
	SizeOut      decimal.Decimal `json:"sizeOut"`      // 请求的输出数量
	Price        decimal.Decimal `json:"price"`        // 从输入/输出数量计算的价格
	Expiry       int64           `json:"expiry"`       // 过期时间戳 (UNIX 格式, 秒)
	ExpiryTime   time.Time       `json:"-"`            // 解析后的过期时间
}

// UnmarshalJSON 实现 RFQRequest 的自定义 JSON 反序列化
func (r *RFQRequest) UnmarshalJSON(data []byte) error {
	type Alias RFQRequest
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 将过期时间解析为 time.Time (秒)
	if r.Expiry != 0 {
		r.ExpiryTime = time.Unix(r.Expiry, 0)
	}

	return nil
}

// RFQQuote 代表对交易请求的响应。
type RFQQuote struct {
	QuoteID      string          `json:"quoteId"`      // 报价的唯一标识符
	RequestID    string          `json:"requestId"`    // 关联的请求标识符
	ProxyAddress string          `json:"proxyAddress"` // 用户代理地址
	Token        string          `json:"token"`        // 正在交易的条件代币的 ERC1155 代币 ID
	State        string          `json:"state"`        // 报价的当前状态
	Side         Side            `json:"side"`         // 指示买方或卖方
	SizeIn       decimal.Decimal `json:"sizeIn"`       // 报价的输入数量
	SizeOut      decimal.Decimal `json:"sizeOut"`      // 报价的输出数量
	Condition    string          `json:"condition"`    // 市场 ID，也是 CTF 条件 ID
	Complement   string          `json:"complement"`   // 正在交易的条件代币的互补 ERC1155 代币 ID
	Expiry       int64           `json:"expiry"`       // 过期时间戳 (UNIX 格式, 秒)
	ExpiryTime   time.Time       `json:"-"`            // 解析后的过期时间
}

// UnmarshalJSON 实现 RFQQuote 的自定义 JSON 反序列化
func (q *RFQQuote) UnmarshalJSON(data []byte) error {
	type Alias RFQQuote
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(q),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 将过期时间解析为 time.Time (秒)
	if q.Expiry != 0 {
		q.ExpiryTime = time.Unix(q.Expiry, 0)
	}

	return nil
}

// CryptoPrice 代表加密货币价格更新。
type CryptoPrice struct {
	Symbol    string          `json:"symbol"`    // 资产代码
	Timestamp int64           `json:"timestamp"` // 更新的时间戳（毫秒）
	Time      time.Time       `json:"-"`         // 从时间戳解析的时间
	Value     decimal.Decimal `json:"value"`     // 更新时的值
}

// UnmarshalJSON 实现 CryptoPrice 的自定义 JSON 反序列化
func (c *CryptoPrice) UnmarshalJSON(data []byte) error {
	type Alias CryptoPrice
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 将时间戳解析为 time.Time (毫秒)
	if c.Timestamp != 0 {
		c.Time = time.UnixMilli(c.Timestamp)
	}

	return nil
}

// EquityPrice 代表股票价格更新。
type EquityPrice struct {
	Symbol    string          `json:"symbol"`    // 资产代码
	Timestamp int64           `json:"timestamp"` // 更新的时间戳（毫秒）
	Time      time.Time       `json:"-"`         // 从时间戳解析的时间
	Value     decimal.Decimal `json:"value"`     // 更新时的值
}

// UnmarshalJSON 实现 EquityPrice 的自定义 JSON 反序列化
func (e *EquityPrice) UnmarshalJSON(data []byte) error {
	type Alias EquityPrice
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 将时间戳解析为 time.Time (毫秒)
	if e.Timestamp != 0 {
		e.Time = time.UnixMilli(e.Timestamp)
	}

	return nil
}

// CLOBOrder 代表 CLOB 用户订单。
type CLOBOrder struct {
	AssetID      string          `json:"asset_id"`      // 订单的条件代币 ERC1155 代币 ID
	CreatedAt    string          `json:"created_at"`    // 订单创建 UNIX 时间戳
	Expiration   string          `json:"expiration"`    // 订单过期 UNIX 时间戳
	ID           string          `json:"id"`            // 唯一订单哈希标识符
	MakerAddress string          `json:"maker_address"` // 挂单者地址 (资助者)
	Market       string          `json:"market"`        // 条件 ID 或市场标识符
	OrderType    string          `json:"order_type"`    // 订单类型: GTC, GTD, FOK, FAK
	OriginalSize decimal.Decimal `json:"original_size"` // 下单时的原始数量
	Outcome      string          `json:"outcome"`       // 订单结果: YES / NO
	Owner        string          `json:"owner"`         // 订单所有者的 UUID
	Price        decimal.Decimal `json:"price"`         // 订单价格 (例如, 小数点格式如 0.5)
	Side         Side            `json:"side"`          // 交易方向: BUY 或 SELL
	SizeMatched  decimal.Decimal `json:"size_matched"`  // 已撮合的订单数量
	Status       string          `json:"status"`        // 订单状态 (例如, MATCHED)
	Type         string          `json:"type"`          // 更新类型: PLACEMENT, CANCELLATION, FILL, 等.
}

// CLOBTrade 代表 CLOB 用户交易。
type CLOBTrade struct {
	AssetID         string           `json:"asset_id"`         // 交易中涉及的条件代币的 ERC1155 代币 ID
	FeeRateBps      decimal.Decimal  `json:"fee_rate_bps"`     // 费率（基点 bps）
	ID              string           `json:"id"`               // 撮合记录的唯一标识符
	LastUpdate      string           `json:"last_update"`      // 最后更新时间戳 (UNIX)
	MakerAddress    string           `json:"maker_address"`    // 挂单者地址
	MakerOrders     []CLOBMakerOrder `json:"maker_orders"`     // 挂单列表
	Market          string           `json:"market"`           // 条件 ID 或市场标识符
	MatchTime       string           `json:"match_time"`       // 撮合执行时间戳 (UNIX)
	Outcome         string           `json:"outcome"`          // 市场结果: YES / NO
	Owner           string           `json:"owner"`            // 吃单者（被撮合订单的所有者）的 UUID
	Price           decimal.Decimal  `json:"price"`            // 成交价格 (小数格式，例如 0.5)
	Side            Side             `json:"side"`             // 交易的吃单方: BUY 或 SELL
	Size            decimal.Decimal  `json:"size"`             // 总成交数量
	Status          TradeStatus      `json:"status"`           // 撮合状态: 例如, MINED
	TakerOrderID    string           `json:"taker_order_id"`   // 吃单者的订单 ID
	TransactionHash string           `json:"transaction_hash"` // 结算撮合的交易哈希
}

// CLOBMakerOrder 代表 CLOB 交易中的挂单。
type CLOBMakerOrder struct {
	AssetID       string          `json:"asset_id"`       // 挂单的条件代币 ERC1155 代币 ID
	FeeRateBps    decimal.Decimal `json:"fee_rate_bps"`   // 挂单者费率（基点）
	MakerAddress  string          `json:"maker_address"`  // 挂单者地址
	MatchedAmount decimal.Decimal `json:"matched_amount"` // 从挂单中撮合的数量
	OrderID       string          `json:"order_id"`       // 挂单的订单 ID
	Outcome       string          `json:"outcome"`        // 挂单针对的结果 (YES / NO)
	Owner         string          `json:"owner"`          // 挂单者的 UUID
	Price         decimal.Decimal `json:"price"`          // 订单价格
	Side          Side            `json:"side"`           // 挂单方: BUY 或 SELL
}

// PriceChanges 代表 CLOB 市场价格变动。
type PriceChanges struct {
	Market      string        `json:"m"`  // 条件 ID
	PriceChange []PriceChange `json:"pc"` // 按订单簿的价格变动
	Timestamp   string        `json:"t"`  // 自纪元以来的时间戳（毫秒）(UNIX 时间 * 1000)
}

// PriceChange 代表 CLOB 市场中的单次价格变动。
type PriceChange struct {
	AssetID string          `json:"a"`  // 资产标识符
	Hash    string          `json:"h"`  // 订单簿快照的唯一哈希 ID
	Price   decimal.Decimal `json:"p"`  // 报价 (例如, 0.5)
	Side    Side            `json:"si"` // 报价方: BUY 或 SELL
	Size    decimal.Decimal `json:"s"`  // 报价处可用的数量或体积 (例如, 0, 100)
	BestAsk decimal.Decimal `json:"ba"` // 最佳卖价
	BestBid decimal.Decimal `json:"bb"` // 最佳买价
}

// AggOrderbook 代表 CLOB 市场聚合订单簿。
type AggOrderbook struct {
	AssetID      string           `json:"asset_id"`       // 资产 Id 标识符
	Asks         []OrderbookLevel `json:"asks"`           // 聚合卖单列表 (卖方)
	Bids         []OrderbookLevel `json:"bids"`           // 聚合买单列表 (买方)
	Hash         string           `json:"hash"`           // 此订单簿快照的唯一哈希 ID
	Market       string           `json:"market"`         // 市场或条件 ID
	MinOrderSize string           `json:"min_order_size"` // 允许的最小订单数量
	NegRisk      bool             `json:"neg_risk"`       // 是否 NegRisk
	TickSize     string           `json:"tick_size"`      // 最小 tick 大小
	Timestamp    string           `json:"timestamp"`      // 自纪元以来的时间戳（毫秒）(UNIX 时间 * 1000)
}

// OrderbookLevel 代表订单簿中的价格档位。
type OrderbookLevel struct {
	Price decimal.Decimal `json:"price"` // 价格档位
	Size  decimal.Decimal `json:"size"`  // 该价格的数量
}

// LastTradePrice 代表 CLOB 市场最新成交价。
type LastTradePrice struct {
	AssetID    string          `json:"asset_id"`     // 资产 Id 标识符
	FeeRateBps decimal.Decimal `json:"fee_rate_bps"` // 费率（基点 bps）
	Market     string          `json:"market"`       // 市场或条件 ID
	Price      decimal.Decimal `json:"price"`        // 成交价格 (例如, 0.5)
	Side       Side            `json:"side"`         // 订单方: BUY 或 SELL
	Size       decimal.Decimal `json:"size"`         // 成交数量
}

// TickSizeChange 代表 CLOB 市场 tick 大小变动。
type TickSizeChange struct {
	Market      string          `json:"market"`        // 市场或条件 ID
	AssetID     []string        `json:"asset_id"`      // 两个 ERC1155 资产 ID 的数组
	OldTickSize decimal.Decimal `json:"old_tick_size"` // 变更前的 tick 大小
	NewTickSize decimal.Decimal `json:"new_tick_size"` // 变更后的 tick 大小
}

// ClobMarket 代表 CLOB 市场信息。
// 这用于 market_created 和 market_resolved 消息类型。
type ClobMarket struct {
	Market       string          `json:"market"`         // 市场或条件 ID
	AssetIDs     []string        `json:"asset_ids"`      // 与市场关联的两个 ERC1155 资产 ID 标识符数组
	MinOrderSize decimal.Decimal `json:"min_order_size"` // 允许的最小订单数量
	TickSize     decimal.Decimal `json:"tick_size"`      // 最小允许价格增量
	NegRisk      bool            `json:"neg_risk"`       // 指示市场是否为负风险
}

