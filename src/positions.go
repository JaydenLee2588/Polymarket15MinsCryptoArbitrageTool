package src

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GetPositions 获取用户的当前仓位
func (c *Client) GetPositions(ctx context.Context, params *GetPositionsParams) ([]Position, error) {
	if params.User == "" {
		return nil, fmt.Errorf("user 地址是必需的")
	}

	// 构建查询参数
	queryParams := url.Values{}
	queryParams.Set("user", params.User)

	// 添加可选参数
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

	if params.SizeThreshold != nil {
		queryParams.Set("sizeThreshold", params.SizeThreshold.String())
	}

	if params.Redeemable != nil {
		queryParams.Set("redeemable", strconv.FormatBool(*params.Redeemable))
	}

	if params.Mergeable != nil {
		queryParams.Set("mergeable", strconv.FormatBool(*params.Mergeable))
	}

	if params.Limit > 0 {
		if params.Limit > 500 {
			return nil, fmt.Errorf("limit 必须在 0 到 500 之间")
		}
		queryParams.Set("limit", strconv.Itoa(params.Limit))
	}

	if params.Offset > 0 {
		if params.Offset > 10000 {
			return nil, fmt.Errorf("offset 必须在 0 到 10000 之间")
		}
		queryParams.Set("offset", strconv.Itoa(params.Offset))
	}

	if params.SortBy != "" {
		queryParams.Set("sortBy", string(params.SortBy))
	}

	if params.SortDirection != "" {
		queryParams.Set("sortDirection", string(params.SortDirection))
	}

	if params.Title != "" {
		if len(params.Title) > 100 {
			return nil, fmt.Errorf("标题不能超过 100 个字符")
		}
		queryParams.Set("title", params.Title)
	}

	// 构建 URL
	reqURL := fmt.Sprintf("%s/positions?%s", DataAPIURL, queryParams.Encode())

	// 发送请求
	body, err := c.doRequest(ctx, "GET", reqURL)
	if err != nil {
		return nil, fmt.Errorf("positions 请求失败: %w", err)
	}

	// 解析成功响应
	var positions []Position
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("解码 positions 响应失败: %w", err)
	}

	return positions, nil
}

// GetClosedPositions 获取用户的已平仓仓位
func (c *Client) GetClosedPositions(ctx context.Context, params *GetClosedPositionsParams) ([]ClosedPosition, error) {
	if params.User == "" {
		return nil, fmt.Errorf("user 地址是必需的")
	}

	// 验证互斥参数
	if len(params.Market) > 0 && len(params.EventId) > 0 {
		return nil, fmt.Errorf("market 和 eventId 参数互斥")
	}

	// 构建查询参数
	queryParams := url.Values{}
	queryParams.Set("user", params.User)

	// 添加可选参数
	if len(params.Market) > 0 {
		queryParams.Set("market", strings.Join(params.Market, ","))
	}

	if params.Title != "" {
		if len(params.Title) > 100 {
			return nil, fmt.Errorf("标题不能超过 100 个字符")
		}
		queryParams.Set("title", params.Title)
	}

	if len(params.EventId) > 0 {
		eventIds := make([]string, len(params.EventId))
		for i, id := range params.EventId {
			eventIds[i] = strconv.Itoa(id)
		}
		queryParams.Set("eventId", strings.Join(eventIds, ","))
	}

	if params.Limit > 0 {
		if params.Limit > 500 {
			return nil, fmt.Errorf("limit 必须在 0 到 500 之间")
		}
		queryParams.Set("limit", strconv.Itoa(params.Limit))
	}

	if params.Offset > 0 {
		if params.Offset > 10000 {
			return nil, fmt.Errorf("offset 必须在 0 到 10000 之间")
		}
		queryParams.Set("offset", strconv.Itoa(params.Offset))
	}

	if params.SortBy != "" {
		queryParams.Set("sortBy", string(params.SortBy))
	}

	if params.SortDirection != "" {
		queryParams.Set("sortDirection", string(params.SortDirection))
	}

	// 构建 URL
	reqURL := fmt.Sprintf("%s/closed-positions?%s", DataAPIURL, queryParams.Encode())

	// 发送请求
	body, err := c.doRequest(ctx, "GET", reqURL)
	if err != nil {
		return nil, fmt.Errorf("closed-positions 请求失败: %w", err)
	}

	// 解析成功响应
	var closedPositions []ClosedPosition
	if err := json.Unmarshal(body, &closedPositions); err != nil {
		return nil, fmt.Errorf("解码 closed-positions 响应失败: %w", err)
	}

	return closedPositions, nil
}

// GetPositionsValue 获取用户仓位的总价值
func (c *Client) GetPositionsValue(ctx context.Context, params *GetValueParams) ([]UserValue, error) {
	if params.User == "" {
		return nil, fmt.Errorf("user 地址是必需的")
	}

	// 构建查询参数
	queryParams := url.Values{}
	queryParams.Set("user", params.User)

	// 添加可选参数
	if len(params.Market) > 0 {
		queryParams.Set("market", strings.Join(params.Market, ","))
	}

	// 构建 URL
	reqURL := fmt.Sprintf("%s/value?%s", DataAPIURL, queryParams.Encode())

	// 发送请求
	body, err := c.doRequest(ctx, "GET", reqURL)
	if err != nil {
		return nil, fmt.Errorf("value 请求失败: %w", err)
	}

	// 解析成功响应
	var userValues []UserValue
	if err := json.Unmarshal(body, &userValues); err != nil {
		return nil, fmt.Errorf("解码 value 响应失败: %w", err)
	}

	return userValues, nil
}

