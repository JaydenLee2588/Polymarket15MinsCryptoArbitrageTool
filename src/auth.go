package src

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ClobAPICredentials 包含 CLOB API 的完整凭证
type ClobAPICredentials struct {
	APIKey     string `json:"apiKey"`
	APISecret  string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

// AuthClient 用于处理 Polymarket CLOB 认证
type AuthClient struct {
	host       string
	chainID    int
	privateKey *ecdsa.PrivateKey
	address    common.Address
	httpClient *http.Client
}

// CLOB 认证相关常量
const (
	// ClobHost 是 Polymarket CLOB API 的主机地址
	ClobHost = "https://clob.polymarket.com"

	// Polygon Mainnet Chain ID
	PolygonChainID = 137

	// EIP-712 域配置
	ClobDomainName    = "ClobAuthDomain"
	ClobDomainVersion = "1"
	MsgToSign         = "This message attests that I control the given wallet"
)

// NewAuthClient 创建一个新的认证客户端
// privateKeyHex 是您从 Polymarket 导出的私钥（可以带或不带 0x 前缀）
func NewAuthClient(privateKeyHex string) (*AuthClient, error) {
	return NewAuthClientWithOptions(privateKeyHex, ClobHost, PolygonChainID, http.DefaultClient)
}

// NewAuthClientWithOptions 创建一个带有自定义选项的认证客户端
func NewAuthClientWithOptions(privateKeyHex, host string, chainID int, httpClient *http.Client) (*AuthClient, error) {
	// 移除 0x 前缀（如果存在）
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

	return &AuthClient{
		host:       strings.TrimSuffix(host, "/"),
		chainID:    chainID,
		privateKey: privateKey,
		address:    address,
		httpClient: httpClient,
	}, nil
}

// GetAddress 返回与私钥关联的钱包地址
func (c *AuthClient) GetAddress() string {
	return c.address.Hex()
}

// DeriveAPICredentials 从私钥派生 CLOB API 凭证
// 如果凭证不存在，会先尝试创建，然后派生
func (c *AuthClient) DeriveAPICredentials(ctx context.Context) (*ClobAPICredentials, error) {
	// 首先尝试派生现有凭证
	creds, err := c.deriveAPIKey(ctx)
	if err != nil {
		// 如果派生失败，尝试创建新凭证
		creds, err = c.createAPIKey(ctx)
		if err != nil {
			return nil, fmt.Errorf("创建或派生 API 凭证失败: %w", err)
		}
	}
	return creds, nil
}

// CreateAPICredentials 创建新的 CLOB API 凭证
func (c *AuthClient) CreateAPICredentials(ctx context.Context) (*ClobAPICredentials, error) {
	return c.createAPIKey(ctx)
}

// createAPIKey 调用 CLOB API 创建新的 API Key
func (c *AuthClient) createAPIKey(ctx context.Context) (*ClobAPICredentials, error) {
	endpoint := c.host + "/auth/api-key"
	return c.doAuthRequest(ctx, "POST", endpoint)
}

// deriveAPIKey 调用 CLOB API 派生现有的 API Key
func (c *AuthClient) deriveAPIKey(ctx context.Context) (*ClobAPICredentials, error) {
	endpoint := c.host + "/auth/derive-api-key"
	return c.doAuthRequest(ctx, "GET", endpoint)
}

// doAuthRequest 执行带有 L1 认证头的请求
func (c *AuthClient) doAuthRequest(ctx context.Context, method, endpoint string) (*ClobAPICredentials, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加 L1 认证头
	timestamp := time.Now().Unix()
	nonce := 0

	signature, err := c.signClobAuthMessage(timestamp, nonce)
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}

	req.Header.Set("POLY_ADDRESS", c.address.Hex())
	req.Header.Set("POLY_SIGNATURE", signature)
	req.Header.Set("POLY_TIMESTAMP", strconv.FormatInt(timestamp, 10))
	req.Header.Set("POLY_NONCE", strconv.Itoa(nonce))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, string(body))
	}

	var creds ClobAPICredentials
	if err := json.Unmarshal(body, &creds); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &creds, nil
}

// signClobAuthMessage 使用 EIP-712 签名认证消息
func (c *AuthClient) signClobAuthMessage(timestamp int64, nonce int) (string, error) {
	// 构建 EIP-712 类型化数据
	// 这是简化版本，实际实现需要完整的 EIP-712 结构

	// 域分隔符哈希
	domainSeparator := c.getDomainSeparator()

	// 消息结构哈希
	structHash := c.getStructHash(timestamp, nonce)

	// 最终消息哈希
	messageHash := crypto.Keccak256(
		[]byte("\x19\x01"),
		domainSeparator,
		structHash,
	)

	// 签名
	signature, err := crypto.Sign(messageHash, c.privateKey)
	if err != nil {
		return "", fmt.Errorf("签名失败: %w", err)
	}

	// 调整 v 值 (以太坊签名格式)
	if signature[64] < 27 {
		signature[64] += 27
	}

	return "0x" + hex.EncodeToString(signature), nil
}

// getDomainSeparator 计算 EIP-712 域分隔符
func (c *AuthClient) getDomainSeparator() []byte {
	// EIP-712 域类型哈希
	domainTypeHash := crypto.Keccak256([]byte("EIP712Domain(string name,string version,uint256 chainId)"))

	// 域值
	nameHash := crypto.Keccak256([]byte(ClobDomainName))
	versionHash := crypto.Keccak256([]byte(ClobDomainVersion))

	// 编码 chainId
	chainIDBytes := make([]byte, 32)
	chainIDBytes[31] = byte(c.chainID & 0xff)
	chainIDBytes[30] = byte((c.chainID >> 8) & 0xff)

	// 计算域分隔符
	return crypto.Keccak256(
		domainTypeHash,
		nameHash,
		versionHash,
		chainIDBytes,
	)
}

// getStructHash 计算消息结构哈希
func (c *AuthClient) getStructHash(timestamp int64, nonce int) []byte {
	// ClobAuth 结构类型哈希
	typeHash := crypto.Keccak256([]byte("ClobAuth(address address,string timestamp,uint256 nonce,string message)"))

	// 填充地址到 32 字节
	addressBytes := make([]byte, 32)
	copy(addressBytes[12:], c.address.Bytes())

	// 时间戳字符串哈希
	timestampHash := crypto.Keccak256([]byte(strconv.FormatInt(timestamp, 10)))

	// nonce (32 字节)
	nonceBytes := make([]byte, 32)
	nonceBytes[31] = byte(nonce & 0xff)

	// 消息哈希
	messageHash := crypto.Keccak256([]byte(MsgToSign))

	return crypto.Keccak256(
		typeHash,
		addressBytes,
		timestampHash,
		nonceBytes,
		messageHash,
	)
}

// ToClobAuth 将凭证转换为 ClobAuth 结构（用于 WebSocket 订阅）
func (creds *ClobAPICredentials) ToClobAuth() *ClobAuth {
	return &ClobAuth{
		Key:        creds.APIKey,
		Secret:     creds.APISecret,
		Passphrase: creds.Passphrase,
	}
}

