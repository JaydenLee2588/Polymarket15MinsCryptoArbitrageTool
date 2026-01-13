package src

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// OrderStatusResponse 订单状态响应
type OrderStatusResponse struct {
	OrderID      string `json:"id"`
	MarketHash   string `json:"market"`
	AssetID      string `json:"asset_id"`
	OrderType    string `json:"order_type"`
	Status       string `json:"status"` // "LIVE", "MATCHED", "CANCELLED", etc.
	Side         string `json:"side"`   // "BUY" or "SELL"
	Price        string `json:"price"`
	Size         string `json:"size"`
	SizeMatched  string `json:"size_matched"`
	OriginalSize string `json:"original_size"`
	Owner        string `json:"owner"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	ExpiresAt    string `json:"expires_at"`
	MakerAddress string `json:"maker_address"`
}

// GetOrderStatus 查询订单状态
// orderID: 订单ID (从下单响应中获取)
// 返回订单详细状态信息
func (c *ClobOrderClient) GetOrderStatus(ctx context.Context, orderID string) (*OrderStatusResponse, error) {
	if c.credentials == nil {
		return nil, fmt.Errorf("未设置 API 凭证，请先调用 SetCredentials")
	}

	// 构建请求路径
	endpoint := "/data/order/" + orderID

	// 获取当前时间戳
	timestamp := time.Now().Unix()
	nonce := 0

	// 签名请求 (GET 请求，body 为空)
	signature, err := c.signL2Request("GET", endpoint, "", timestamp)
	if err != nil {
		return nil, fmt.Errorf("签名请求失败: %w", err)
	}

	// 创建 HTTP 请求
	fullURL := ClobHost + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加 L2 认证头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("POLY_ADDRESS", c.address.Hex())
	httpReq.Header.Set("POLY_SIGNATURE", signature)
	httpReq.Header.Set("POLY_TIMESTAMP", strconv.FormatInt(timestamp, 10))
	httpReq.Header.Set("POLY_NONCE", strconv.Itoa(nonce))
	httpReq.Header.Set("POLY_API_KEY", c.credentials.APIKey)
	httpReq.Header.Set("POLY_PASSPHRASE", c.credentials.Passphrase)

	// 发送请求
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var orderStatus OrderStatusResponse
	if err := json.Unmarshal(body, &orderStatus); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, body: %s", err, string(body))
	}

	return &orderStatus, nil
}
