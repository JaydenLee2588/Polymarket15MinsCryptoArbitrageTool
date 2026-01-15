package src

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/shopspring/decimal"
)

// OrderType 订单类型
type OrderType string

const (
	OrderTypeFOK OrderType = "FOK" // Fill-Or-Kill: 必须立即全部成交，否则取消
	OrderTypeGTC OrderType = "GTC" // Good-Til-Cancelled: 一直挂着直到成交或取消
	OrderTypeFAK OrderType = "FAK" // Fill-And-Kill: 立即成交能成交的部分，剩余取消
	OrderTypeGTD OrderType = "GTD" // Good-Til-Date: 挂到指定时间
)

// OrderSide 订单方向
type ClobOrderSide int64

const (
	ClobOrderSideBuy  ClobOrderSide = 0
	ClobOrderSideSell ClobOrderSide = 1
)

// ClobOrderClient 处理订单创建和提交到 Polymarket CLOB
type ClobOrderClient struct {
	privateKey   *ecdsa.PrivateKey
	address      common.Address
	orderBuilder *ExchangeOrderBuilder // 使用本地的 ExchangeOrderBuilder
	credentials  *ClobAPICredentials
	httpClient   *http.Client
	chainID      *big.Int
}

// ClobOrderResult 下单结果
type ClobOrderResult struct {
	Success     bool    // 是否成功
	OrderID     string  // 订单ID
	FilledPrice float64 // 实际成交价格
	FilledSize  float64 // 实际成交数量
	ErrorMsg    string  // 错误信息
}

// PostOrderRequest 下单请求结构
type PostOrderRequest struct {
	Order     *SignedOrderPayload `json:"order"`
	Owner     string              `json:"owner"`
	OrderType string              `json:"orderType"`
	PostOnly  *bool               `json:"postOnly,omitempty"` // 可选字段，GTC订单专用
}

// SignedOrderPayload 签名订单的 JSON 表示
type SignedOrderPayload struct {
	Salt          int64  `json:"salt"` // 必须是整数，不是字符串
	Maker         string `json:"maker"`
	Signer        string `json:"signer"`
	Taker         string `json:"taker"`
	TokenId       string `json:"tokenId"`
	MakerAmount   string `json:"makerAmount"`
	TakerAmount   string `json:"takerAmount"`
	Expiration    string `json:"expiration"`
	Nonce         string `json:"nonce"`
	FeeRateBps    string `json:"feeRateBps"`
	Side          string `json:"side"` // 必须是 "BUY" 或 "SELL"
	SignatureType int    `json:"signatureType"`
	Signature     string `json:"signature"`
}

// sideToString 将数字 side (0=BUY, 1=SELL) 转换为 API 字符串格式
func sideToString(side int64) string {
	if side == 0 {
		return "BUY"
	}
	return "SELL"
}

// ClobOrderResponse 下单响应
type ClobOrderResponse struct {
	Success     bool   `json:"success"`
	OrderID     string `json:"orderID"`
	ErrorMsg    string `json:"errorMsg"`
	FilledPrice string `json:"filledPrice,omitempty"`
	FilledSize  string `json:"filledSize,omitempty"`
}

// USDC 精度常量 (6 位小数)
const USDCDecimals = 6

// CTF Token 精度 (6 位小数)
const CTFDecimals = 6

// Polygon Mainnet Chain ID
var PolygonMainnetChainID = big.NewInt(137)

// 合约地址 (Polygon Mainnet)
const (
	// NegRisk CTF Exchange
	NegRiskCTFExchangeAddress = "0xC5d563A36AE78145C45a50134d48A1215220f80a"
	// CTF Exchange
	CTFExchangeAddress = "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"
	// Operator address (taker address for public orders)
	OperatorAddress = "0x0000000000000000000000000000000000000000"
)

// NewClobOrderClient 创建新的订单客户端
func NewClobOrderClient(privateKeyHex string) (*ClobOrderClient, error) {
	// 移除 0x 前缀
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")

	// 解析私钥
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("无效的私钥格式: %w", err)
	}

	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("无法解析私钥: %w", err)
	}

	// 从私钥派生地址
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("无法获取公钥")
	}
	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	// 创建订单构建器 (使用本地的 ExchangeOrderBuilder)
	orderBuilder := NewExchangeOrderBuilder(PolygonMainnetChainID, nil)

	return &ClobOrderClient{
		privateKey:   privateKey,
		address:      address,
		orderBuilder: orderBuilder,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		chainID:      PolygonMainnetChainID,
	}, nil
}

// SetCredentials 设置 API 凭证
func (c *ClobOrderClient) SetCredentials(creds *ClobAPICredentials) {
	c.credentials = creds
}

// GetAddress 返回钱包地址
func (c *ClobOrderClient) GetAddress() string {
	return c.address.Hex()
}

// PlaceOrder 下单（核心函数）
// tokenID: CTF Token ID
// side: 买/卖方向
// price: 价格 (0.00 - 1.00)
// size: 数量 (shares)
// orderType: 订单类型 (FOK/GTC/FAK/GTD)
// expirationSec: 过期时间（秒），0 表示不过期
func (c *ClobOrderClient) PlaceOrder(
	ctx context.Context,
	tokenID string,
	side ClobOrderSide,
	price float64,
	size float64,
	orderType OrderType,
	expirationSec int64,
) (*ClobOrderResult, error) {
	// ✅ 验证订单类型
	validOrderTypes := map[OrderType]bool{
		OrderTypeFOK: true,
		OrderTypeGTC: true,
		OrderTypeFAK: true,
		OrderTypeGTD: true,
	}
	if !validOrderTypes[orderType] {
		return nil, fmt.Errorf("无效的订单类型: %s (必须是 FOK, GTC, FAK, GTD 之一)", orderType)
	}
	if c.credentials == nil {
		return nil, fmt.Errorf("未设置 API 凭证，请先调用 SetCredentials")
	}

	// 验证价格范围
	if price <= 0 || price >= 1 {
		return nil, fmt.Errorf("价格必须在 0 和 1 之间: %.4f", price)
	}

	// ✅ 确保订单金额 >= $1 (Polymarket API 最低要求)
	minOrderValue := 1.0
	orderValue := price * size
	if orderValue < minOrderValue {
		// 自动增加 share 数量以满足最低订单金额
		adjustedSize := minOrderValue / price
		// 向上取整到整数 share
		adjustedSize = float64(int(adjustedSize) + 1)
		fmt.Printf("📊 [订单调整] 原金额 $%.2f < $1, 自动调整: %.0f shares → %.0f shares\n", orderValue, size, adjustedSize)
		size = adjustedSize
	}

	// 计算 makerAmount 和 takerAmount
	// 对于 BUY: makerAmount = 支付的 USDC, takerAmount = 获得的 shares
	// 对于 SELL: makerAmount = 卖出的 shares, takerAmount = 获得的 USDC
	priceDecimal := decimal.NewFromFloat(price)
	sizeDecimal := decimal.NewFromFloat(size)
	multiplier := decimal.NewFromInt(1000000) // 6 位小数

	var makerAmount, takerAmount *big.Int

	if side == ClobOrderSideBuy {
		// BUY: 支付 price * size USDC，获得 size shares
		usdcAmount := priceDecimal.Mul(sizeDecimal).Mul(multiplier)
		sharesAmount := sizeDecimal.Mul(multiplier)
		makerAmount = usdcAmount.BigInt()
		takerAmount = sharesAmount.BigInt()
	} else {
		// SELL: 卖出 size shares，获得 price * size USDC
		sharesAmount := sizeDecimal.Mul(multiplier)
		usdcAmount := priceDecimal.Mul(sizeDecimal).Mul(multiplier)
		makerAmount = sharesAmount.BigInt()
		takerAmount = usdcAmount.BigInt()
	}

	// 计算过期时间
	// 注意: 只有 GTD (Good-Til-Date) 订单可以设置过期时间
	// 对于 FOK, GTC, FAK 订单，expiration 必须为 "0"
	var expiration string
	if orderType == OrderTypeGTD && expirationSec > 0 {
		expirationTime := time.Now().Unix() + expirationSec
		expiration = strconv.FormatInt(expirationTime, 10)
	} else {
		expiration = "0"
	}

	// 获取当前 nonce (简化: 使用时间戳)
	nonce := strconv.FormatInt(time.Now().UnixNano(), 10)

	// 构建订单数据 (使用本地的 OrderData 类型)
	orderData := &OrderData{
		Maker:         c.address.Hex(),
		Taker:         OperatorAddress, // Polymarket 操作员地址
		TokenId:       tokenID,
		MakerAmount:   makerAmount.String(),
		TakerAmount:   takerAmount.String(),
		FeeRateBps:    "1000", // 15分钟市场要求的费率 (10%)
		Nonce:         nonce,
		Signer:        c.address.Hex(),
		Expiration:    expiration,
		Side:          OrderBuilderSide(side),
		SignatureType: EOA, // 从 Polymarket 导出的私钥是 EOA 类型
	}

	// 确定使用哪个合约 (测试: 使用普通 CTF Exchange)
	verifyingContract := CTFExchange

	// 构建并签名订单 (使用本地的 ExchangeOrderBuilder)
	signedOrder, err := c.orderBuilder.BuildSignedOrder(c.privateKey, orderData, verifyingContract)
	if err != nil {
		return nil, fmt.Errorf("构建签名订单失败: %w", err)
	}

	// 转换为 API 请求格式
	orderPayload := &SignedOrderPayload{
		Salt:          signedOrder.Salt.Int64(), // 必须是整数
		Maker:         signedOrder.Maker.Hex(),
		Signer:        signedOrder.Signer.Hex(),
		Taker:         signedOrder.Taker.Hex(),
		TokenId:       signedOrder.TokenId.String(),
		MakerAmount:   signedOrder.MakerAmount.String(),
		TakerAmount:   signedOrder.TakerAmount.String(),
		Expiration:    signedOrder.Expiration.String(),
		Nonce:         signedOrder.Nonce.String(),
		FeeRateBps:    signedOrder.FeeRateBps.String(),
		Side:          sideToString(signedOrder.Side.Int64()), // 必须是 "BUY" 或 "SELL"
		SignatureType: int(signedOrder.SignatureType.Int64()),
		Signature:     "0x" + hex.EncodeToString(signedOrder.Signature),
	}

	// 构建请求
	// 注意: orderType 必须是大写 (FOK, GTC, FAK, GTD)
	postOrderReq := &PostOrderRequest{
		Order:     orderPayload,
		Owner:     c.credentials.APIKey, // Polymarket 要求 owner 是 API Key
		OrderType: string(orderType),    // OrderType 已经是大写常量
		PostOnly:  nil,                  // GTC 订单可以设置为 true 以做 maker
	}

	// 发送请求
	return c.submitOrder(ctx, postOrderReq)
}

// submitOrder 提交订单到 CLOB API
func (c *ClobOrderClient) submitOrder(ctx context.Context, req *PostOrderRequest) (*ClobOrderResult, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 🔍 DEBUG: 打印完整的请求payload
	fmt.Printf("📤 [DEBUG] Order Payload:\n%s\n", string(jsonData))

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", ClobHost+"/order", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加 L2 认证头 (使用 HMAC-SHA256 签名)
	timestamp := time.Now().Unix()

	// 使用 HMAC-SHA256 签名请求
	signature, err := c.signL2HMAC("POST", "/order", string(jsonData), timestamp)
	if err != nil {
		return nil, fmt.Errorf("签名请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("POLY_ADDRESS", c.address.Hex())
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

	// 🔍 DEBUG: 打印完整的响应
	fmt.Printf("📥 [DEBUG] Response Status: %d\n📥 [DEBUG] Response Body: %s\n", resp.StatusCode, string(body))

	// 解析响应
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return &ClobOrderResult{
			Success:  false,
			ErrorMsg: fmt.Sprintf("API 错误 (%d): %s", resp.StatusCode, string(body)),
		}, nil
	}

	var orderResp ClobOrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, body: %s", err, string(body))
	}

	result := &ClobOrderResult{
		Success:  orderResp.Success,
		OrderID:  orderResp.OrderID,
		ErrorMsg: orderResp.ErrorMsg,
	}

	// 解析成交信息
	if orderResp.FilledPrice != "" {
		if fp, err := strconv.ParseFloat(orderResp.FilledPrice, 64); err == nil {
			result.FilledPrice = fp
		}
	}
	if orderResp.FilledSize != "" {
		if fs, err := strconv.ParseFloat(orderResp.FilledSize, 64); err == nil {
			result.FilledSize = fs
		}
	}

	return result, nil
}

// signL2HMAC 使用 HMAC-SHA256 签名 L2 请求 (Polymarket 官方格式)
// 消息格式: timestamp + method + path + body
// 使用 API Secret (Base64 解码后) 作为密钥
func (c *ClobOrderClient) signL2HMAC(method, path, body string, timestamp int64) (string, error) {
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

// signL2Request 保留旧方法用于兼容性 (使用私钥签名，用于某些特殊端点)
func (c *ClobOrderClient) signL2Request(method, path, body string, timestamp int64) (string, error) {
	// L2 签名格式: method + timestamp + path + body
	message := method + strconv.FormatInt(timestamp, 10) + path + body
	messageHash := crypto.Keccak256([]byte(message))

	signature, err := crypto.Sign(messageHash, c.privateKey)
	if err != nil {
		return "", err
	}

	// 调整 v 值
	if signature[64] < 27 {
		signature[64] += 27
	}

	return "0x" + hex.EncodeToString(signature), nil
}

// PlaceMarketBuyOrder 下市价买单 (FOK)
func (c *ClobOrderClient) PlaceMarketBuyOrder(ctx context.Context, tokenID string, price float64, size float64, expirationSec int64) (*ClobOrderResult, error) {
	return c.PlaceOrder(ctx, tokenID, ClobOrderSideBuy, price, size, OrderTypeFOK, expirationSec)
}

// PlaceMarketSellOrder 下市价卖单 (FOK)
func (c *ClobOrderClient) PlaceMarketSellOrder(ctx context.Context, tokenID string, price float64, size float64, expirationSec int64) (*ClobOrderResult, error) {
	return c.PlaceOrder(ctx, tokenID, ClobOrderSideSell, price, size, OrderTypeFOK, expirationSec)
}

// PlaceMarketBuyOrderFAK 下市价买单 (FAK - Fill-And-Kill，允许部分成交)
func (c *ClobOrderClient) PlaceMarketBuyOrderFAK(ctx context.Context, tokenID string, price float64, size float64, expirationSec int64) (*ClobOrderResult, error) {
	return c.PlaceOrder(ctx, tokenID, ClobOrderSideBuy, price, size, OrderTypeFAK, expirationSec)
}

// PlaceMarketSellOrderFAK 下市价卖单 (FAK - Fill-And-Kill，允许部分成交)
func (c *ClobOrderClient) PlaceMarketSellOrderFAK(ctx context.Context, tokenID string, price float64, size float64, expirationSec int64) (*ClobOrderResult, error) {
	return c.PlaceOrder(ctx, tokenID, ClobOrderSideSell, price, size, OrderTypeFAK, expirationSec)
}

// PlaceLimitBuyOrder 下限价买单 (GTC)
func (c *ClobOrderClient) PlaceLimitBuyOrder(ctx context.Context, tokenID string, price float64, size float64) (*ClobOrderResult, error) {
	return c.PlaceOrder(ctx, tokenID, ClobOrderSideBuy, price, size, OrderTypeGTC, 0)
}

// PlaceLimitSellOrder 下限价卖单 (GTC)
func (c *ClobOrderClient) PlaceLimitSellOrder(ctx context.Context, tokenID string, price float64, size float64) (*ClobOrderResult, error) {
	return c.PlaceOrder(ctx, tokenID, ClobOrderSideSell, price, size, OrderTypeGTC, 0)
}
