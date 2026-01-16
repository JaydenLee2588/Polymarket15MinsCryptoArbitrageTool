package src

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/shopspring/decimal"
)

// GetFullOrderbook 获取完整的订单簿 (使用 src/payload.go 中定义的结构)
func (c *ClobOrderClient) GetFullOrderbook(ctx context.Context, tokenID string) (*AggOrderbook, error) {
	url := fmt.Sprintf("%s/book?token_id=%s", ClobHost, tokenID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API错误 (%d): %s", resp.StatusCode, string(body))
	}

	var orderbook AggOrderbook
	if err := json.Unmarshal(body, &orderbook); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &orderbook, nil
}

// OrderbookDepthResult 订单簿深度查询结果
type OrderbookDepthResult struct {
	AvailableSize float64 // 指定价格的可用数量
	TotalAskSize  float64 // 总卖单数量（所有价格）
	TotalBidSize  float64 // 总买单数量（所有价格）
}

// GetOrderbookDepth 查询指定Token和价格的订单簿深度
//
// 参数:
//   - ctx: 上下文
//   - tokenID: Token ID
//   - targetPrice: 目标价格
//   - side: "BUY" (查询卖单深度) 或 "SELL" (查询买单深度)
//
// 返回:
//   - OrderbookDepthResult: 深度信息
//   - error: 错误信息
func (c *ClobOrderClient) GetOrderbookDepth(
	ctx context.Context,
	tokenID string,
	targetPrice float64,
	side string,
) (*OrderbookDepthResult, error) {
	orderbook, err := c.GetFullOrderbook(ctx, tokenID)
	if err != nil {
		return nil, err
	}

	result := &OrderbookDepthResult{}
	targetPriceDec := decimal.NewFromFloat(targetPrice)

	// 根据方向查找深度
	if side == "BUY" {
		// 买入时需要查看卖单（asks）
		for _, ask := range orderbook.Asks {
			sizeFloat, _ := ask.Size.Float64()
			result.TotalAskSize += sizeFloat

			// 找到目标价格或更低价格的卖单
			if ask.Price.LessThanOrEqual(targetPriceDec) {
				result.AvailableSize += sizeFloat
			}
		}
	} else {
		// 卖出时需要查看买单（bids）
		for _, bid := range orderbook.Bids {
			sizeFloat, _ := bid.Size.Float64()
			result.TotalBidSize += sizeFloat

			// 找到目标价格或更高价格的买单
			if bid.Price.GreaterThanOrEqual(targetPriceDec) {
				result.AvailableSize += sizeFloat
			}
		}
	}

	return result, nil
}
