package src

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GetHolders 获取市场的头部持有者
func (c *Client) GetHolders(ctx context.Context, params *GetHoldersParams) ([]MarketHolders, error) {
	if len(params.Market) == 0 {
		return nil, fmt.Errorf("market 是必需的")
	}

	// 构建查询参数
	queryParams := url.Values{}
	queryParams.Set("market", strings.Join(params.Market, ","))

	// 添加可选参数
	if params.Limit > 0 {
		if params.Limit > 500 {
			return nil, fmt.Errorf("limit 必须在 0 到 500 之间")
		}
		queryParams.Set("limit", strconv.Itoa(params.Limit))
	}

	if params.MinBalance > 0 {
		if params.MinBalance > 999999 {
			return nil, fmt.Errorf("minBalance 必须在 0 到 999999 之间")
		}
		queryParams.Set("minBalance", strconv.Itoa(params.MinBalance))
	}

	// 构建 URL
	reqURL := fmt.Sprintf("%s/holders?%s", DataAPIURL, queryParams.Encode())

	// 发送请求
	body, err := c.doRequest(ctx, "GET", reqURL)
	if err != nil {
		return nil, fmt.Errorf("holders 请求失败: %w", err)
	}

	// 解析成功响应
	var holders []MarketHolders
	if err := json.Unmarshal(body, &holders); err != nil {
		return nil, fmt.Errorf("解码 holders 响应失败: %w", err)
	}

	return holders, nil
}

