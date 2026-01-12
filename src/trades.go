package src

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GetTrades 获取用户或市场的交易
func (c *Client) GetTrades(ctx context.Context, params *GetTradesParams) ([]Trade, error) {
	// 验证互斥参数
	if len(params.Market) > 0 && len(params.EventId) > 0 {
		return nil, fmt.Errorf("market 和 eventId 参数互斥")
	}

	// 验证 FilterType 和 FilterAmount 必须同时提供
	if (params.FilterType != "" && params.FilterAmount == nil) || (params.FilterType == "" && params.FilterAmount != nil) {
		return nil, fmt.Errorf("filterType 和 filterAmount 必须同时提供")
	}

	// 构建查询参数
	queryParams := url.Values{}

	// 添加可选参数
	if params.Limit > 0 {
		if params.Limit > 10000 {
			return nil, fmt.Errorf("limit 必须在 0 到 10000 之间")
		}
		queryParams.Set("limit", strconv.Itoa(params.Limit))
	}

	if params.Offset > 0 {
		if params.Offset > 10000 {
			return nil, fmt.Errorf("offset 必须在 0 到 10000 之间")
		}
		queryParams.Set("offset", strconv.Itoa(params.Offset))
	}

	if params.TakerOnly != nil {
		queryParams.Set("takerOnly", strconv.FormatBool(*params.TakerOnly))
	}

	if params.FilterType != "" {
		queryParams.Set("filterType", string(params.FilterType))
		queryParams.Set("filterAmount", params.FilterAmount.String())
	}

	if len(params.Market) > 0 {
		queryParams.Set("market", strings.Join(params.Market, ","))
	}

	if len(params.EventId) > 0 {
		eventIds := make([]string, len(params.EventId))
		for i, id := range params.EventId {
			eventIds[i] = strconv.Itoa(id)
		}
		queryParams.Set("eventId", strings.Join(eventIds, ","))
	}

	if params.User != "" {
		queryParams.Set("user", params.User)
	}

	if params.Side != "" {
		queryParams.Set("side", string(params.Side))
	}

	// 构建 URL
	reqURL := fmt.Sprintf("%s/trades?%s", DataAPIURL, queryParams.Encode())

	// 发送请求
	body, err := c.doRequest(ctx, "GET", reqURL)
	if err != nil {
		return nil, fmt.Errorf("trades 请求失败: %w", err)
	}

	// 解析成功响应
	var trades []Trade
	if err := json.Unmarshal(body, &trades); err != nil {
		return nil, fmt.Errorf("解码 trades 响应失败: %w", err)
	}

	return trades, nil
}

// GetTradedMarketsCount 获取用户交易过的市场总数
func (c *Client) GetTradedMarketsCount(ctx context.Context, params *GetTradedMarketsCountParams) (*TradedMarketsCount, error) {
	if params.User == "" {
		return nil, fmt.Errorf("user 地址是必需的")
	}

	// 构建查询参数
	queryParams := url.Values{}
	queryParams.Set("user", params.User)

	// 构建 URL
	reqURL := fmt.Sprintf("%s/traded?%s", DataAPIURL, queryParams.Encode())

	// 发送请求
	body, err := c.doRequest(ctx, "GET", reqURL)
	if err != nil {
		return nil, fmt.Errorf("traded 请求失败: %w", err)
	}

	// 解析成功响应
	var tradedCount TradedMarketsCount
	if err := json.Unmarshal(body, &tradedCount); err != nil {
		return nil, fmt.Errorf("解码 traded 响应失败: %w", err)
	}

	return &tradedCount, nil
}

