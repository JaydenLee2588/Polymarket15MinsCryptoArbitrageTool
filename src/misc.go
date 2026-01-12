package src

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// GetOpenInterest 获取市场的未平仓合约
func (c *Client) GetOpenInterest(ctx context.Context, params *GetOpenInterestParams) ([]OpenInterest, error) {
	// 构建查询参数
	queryParams := url.Values{}

	// 添加可选参数
	if len(params.Market) > 0 {
		queryParams.Set("market", strings.Join(params.Market, ","))
	}

	// 构建 URL
	reqURL := fmt.Sprintf("%s/oi?%s", DataAPIURL, queryParams.Encode())

	// 发送请求
	body, err := c.doRequest(ctx, "GET", reqURL)
	if err != nil {
		return nil, fmt.Errorf("open interest 请求失败: %w", err)
	}

	// 解析成功响应
	var openInterest []OpenInterest
	if err := json.Unmarshal(body, &openInterest); err != nil {
		return nil, fmt.Errorf("解码 open interest 响应失败: %w", err)
	}

	return openInterest, nil
}

// GetLiveVolume 获取事件的实时成交量
func (c *Client) GetLiveVolume(ctx context.Context, params *GetLiveVolumeParams) ([]LiveVolume, error) {
	if params.Id < 1 {
		return nil, fmt.Errorf("id 必须 >= 1")
	}

	// 构建查询参数
	queryParams := url.Values{}
	queryParams.Set("id", fmt.Sprintf("%d", params.Id))

	// 构建 URL
	reqURL := fmt.Sprintf("%s/live-volume?%s", DataAPIURL, queryParams.Encode())

	// 发送请求
	body, err := c.doRequest(ctx, "GET", reqURL)
	if err != nil {
		return nil, fmt.Errorf("live-volume 请求失败: %w", err)
	}

	// 解析成功响应
	var liveVolume []LiveVolume
	if err := json.Unmarshal(body, &liveVolume); err != nil {
		return nil, fmt.Errorf("解码 live-volume 响应失败: %w", err)
	}

	return liveVolume, nil
}

