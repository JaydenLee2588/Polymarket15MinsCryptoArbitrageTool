package src

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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

	// 创建 HTTP 请求
	fullURL := ClobHost + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 获取当前时间戳
	timestamp := time.Now().Unix()

	// 使用 HMAC-SHA256 签名请求
	signature, err := c.signL2HMACForStatus("GET", endpoint, "", timestamp)
	if err != nil {
		return nil, fmt.Errorf("签名请求失败: %w", err)
	}

	// 添加 L2 认证头 (使用下划线格式，与官方客户端一致)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("POLY_ADDRESS", c.apiAddress.Hex())
	httpReq.Header.Set("POLY_SIGNATURE", signature)
	httpReq.Header.Set("POLY_TIMESTAMP", strconv.FormatInt(timestamp, 10))
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

// signL2HMACForStatus 使用 HMAC-SHA256 签名 L2 请求 (用于订单状态查询)
// 消息格式: timestamp + method + path + body
// 使用 API Secret (Base64 解码后) 作为密钥
func (c *ClobOrderClient) signL2HMACForStatus(method, path, body string, timestamp int64) (string, error) {
	if c.credentials == nil || c.credentials.APISecret == "" {
		return "", fmt.Errorf("未设置 API Secret")
	}

	// 解码 Base64 格式的 API Secret (支持标准和 URL-safe Base64)
	secret := c.credentials.APISecret
	// 先将 URL-safe 转换为标准 Base64
	secret = strings.ReplaceAll(secret, "-", "+")
	secret = strings.ReplaceAll(secret, "_", "/")

	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("解码 API Secret 失败: %w", err)
	}

	// 构建待签名消息: timestamp + method + path + body
	message := strconv.FormatInt(timestamp, 10) + method + path + body

	// 计算 HMAC-SHA256
	h := hmac.New(sha256.New, secretBytes)
	h.Write([]byte(message))
	signatureBytes := h.Sum(nil)

	// 返回 Base64 编码，然后转换为 URL-safe 格式
	sig := base64.StdEncoding.EncodeToString(signatureBytes)
	sig = strings.ReplaceAll(sig, "+", "-")
	sig = strings.ReplaceAll(sig, "/", "_")

	return sig, nil
}
