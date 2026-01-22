package realtime

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// REST API 端点
const (
	CLOBBaseURL = "https://clob.polymarket.com"
)

// RESTClient 用于获取 REST 快照数据
type RESTClient struct {
	baseURL    string
	httpClient *http.Client
	logger     Logger
}

// OrderBookSnapshot 订单簿快照 (REST API 响应)
type OrderBookSnapshot struct {
	Market    string      `json:"market"`
	AssetID   string      `json:"asset_id"`
	Hash      string      `json:"hash"`
	Bids      []BookLevel `json:"bids"`
	Asks      []BookLevel `json:"asks"`
	Timestamp string      `json:"timestamp"`
}

// NewRESTClient 创建 REST 客户端
func NewRESTClient(logger Logger) *RESTClient {
	if logger == nil {
		logger = &defaultLogger{}
	}
	return &RESTClient{
		baseURL: CLOBBaseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// GetOrderBook 获取指定 token ID 的订单簿快照
// 端点: https://clob.polymarket.com/book?token_id=<token_id>
func (c *RESTClient) GetOrderBook(tokenID string) (*OrderBookSnapshot, error) {
	url := fmt.Sprintf("%s/book?token_id=%s", c.baseURL, tokenID)

	c.logger.Debug("获取订单簿快照: %s", url)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, string(body))
	}

	var snapshot OrderBookSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 确保 AssetID 被正确设置（REST API 可能不返回 asset_id 字段）
	if snapshot.AssetID == "" {
		snapshot.AssetID = tokenID
	}

	c.logger.Debug("获取订单簿成功: %d bids, %d asks (asset=%s)", len(snapshot.Bids), len(snapshot.Asks), snapshot.AssetID)
	return &snapshot, nil
}

// GetMidPrice 获取指定 token ID 的中间价
// 端点: https://clob.polymarket.com/midpoint?token_id=<token_id>
func (c *RESTClient) GetMidPrice(tokenID string) (string, error) {
	url := fmt.Sprintf("%s/midpoint?token_id=%s", c.baseURL, tokenID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Mid string `json:"mid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	return result.Mid, nil
}

// GetPrice 获取指定 token ID 的价格信息
// 端点: https://clob.polymarket.com/price?token_id=<token_id>&side=<BUY|SELL>
func (c *RESTClient) GetPrice(tokenID string, side string) (string, error) {
	url := fmt.Sprintf("%s/price?token_id=%s&side=%s", c.baseURL, tokenID, side)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	return result.Price, nil
}

// SnapshotValidator 订单簿快照校验器
type SnapshotValidator struct {
	restClient       *RESTClient
	logger           Logger
	validateInterval time.Duration
	lastValidation   map[string]time.Time // 每个 token 的最后校验时间
}

// NewSnapshotValidator 创建快照校验器
func NewSnapshotValidator(logger Logger, interval time.Duration) *SnapshotValidator {
	if interval == 0 {
		interval = 2 * time.Minute // 默认每 2 分钟校验一次
	}
	return &SnapshotValidator{
		restClient:       NewRESTClient(logger),
		logger:           logger,
		validateInterval: interval,
		lastValidation:   make(map[string]time.Time),
	}
}

// ShouldValidate 检查是否应该校验指定 token
func (v *SnapshotValidator) ShouldValidate(tokenID string) bool {
	lastTime, exists := v.lastValidation[tokenID]
	if !exists {
		return true
	}
	return time.Since(lastTime) >= v.validateInterval
}

// ValidateOrderBook 校验本地订单簿与 REST 快照
// 返回 REST 快照和是否有差异
func (v *SnapshotValidator) ValidateOrderBook(tokenID string, localBook *BookUpdate) (*OrderBookSnapshot, bool, error) {
	snapshot, err := v.restClient.GetOrderBook(tokenID)
	if err != nil {
		return nil, false, err
	}

	v.lastValidation[tokenID] = time.Now()

	// 使用 hash 快速比较
	if localBook != nil && localBook.Hash != "" && snapshot.Hash != "" {
		if localBook.Hash != snapshot.Hash {
			// 提取关键数据用于对比
			localBestBid := "N/A"
			localBestAsk := "N/A"
			remoteBestBid := "N/A"
			remoteBestAsk := "N/A"

			if len(localBook.Bids) > 0 {
				localBestBid = localBook.Bids[0].Price
			}
			if len(localBook.Asks) > 0 {
				localBestAsk = localBook.Asks[0].Price
			}
			if len(snapshot.Bids) > 0 {
				remoteBestBid = snapshot.Bids[0].Price
			}
			if len(snapshot.Asks) > 0 {
				remoteBestAsk = snapshot.Asks[0].Price
			}

			// 检查关键数据是否一致 (最佳买卖价)
			pricesMatch := false
			if localBestBid == remoteBestBid && localBestAsk == remoteBestAsk {
				pricesMatch = true
			}

			if pricesMatch {
				// 关键数据一致，仅作为常规同步 (静默，Info级别整合到 manager)
				// v.logger.Debug("ℹ️ 订单簿同步 (Hash差异但最佳价格一致): local=%s... remote=%s...",
				// 	shortHash(localBook.Hash), shortHash(snapshot.Hash))
			} else {
				// 关键数据不一致，发出警告
				v.logger.Warn("⚠️ 订单簿 Hash 不匹配且价格差异: local=%s, remote=%s", localBook.Hash, snapshot.Hash)
				v.logger.Warn("   📊 本地: %d bids (best=%s), %d asks (best=%s)",
					len(localBook.Bids), localBestBid, len(localBook.Asks), localBestAsk)
				v.logger.Warn("   📊 远程: %d bids (best=%s), %d asks (best=%s)",
					len(snapshot.Bids), remoteBestBid, len(snapshot.Asks), remoteBestAsk)
			}

			return snapshot, true, nil // 无论如何都更新，保证最新
		}
		v.logger.Debug("订单簿 Hash 完全匹配: %s", snapshot.Hash)
		return snapshot, false, nil // 无差异
	}

	// 简单比较 bid/ask 数量
	localBidCount := 0
	localAskCount := 0
	if localBook != nil {
		localBidCount = len(localBook.Bids)
		localAskCount = len(localBook.Asks)
	}

	if localBidCount != len(snapshot.Bids) || localAskCount != len(snapshot.Asks) {
		v.logger.Warn("⚠️ 订单簿数量不匹配: local=%d/%d, remote=%d/%d",
			localBidCount, localAskCount, len(snapshot.Bids), len(snapshot.Asks))
		return snapshot, true, nil // 有差异
	}

	v.logger.Debug("订单簿校验通过: %d bids, %d asks", len(snapshot.Bids), len(snapshot.Asks))
	return snapshot, false, nil
}

// shortHash 截取 hash 前几位用于显示
func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

// FetchInitialSnapshot 获取初始快照（用于市场切换时）
func (v *SnapshotValidator) FetchInitialSnapshot(tokenID string) (*OrderBookSnapshot, error) {
	snapshot, err := v.restClient.GetOrderBook(tokenID)
	if err != nil {
		return nil, err
	}
	v.lastValidation[tokenID] = time.Now()
	v.logger.Info("获取初始快照成功: %d bids, %d asks", len(snapshot.Bids), len(snapshot.Asks))
	return snapshot, nil
}

// Reset 重置校验状态（市场切换时调用）
func (v *SnapshotValidator) Reset() {
	v.lastValidation = make(map[string]time.Time)
}
