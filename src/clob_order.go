package src

import (
	"bytes"
	"context"
	"crypto/ecdsa"
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
}

// SignedOrderPayload 签名订单的 JSON 表示
type SignedOrderPayload struct {
	Salt          string `json:"salt"`
	Maker         string `json:"maker"`
	Signer        string `json:"signer"`
	Taker         string `json:"taker"`
	TokenId       string `json:"tokenId"`
	MakerAmount   string `json:"makerAmount"`
	TakerAmount   string `json:"takerAmount"`
	Expiration    string `json:"expiration"`
	Nonce         string `json:"nonce"`
	FeeRateBps    string `json:"feeRateBps"`
	Side          string `json:"side"`
	SignatureType int    `json:"signatureType"`
	Signature     string `json:"signature"`
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
	OperatorAddress = "0xC5d563A36AE78145C45a50134d48A1215220f80a"
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
	if c.credentials == nil {
		return nil, fmt.Errorf("未设置 API 凭证，请先调用 SetCredentials")
	}

	// 验证价格范围
	if price <= 0 || price >= 1 {
		return nil, fmt.Errorf("价格必须在 0 和 1 之间: %.4f", price)
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
	var expiration string
	if expirationSec > 0 {
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
		FeeRateBps:    "0", // 费率由 Polymarket 设置
		Nonce:         nonce,
		Signer:        c.address.Hex(),
		Expiration:    expiration,
		Side:          OrderBuilderSide(side),
		SignatureType: EOA,
	}

	// 确定使用哪个合约 (这里使用 NegRisk CTF Exchange)
	verifyingContract := NegRiskCTFExchange

	// 构建并签名订单 (使用本地的 ExchangeOrderBuilder)
	signedOrder, err := c.orderBuilder.BuildSignedOrder(c.privateKey, orderData, verifyingContract)
	if err != nil {
		return nil, fmt.Errorf("构建签名订单失败: %w", err)
	}

	// 转换为 API 请求格式
	orderPayload := &SignedOrderPayload{
		Salt:          signedOrder.Salt.String(),
		Maker:         signedOrder.Maker.Hex(),
		Signer:        signedOrder.Signer.Hex(),
		Taker:         signedOrder.Taker.Hex(),
		TokenId:       signedOrder.TokenId.String(),
		MakerAmount:   signedOrder.MakerAmount.String(),
		TakerAmount:   signedOrder.TakerAmount.String(),
		Expiration:    signedOrder.Expiration.String(),
		Nonce:         signedOrder.Nonce.String(),
		FeeRateBps:    signedOrder.FeeRateBps.String(),
		Side:          strconv.FormatInt(signedOrder.Side.Int64(), 10),
		SignatureType: int(signedOrder.SignatureType.Int64()),
		Signature:     "0x" + hex.EncodeToString(signedOrder.Signature),
	}

	// 构建请求
	postOrderReq := &PostOrderRequest{
		Order:     orderPayload,
		Owner:     c.credentials.APIKey,
		OrderType: string(orderType),
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

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", ClobHost+"/order", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加 L2 认证头
	timestamp := time.Now().Unix()
	nonce := 0

	// 签名请求
	signature, err := c.signL2Request("POST", "/order", string(jsonData), timestamp)
	if err != nil {
		return nil, fmt.Errorf("签名请求失败: %w", err)
	}

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

// signL2Request 签名 L2 请求
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

// PlaceLimitBuyOrder 下限价买单 (GTC)
func (c *ClobOrderClient) PlaceLimitBuyOrder(ctx context.Context, tokenID string, price float64, size float64) (*ClobOrderResult, error) {
	return c.PlaceOrder(ctx, tokenID, ClobOrderSideBuy, price, size, OrderTypeGTC, 0)
}

// PlaceLimitSellOrder 下限价卖单 (GTC)
func (c *ClobOrderClient) PlaceLimitSellOrder(ctx context.Context, tokenID string, price float64, size float64) (*ClobOrderResult, error) {
	return c.PlaceOrder(ctx, tokenID, ClobOrderSideSell, price, size, OrderTypeGTC, 0)
}

