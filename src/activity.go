package src

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GetActivity 获取用户的链上活动
func (c *Client) GetActivity(ctx context.Context, params *GetActivityParams) ([]Activity, error) {
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

	if len(params.Type) > 0 {
		types := make([]string, len(params.Type))
		for i, t := range params.Type {
			types[i] = string(t)
		}
		queryParams.Set("type", strings.Join(types, ","))
	}

	if params.Start > 0 {
		queryParams.Set("start", strconv.FormatInt(params.Start, 10))
	}

	if params.End > 0 {
		queryParams.Set("end", strconv.FormatInt(params.End, 10))
	}

	if params.SortBy != "" {
		queryParams.Set("sortBy", string(params.SortBy))
	}

	if params.SortDirection != "" {
		queryParams.Set("sortDirection", string(params.SortDirection))
	}

	if params.Side != "" {
		queryParams.Set("side", string(params.Side))
	}

	// 构建 URL
	reqURL := fmt.Sprintf("%s/activity?%s", DataAPIURL, queryParams.Encode())

	// 发送请求
	body, err := c.doRequest(ctx, "GET", reqURL)
	if err != nil {
		return nil, fmt.Errorf("activity 请求失败: %w", err)
	}

	// 解析成功响应
	var activities []Activity
	if err := json.Unmarshal(body, &activities); err != nil {
		return nil, fmt.Errorf("解码 activity 响应失败: %w", err)
	}

	return activities, nil
}

