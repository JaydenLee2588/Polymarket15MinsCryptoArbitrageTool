package src

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ActivityFilter 代表活动订阅的过滤配置
type ActivityFilter struct {
	// Market 是要过滤的市场条件 ID (可选)
	Market string `json:"market,omitempty"`
	// AssetID 是要过滤的资产 ID (可选)
	AssetID string `json:"asset_id,omitempty"`
}

// ToJSON 将过滤器转换为 JSON 字符串
// 如果过滤器无效（Market 和 AssetID 都为空）则返回错误
func (f *ActivityFilter) ToJSON() (string, error) {
	if f == nil {
		return "", nil
	}
	if f.Market == "" && f.AssetID == "" {
		return "", fmt.Errorf("ActivityFilter: 必须指定 Market 或 AssetID 之一")
	}
	data, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("ActivityFilter: 转换为 JSON 失败: %w", err)
	}
	return string(data), nil
}

// CommentFilter 代表评论订阅的过滤配置
type CommentFilter struct {
	// EventID 是要过滤评论的事件 ID (可选)
	EventID int64 `json:"event_id,omitempty"`
	// SeriesID 是要过滤评论的系列 ID (可选)
	SeriesID int64 `json:"series_id,omitempty"`
}

// ToJSON 将过滤器转换为 JSON 字符串
// 如果过滤器无效（EventID 和 SeriesID 都为零）则返回错误
func (f *CommentFilter) ToJSON() (string, error) {
	if f == nil {
		return "", nil
	}
	if f.EventID == 0 && f.SeriesID == 0 {
		return "", fmt.Errorf("CommentFilter: 必须指定 EventID 或 SeriesID 之一")
	}
	data, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("CommentFilter: 转换为 JSON 失败: %w", err)
	}
	return string(data), nil
}

// CryptoPriceFilter 代表加密货币价格订阅的过滤配置
type CryptoPriceFilter struct {
	// Symbol 是要订阅的加密货币代码 (例如 "btcusdt", "ethusdt")
	Symbol string `json:"symbol"`
}

// ToJSON 将过滤器转换为 JSON 字符串
// 如果过滤器无效（symbol 为空）则返回错误
func (f *CryptoPriceFilter) ToJSON() (string, error) {
	if f == nil {
		return "", fmt.Errorf("CryptoPriceFilter: 过滤器不能为 nil")
	}
	if f.Symbol == "" {
		return "", fmt.Errorf("CryptoPriceFilter: 必须指定 symbol")
	}
	// 验证 symbol 格式 (应为小写)
	if strings.ToLower(f.Symbol) != f.Symbol {
		return "", fmt.Errorf("CryptoPriceFilter: symbol 必须为小写 (例如 'btcusdt' 而不是 'BTCUSDT')")
	}
	data, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("CryptoPriceFilter: 转换为 JSON 失败: %w", err)
	}
	return string(data), nil
}

// EquityPriceFilter 代表股票价格订阅的过滤配置
type EquityPriceFilter struct {
	// Symbol 是要订阅的股票代码 (例如 "AAPL", "GOOGL")
	Symbol string `json:"symbol"`
}

// ToJSON 将过滤器转换为 JSON 字符串
// 如果过滤器无效（symbol 为空）则返回错误
func (f *EquityPriceFilter) ToJSON() (string, error) {
	if f == nil {
		return "", fmt.Errorf("EquityPriceFilter: 过滤器不能为 nil")
	}
	if f.Symbol == "" {
		return "", fmt.Errorf("EquityPriceFilter: 必须指定 symbol")
	}
	// 验证 symbol 格式 (应为大写)
	if strings.ToUpper(f.Symbol) != f.Symbol {
		return "", fmt.Errorf("EquityPriceFilter: symbol 必须为大写 (例如 'AAPL' 而不是 'aapl')")
	}
	data, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("EquityPriceFilter: 转换为 JSON 失败: %w", err)
	}
	return string(data), nil
}

// CLOBMarketFilter 代表 CLOB 市场订阅的过滤配置
type CLOBMarketFilter struct {
	// TokenIDs 是要订阅的代币 ID 列表
	TokenIDs []string `json:"token_ids"`
}

// ToJSON 将过滤器转换为 JSON 字符串
// 如果过滤器无效（token IDs 为空）则返回错误
func (f *CLOBMarketFilter) ToJSON() (string, error) {
	if f == nil {
		return "", fmt.Errorf("CLOBMarketFilter: 过滤器不能为 nil")
	}
	if len(f.TokenIDs) == 0 {
		return "", fmt.Errorf("CLOBMarketFilter: 必须指定至少一个代币 ID")
	}
	// 验证 token ID 是否为空字符串
	for i, tokenID := range f.TokenIDs {
		if tokenID == "" {
			return "", fmt.Errorf("CLOBMarketFilter: 索引 %d 处的代币 ID 为空", i)
		}
	}
	// API 期望 JSON 数组格式，如 ["100","200"]
	data, err := json.Marshal(f.TokenIDs)
	if err != nil {
		return "", fmt.Errorf("CLOBMarketFilter: 转换为 JSON 失败: %w", err)
	}
	return string(data), nil
}

// NewActivityFilter 创建一个新的活动过滤器
func NewActivityFilter() *ActivityFilter {
	return &ActivityFilter{}
}

// WithMarket 设置市场过滤器
func (f *ActivityFilter) WithMarket(market string) *ActivityFilter {
	f.Market = market
	return f
}

// WithAssetID 设置资产 ID 过滤器
func (f *ActivityFilter) WithAssetID(assetID string) *ActivityFilter {
	f.AssetID = assetID
	return f
}

// NewCommentFilter 创建一个新的评论过滤器
func NewCommentFilter() *CommentFilter {
	return &CommentFilter{}
}

// WithEventID 设置事件 ID 过滤器
func (f *CommentFilter) WithEventID(eventID int64) *CommentFilter {
	f.EventID = eventID
	return f
}

// WithSeriesID 设置系列 ID 过滤器
func (f *CommentFilter) WithSeriesID(seriesID int64) *CommentFilter {
	f.SeriesID = seriesID
	return f
}

// NewCryptoPriceFilter 用给定的符合创建新的加密货币价格过滤器
func NewCryptoPriceFilter(symbol string) *CryptoPriceFilter {
	return &CryptoPriceFilter{Symbol: symbol}
}

// NewEquityPriceFilter 用给定的符合创建新的股票价格过滤器
func NewEquityPriceFilter(symbol string) *EquityPriceFilter {
	return &EquityPriceFilter{Symbol: symbol}
}

// NewCLOBMarketFilter 用给定的代币 ID 创建新的 CLOB 市场过滤器
func NewCLOBMarketFilter(tokenIDs ...string) *CLOBMarketFilter {
	return &CLOBMarketFilter{TokenIDs: tokenIDs}
}

// AddTokenID 向过滤器添加代币 ID
func (f *CLOBMarketFilter) AddTokenID(tokenID string) *CLOBMarketFilter {
	f.TokenIDs = append(f.TokenIDs, tokenID)
	return f
}

