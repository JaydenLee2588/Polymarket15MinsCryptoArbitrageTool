// Package src provides the redemption service for Polymarket.
// This file implements automatic redemption of winning positions after market resolution
// using the Builder Relayer API for proxy wallet support.
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
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/sha3"
)

// RedemptionDelaySeconds 是决议后等待赎回的延迟时间 (秒)
const RedemptionDelaySeconds = 600 // 10分钟

// Builder Relayer API 常量
const (
	RelayerURL       = "https://relayer-v2.polymarket.com"
	RelayPayloadPath = "/relay-payload"
	SubmitPath       = "/submit"
	TransactionPath  = "/transaction"
	StateConfirmed   = "STATE_CONFIRMED"
	StateMined       = "STATE_MINED"
	StateFailed      = "STATE_FAILED"
)

// 合约地址 (Polygon Mainnet)
var (
	CTFAddress            = common.HexToAddress("0x4D97DCd97eC945f40cF65F87097ACe5EA0476045")
	USDCPolygon           = common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174")
	NegRiskAdapterAddress = common.HexToAddress("0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296")
	ProxyFactoryAddress   = common.HexToAddress("0xaB45c5A4B0c941a2F231C04C3f49182e1A254052")
	RelayHubAddress       = common.HexToAddress("0xD216153c06E857cD7f72665E0aF1d7D82172F494")
	ZeroBytes32           = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	ProxyInitCodeHash     = common.HexToHash("0xd21df8dc65880a8606f09fe0ce3df9b8869287ab0b058be05aa9e8af6330a00b")
)

// PendingRedemption 表示一个待赎回的持仓
type PendingRedemption struct {
	Slug         string    // 市场 slug
	ConditionID  string    // 市场条件 ID
	TokenID      string    // 获胜代币 ID
	Amount       float64   // 赎回数量
	ScheduledAt  time.Time // 计划执行时间
	ConfigGroup  string    // 配置组
	UserAddress  string    // 用户地址 (签名者)
	ProxyAddress string    // 代理钱包地址
	IsWinner     bool      // 是否获胜 (只有获胜方才能赎回)
	IsNegRisk    bool      // 是否是 NegRisk 市场
}

// BuilderCredentials Builder API 凭证
type BuilderCredentials struct {
	Key        string
	Secret     string
	Passphrase string
}

// Redeemer 赎回服务
type Redeemer struct {
	privateKey    *ecdsa.PrivateKey
	signerAddress common.Address // 签名者地址 (EOA)
	proxyAddress  common.Address // 代理钱包地址
	ethClient     *ethclient.Client
	httpClient    *http.Client
	chainID       *big.Int
	dataClient    *Client // 用于查询持仓
	builderCreds  *BuilderCredentials
	mu            sync.Mutex
	pendingQueue  map[string]*PendingRedemption // key: slug
	queueMu       sync.RWMutex
	stopChan      chan struct{}
	running       bool
}

// CTF 合约 ABI
const ctfABI = `[
	{
		"inputs": [{"internalType": "bytes32", "name": "", "type": "bytes32"}],
		"name": "payoutDenominator",
		"outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "collateralToken", "type": "address"},
			{"internalType": "bytes32", "name": "parentCollectionId", "type": "bytes32"},
			{"internalType": "bytes32", "name": "conditionId", "type": "bytes32"},
			{"internalType": "uint256[]", "name": "indexSets", "type": "uint256[]"}
		],
		"name": "redeemPositions",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

// NegRiskAdapter ABI
const negRiskAdapterABI = `[
	{
		"inputs": [
			{"internalType": "bytes32", "name": "conditionId", "type": "bytes32"},
			{"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}
		],
		"name": "redeemPositions",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

// ProxyWallet proxy function ABI
const proxyWalletABI = `[
	{
		"inputs": [
			{
				"components": [
					{"internalType": "uint8", "name": "typeCode", "type": "uint8"},
					{"internalType": "address", "name": "to", "type": "address"},
					{"internalType": "uint256", "name": "value", "type": "uint256"},
					{"internalType": "bytes", "name": "data", "type": "bytes"}
				],
				"internalType": "struct Call[]",
				"name": "calls",
				"type": "tuple[]"
			}
		],
		"name": "proxy",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

// NewRedeemer 创建新的赎回服务
func NewRedeemer(privateKeyHex, proxyWalletAddr, rpcURL string, dataClient *Client, builderCreds *BuilderCredentials) (*Redeemer, error) {
	// 解析私钥
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("无效的私钥格式: %w", err)
	}

	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("无法解析私钥: %w", err)
	}

	// 派生签名者地址
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("无法获取公钥")
	}
	signerAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	// 代理钱包地址
	proxyAddress := common.HexToAddress(proxyWalletAddr)

	// 连接到 Polygon 节点
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("连接 RPC 失败: %w", err)
	}

	return &Redeemer{
		privateKey:    privateKey,
		signerAddress: signerAddress,
		proxyAddress:  proxyAddress,
		ethClient:     client,
		httpClient:    &http.Client{Timeout: 60 * time.Second},
		chainID:       big.NewInt(137), // Polygon Mainnet
		dataClient:    dataClient,
		builderCreds:  builderCreds,
		pendingQueue:  make(map[string]*PendingRedemption),
		stopChan:      make(chan struct{}),
	}, nil
}

// ScheduleRedemption 计划一个赎回任务 (5分钟后执行)
func (r *Redeemer) ScheduleRedemption(slug, conditionID, tokenID, userAddress, proxyAddress string, amount float64, isWinner, isNegRisk bool, configGroup string) {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()

	// 只有获胜方才能赎回
	if !isWinner {
		log.Printf("⏭️ [赎回] %s: 持仓失败，无需赎回", slug)
		return
	}

	// 检查是否已在队列中
	if _, exists := r.pendingQueue[slug]; exists {
		log.Printf("⚠️ [赎回] %s: 已在队列中，跳过", slug)
		return
	}

	scheduledAt := time.Now().Add(time.Duration(RedemptionDelaySeconds) * time.Second)

	r.pendingQueue[slug] = &PendingRedemption{
		Slug:         slug,
		ConditionID:  conditionID,
		TokenID:      tokenID,
		Amount:       amount,
		ScheduledAt:  scheduledAt,
		ConfigGroup:  configGroup,
		UserAddress:  userAddress,
		ProxyAddress: proxyAddress,
		IsWinner:     isWinner,
		IsNegRisk:    isNegRisk,
	}

	log.Printf("📋 [赎回] 已计划: %s, 将在 %s 执行 (%.0f秒后)", slug, scheduledAt.Format("15:04:05"), float64(RedemptionDelaySeconds))
}

// Start 启动赎回处理循环
func (r *Redeemer) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	log.Println("🔄 [赎回] 服务已启动")

	go r.processLoop()
}

// Stop 停止赎回服务
func (r *Redeemer) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	close(r.stopChan)
	r.running = false
	log.Println("🛑 [赎回] 服务已停止")
}

// processLoop 处理赎回队列的主循环
func (r *Redeemer) processLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopChan:
			return
		case <-ticker.C:
			r.processPendingRedemptions()
		}
	}
}

// processPendingRedemptions 处理到期的赎回请求
func (r *Redeemer) processPendingRedemptions() {
	r.queueMu.Lock()
	now := time.Now()
	var toProcess []*PendingRedemption

	for slug, pending := range r.pendingQueue {
		if now.After(pending.ScheduledAt) {
			toProcess = append(toProcess, pending)
			delete(r.pendingQueue, slug)
		}
	}
	r.queueMu.Unlock()

	// 处理到期的赎回
	for _, pending := range toProcess {
		r.executeRedemption(pending)
	}
}

// executeRedemption 执行单个赎回
func (r *Redeemer) executeRedemption(pending *PendingRedemption) {
	log.Printf("💰 [赎回] 正在执行: %s (ConditionID: %s)", pending.Slug, pending.ConditionID)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 构建赎回交易数据
	var calldata []byte
	var targetAddr common.Address
	var err error

	if pending.IsNegRisk {
		// NegRisk 市场赎回
		calldata, err = r.buildNegRiskRedeemData(pending.ConditionID, pending.Amount)
		targetAddr = NegRiskAdapterAddress
	} else {
		// 标准 CTF 赎回
		calldata, err = r.buildCTFRedeemData(pending.ConditionID)
		targetAddr = CTFAddress
	}

	if err != nil {
		log.Printf("❌ [赎回] 构建交易数据失败: %v", err)
		return
	}

	// 通过 Builder Relayer API 提交赎回交易
	txID, err := r.submitViaRelayer(ctx, targetAddr, calldata)
	if err != nil {
		log.Printf("❌ [赎回] 提交交易失败: %v", err)
		return
	}

	// 等待交易确认
	result, err := r.pollTransactionStatus(ctx, txID)
	if err != nil {
		log.Printf("⚠️ [赎回] 等待确认超时: %v, TxID: %s", err, txID)
		return
	}

	if result == StateFailed {
		log.Printf("❌ [赎回] %s 交易失败, TxID: %s", pending.Slug, txID)
		return
	}

	log.Printf("✅ [赎回] %s 成功! TxID: %s", pending.Slug, txID)
}

// buildCTFRedeemData 构建标准 CTF 赎回交易数据
func (r *Redeemer) buildCTFRedeemData(conditionID string) ([]byte, error) {
	parsedABI, err := abi.JSON(strings.NewReader(ctfABI))
	if err != nil {
		return nil, fmt.Errorf("解析 ABI 失败: %w", err)
	}

	conditionIDBytes := common.HexToHash(conditionID)
	// indexSets: [1, 2] 表示赎回两个结果的代币
	indexSets := []*big.Int{big.NewInt(1), big.NewInt(2)}

	data, err := parsedABI.Pack("redeemPositions", USDCPolygon, ZeroBytes32, conditionIDBytes, indexSets)
	if err != nil {
		return nil, fmt.Errorf("编码调用数据失败: %w", err)
	}

	return data, nil
}

// buildNegRiskRedeemData 构建 NegRisk 赎回交易数据
func (r *Redeemer) buildNegRiskRedeemData(conditionID string, amount float64) ([]byte, error) {
	parsedABI, err := abi.JSON(strings.NewReader(negRiskAdapterABI))
	if err != nil {
		return nil, fmt.Errorf("解析 ABI 失败: %w", err)
	}

	conditionIDBytes := common.HexToHash(conditionID)
	// 转换为 6 位精度
	amountInt := big.NewInt(int64(amount * 1e6))
	amounts := []*big.Int{amountInt, amountInt}

	data, err := parsedABI.Pack("redeemPositions", conditionIDBytes, amounts)
	if err != nil {
		return nil, fmt.Errorf("编码调用数据失败: %w", err)
	}

	return data, nil
}

// encodeProxyTransaction 编码代理钱包交易
func (r *Redeemer) encodeProxyTransaction(to common.Address, data []byte) ([]byte, error) {
	parsedABI, err := abi.JSON(strings.NewReader(proxyWalletABI))
	if err != nil {
		return nil, fmt.Errorf("解析代理钱包 ABI 失败: %w", err)
	}

	// 构建 Call 结构
	type Call struct {
		TypeCode uint8
		To       common.Address
		Value    *big.Int
		Data     []byte
	}

	calls := []Call{{
		TypeCode: 1,
		To:       to,
		Value:    big.NewInt(0),
		Data:     data,
	}}

	proxyData, err := parsedABI.Pack("proxy", calls)
	if err != nil {
		return nil, fmt.Errorf("编码代理交易失败: %w", err)
	}

	return proxyData, nil
}

// RelayPayload Relayer API 返回的 payload
type RelayPayload struct {
	Address string `json:"address"`
	Nonce   int    `json:"nonce"`
}

// getRelayPayload 获取 Relay 节点信息和 nonce
func (r *Redeemer) getRelayPayload(ctx context.Context) (*RelayPayload, error) {
	// type=PROXY (必须大写，参照 poly-web3)
	url := fmt.Sprintf("%s%s?address=%s&type=PROXY", RelayerURL, RelayPayloadPath, r.signerAddress.Hex())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload RelayPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, body: %s", err, string(body))
	}

	return &payload, nil
}

// ... (removed erroneous comments)

// createStructHash 创建结构哈希用于代理钱包交易签名
// 完全参照 poly-web3/signature/build.py create_struct_hash 实现
func (r *Redeemer) createStructHash(from, to common.Address, data []byte, relayerFee, gasPrice, gasLimit string, nonce int, relayHub, relay common.Address) common.Hash {
	// 参照 poly-web3: relay_hub_prefix = to_bytes("rlx:")
	relayHubPrefix := []byte("rlx:")

	// 将数字字符串转为 32 字节大端序
	toBytesUint256 := func(s string) []byte {
		n := new(big.Int)
		n.SetString(s, 10)
		return common.LeftPadBytes(n.Bytes(), 32)
	}

	// 构建待哈希数据 (参照 poly-web3)
	var buf bytes.Buffer
	buf.Write(relayHubPrefix)                      // "rlx:"
	buf.Write(from.Bytes())                        // from address (20 bytes)
	buf.Write(to.Bytes())                          // to address (20 bytes)
	buf.Write(data)                                // data (variable length)
	buf.Write(toBytesUint256(relayerFee))          // tx_fee (32 bytes)
	buf.Write(toBytesUint256(gasPrice))            // gas_price (32 bytes)
	buf.Write(toBytesUint256(gasLimit))            // gas_limit (32 bytes)
	buf.Write(toBytesUint256(strconv.Itoa(nonce))) // nonce (32 bytes)
	buf.Write(relayHub.Bytes())                    // relay_hub_address (20 bytes)
	buf.Write(relay.Bytes())                       // relay_address (20 bytes)

	return crypto.Keccak256Hash(buf.Bytes())
}

// signMessage 签名消息 (EIP-191 personal sign)
func (r *Redeemer) signMessage(message []byte) ([]byte, error) {
	// EIP-191 前缀
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	prefixedMessage := append([]byte(prefix), message...)

	hash := crypto.Keccak256(prefixedMessage)
	signature, err := crypto.Sign(hash, r.privateKey)
	if err != nil {
		return nil, err
	}

	// 调整 v 值
	if signature[64] < 27 {
		signature[64] += 27
	}

	return signature, nil
}

// deriveProxyWallet 计算代理钱包地址
func deriveProxyWallet(signer, factory common.Address, initCodeHash common.Hash) common.Address {
	// CREATE2: keccak256(0xff ++ factory ++ salt ++ initCodeHash)
	// salt = keccak256(signer)
	salt := crypto.Keccak256Hash(signer[:])

	var buf bytes.Buffer
	buf.WriteByte(0xff)
	buf.Write(factory[:])
	buf.Write(salt[:])
	buf.Write(initCodeHash[:])

	hash := crypto.Keccak256(buf.Bytes())
	return common.BytesToAddress(hash[12:])
}

// SubmitRequest 提交交易请求结构
type SubmitRequest struct {
	From            string                 `json:"from"`
	To              string                 `json:"to"`
	ProxyWallet     string                 `json:"proxyWallet"`
	Data            string                 `json:"data"`
	Nonce           int                    `json:"nonce"`
	Signature       string                 `json:"signature"`
	SignatureParams map[string]interface{} `json:"signatureParams"`
	Type            string                 `json:"type"`
	Metadata        string                 `json:"metadata"`
}

// SubmitResponse 提交交易响应
type SubmitResponse struct {
	TransactionID string `json:"transactionID"`
}

// submitViaRelayer 通过 Builder Relayer API 提交交易
func (r *Redeemer) submitViaRelayer(ctx context.Context, to common.Address, calldata []byte) (string, error) {
	// 1. 获取 relay payload (nonce 和 relay 地址)
	relayPayload, err := r.getRelayPayload(ctx)
	if err != nil {
		return "", fmt.Errorf("获取 relay payload 失败: %w", err)
	}

	// 2. 编码代理交易
	proxyData, err := r.encodeProxyTransaction(to, calldata)
	if err != nil {
		return "", fmt.Errorf("编码代理交易失败: %w", err)
	}

	// 3. 估算 gas
	gasLimit := "300000" // 保守估计

	// 4. 创建签名哈希
	relayAddr := common.HexToAddress(relayPayload.Address)
	structHash := r.createStructHash(
		r.signerAddress,
		ProxyFactoryAddress,
		proxyData,
		"0", // relayerFee
		"0", // gasPrice
		gasLimit,
		relayPayload.Nonce,
		RelayHubAddress,
		relayAddr,
	)

	// 5. 签名
	signature, err := r.signMessage(structHash[:])
	if err != nil {
		return "", fmt.Errorf("签名失败: %w", err)
	}

	// 6. 派生 proxyWallet (参照 poly-web3: proxy = derive_proxy_wallet(from, to, PROXY_INIT_CODE_HASH))
	derivedProxy := deriveProxyWallet(r.signerAddress, ProxyFactoryAddress, ProxyInitCodeHash)

	// 6. 构建请求
	submitReq := SubmitRequest{
		From:        r.signerAddress.Hex(),
		To:          ProxyFactoryAddress.Hex(),
		ProxyWallet: derivedProxy.Hex(), // 使用派生的地址，而非传入的地址
		Data:        "0x" + hex.EncodeToString(proxyData),
		Nonce:       relayPayload.Nonce,
		Signature:   "0x" + hex.EncodeToString(signature),
		SignatureParams: map[string]interface{}{
			"gasPrice":   "0",
			"gasLimit":   gasLimit,
			"relayerFee": "0",
			"relayHub":   RelayHubAddress.Hex(),
			"relay":      relayPayload.Address,
		},
		Type:     "PROXY", // 必须大写
		Metadata: "redeem",
	}

	// 7. 发送请求 (带 Builder 签名)
	reqBody, err := json.Marshal(submitReq)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// Debug: 打印请求内容
	log.Printf("🔧 [赎回] 请求内容: from=%s, to=%s, proxyWallet=%s (派生), nonce=%d",
		submitReq.From, submitReq.To, submitReq.ProxyWallet, submitReq.Nonce)

	req, err := http.NewRequestWithContext(ctx, "POST", RelayerURL+SubmitPath, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}

	// 添加 Builder 认证头
	r.addBuilderHeaders(req, "POST", SubmitPath, reqBody)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, string(body))
	}

	var submitResp SubmitResponse
	if err := json.Unmarshal(body, &submitResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w, body: %s", err, string(body))
	}

	return submitResp.TransactionID, nil
}

// addBuilderHeaders 添加 Builder API 认证头
func (r *Redeemer) addBuilderHeaders(req *http.Request, method, path string, body []byte) {
	if r.builderCreds == nil {
		return
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// HMAC-SHA256 签名: timestamp + method + path + body
	message := timestamp + method + path + string(body)

	// 解码 secret (与 clob_order.go signL2HMAC 保持一致)
	var secretBytes []byte
	var err error

	// 1. 优先 URLEncoding
	secretBytes, err = base64.URLEncoding.DecodeString(r.builderCreds.Secret)
	if err != nil {
		// 2. 尝试 RawURLEncoding (无 padding)
		secretBytes, err = base64.RawURLEncoding.DecodeString(r.builderCreds.Secret)
		if err != nil {
			// 3. 最后尝试 StdEncoding
			secretBytes, err = base64.StdEncoding.DecodeString(r.builderCreds.Secret)
			if err != nil {
				log.Printf("⚠️ [赎回] 解码 Builder Secret 失败: %v", err)
				return
			}
		}
	}

	h := hmac.New(sha256.New, secretBytes)
	h.Write([]byte(message))
	// 返回 URL-Safe Base64 编码 (与 clob_order.go 保持一致)
	signature := base64.URLEncoding.EncodeToString(h.Sum(nil))

	req.Header.Set("POLY_BUILDER_API_KEY", r.builderCreds.Key)
	req.Header.Set("POLY_BUILDER_PASSPHRASE", r.builderCreds.Passphrase)
	req.Header.Set("POLY_BUILDER_TIMESTAMP", timestamp)
	req.Header.Set("POLY_BUILDER_SIGNATURE", signature)
}

// TransactionStatus 交易状态响应
type TransactionStatus struct {
	State  string `json:"state"`
	TxHash string `json:"transactionHash,omitempty"`
}

// pollTransactionStatus 轮询交易状态
func (r *Redeemer) pollTransactionStatus(ctx context.Context, txID string) (string, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	maxAttempts := 60 // 最多等待 3 分钟
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			attempts++
			if attempts > maxAttempts {
				return "", fmt.Errorf("轮询超时")
			}

			url := fmt.Sprintf("%s%s?id=%s", RelayerURL, TransactionPath, txID)
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				continue
			}

			resp, err := r.httpClient.Do(req)
			if err != nil {
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var status TransactionStatus
			if err := json.Unmarshal(body, &status); err != nil {
				continue
			}

			if status.State == StateConfirmed || status.State == StateMined {
				log.Printf("📦 [赎回] 交易已确认: %s", status.TxHash)
				return status.State, nil
			}

			if status.State == StateFailed {
				return StateFailed, nil
			}
		}
	}
}

// GetPendingCount 获取待处理的赎回数量
func (r *Redeemer) GetPendingCount() int {
	r.queueMu.RLock()
	defer r.queueMu.RUnlock()
	return len(r.pendingQueue)
}

// keccak256 helper
func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}
