// Package realtime 提供 Polymarket CLOB WebSocket 实时数据流
//
// 基于 https://github.com/fipso/polymarkt-go 的实现，并添加了：
// - 指数退避自动重连
// - 心跳检测 (PING/PONG)
// - REST 快照校验支持
// - 每次市场切换新建连接策略
//
// 使用端点: wss://ws-subscriptions-clob.polymarket.com/ws/market
package realtime

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

// 默认配置
const (
	DefaultWSURL        = "wss://ws-subscriptions-clob.polymarket.com/ws/market"
	DefaultPingInterval = 10 * time.Second
	DefaultReadTimeout  = 30 * time.Second
	MaxReconnectBackoff = time.Minute
)

// StreamMessage 表示从 WebSocket 收到的消息
type StreamMessage struct {
	EventType    string            `json:"event_type"`
	Market       string            `json:"market,omitempty"`
	AssetID      string            `json:"asset_id,omitempty"`
	Timestamp    string            `json:"timestamp,omitempty"`
	Hash         string            `json:"hash,omitempty"`
	Price        string            `json:"price,omitempty"`
	Size         string            `json:"size,omitempty"`
	Side         string            `json:"side,omitempty"`
	FeeRateBps   string            `json:"fee_rate_bps,omitempty"`
	Bids         []BookLevel       `json:"bids,omitempty"`
	Asks         []BookLevel       `json:"asks,omitempty"`
	PriceChanges []PriceChangeItem `json:"price_changes,omitempty"`
}

// BookLevel 表示订单簿的一个价位
type BookLevel struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

// PriceChangeItem 表示 price_change 事件中的单个价格变动
type PriceChangeItem struct {
	AssetID string `json:"asset_id"`
	Price   string `json:"price"`
	Size    string `json:"size"`
	Side    string `json:"side"`
	Hash    string `json:"hash"`
	BestBid string `json:"best_bid"`
	BestAsk string `json:"best_ask"`
}

// PriceChange 表示解析后的价格变动事件
type PriceChange struct {
	Market    string          `json:"market"`
	AssetID   string          `json:"asset_id"`
	Price     decimal.Decimal `json:"price"`
	Side      string          `json:"side"` // "BUY" or "SELL"
	Size      decimal.Decimal `json:"size"`
	BestBid   decimal.Decimal `json:"best_bid"`
	BestAsk   decimal.Decimal `json:"best_ask"`
	Timestamp string          `json:"timestamp"`
}

// BookUpdate 表示订单簿更新
type BookUpdate struct {
	Market    string      `json:"market"`
	AssetID   string      `json:"asset_id"`
	Bids      []BookLevel `json:"bids"`
	Asks      []BookLevel `json:"asks"`
	Timestamp string      `json:"timestamp"`
	Hash      string      `json:"hash"`
}

// TradeEvent 表示交易事件 (last_trade_price)
type TradeEvent struct {
	Market     string          `json:"market"`
	AssetID    string          `json:"asset_id"`
	Side       string          `json:"side"`
	Size       decimal.Decimal `json:"size"`
	Price      decimal.Decimal `json:"price"`
	FeeRateBps string          `json:"fee_rate_bps"`
	Timestamp  string          `json:"timestamp"`
}

// Logger 定义日志接口
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// 默认日志记录器
type defaultLogger struct{}

func (l *defaultLogger) Debug(format string, args ...interface{}) {
	log.Printf("[DEBUG] "+format, args...)
}
func (l *defaultLogger) Info(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}
func (l *defaultLogger) Warn(format string, args ...interface{}) {
	log.Printf("[WARN] "+format, args...)
}
func (l *defaultLogger) Error(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

// Stream 处理 Polymarket WebSocket 连接
type Stream struct {
	url          string
	conn         *websocket.Conn
	mu           sync.RWMutex
	subscribers  map[string][]chan StreamMessage
	assetIDs     []string // 当前订阅的资产
	done         chan struct{}
	pingDone     chan struct{}
	reconnect    bool
	pingInterval time.Duration
	logger       Logger

	// 僵死连接检测
	lastMessageTime time.Time
	lastMessageMu   sync.RWMutex
	staleThreshold  time.Duration

	// 重连回调（用于触发 REST 快照校验）
	onReconnect func()
}

// StreamConfig 流配置
type StreamConfig struct {
	URL            string
	PingInterval   time.Duration
	StaleThreshold time.Duration // 超过此时间无消息视为僵死
	Logger         Logger
	OnReconnect    func() // 重连成功后的回调
}

// NewStream 创建新的 WebSocket 流
func NewStream(config StreamConfig) *Stream {
	url := config.URL
	if url == "" {
		url = DefaultWSURL
	}

	pingInterval := config.PingInterval
	if pingInterval == 0 {
		pingInterval = DefaultPingInterval
	}

	staleThreshold := config.StaleThreshold
	if staleThreshold == 0 {
		staleThreshold = 60 * time.Second
	}

	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	return &Stream{
		url:             url,
		subscribers:     make(map[string][]chan StreamMessage),
		done:            make(chan struct{}),
		reconnect:       true,
		pingInterval:    pingInterval,
		staleThreshold:  staleThreshold,
		logger:          logger,
		lastMessageTime: time.Now(),
		onReconnect:     config.OnReconnect,
	}
}

// Connect 建立 WebSocket 连接
func (s *Stream) Connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	s.logger.Info("正在连接到 %s", s.url)

	conn, _, err := dialer.Dial(s.url, nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	s.conn = conn
	s.pingDone = make(chan struct{})

	// 重置最后消息时间
	s.lastMessageMu.Lock()
	s.lastMessageTime = time.Now()
	s.lastMessageMu.Unlock()

	// 启动心跳
	go s.pingLoop()

	// 启动消息读取
	go s.readLoop()

	// 启动僵死检测
	go s.healthCheck()

	s.logger.Info("✅ WebSocket 连接成功")
	return nil
}

// Subscribe 订阅指定资产的市场更新
func (s *Stream) Subscribe(assetIDs []string, ch chan StreamMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return fmt.Errorf("not connected")
	}

	// 存储资产 ID 用于重订阅
	s.assetIDs = append(s.assetIDs, assetIDs...)

	// 为每个资产注册通道
	for _, id := range assetIDs {
		s.subscribers[id] = append(s.subscribers[id], ch)
	}

	// 发送 market 订阅 (price_change 事件)
	marketMsg := map[string]interface{}{
		"assets_ids": assetIDs,
		"type":       "market",
	}
	if err := s.conn.WriteJSON(marketMsg); err != nil {
		return fmt.Errorf("failed to send market subscription: %w", err)
	}

	// 发送 book 订阅 (订单簿更新)
	bookMsg := map[string]interface{}{
		"assets_ids": assetIDs,
		"type":       "book",
	}
	if err := s.conn.WriteJSON(bookMsg); err != nil {
		return fmt.Errorf("failed to send book subscription: %w", err)
	}

	s.logger.Info("已订阅 %d 个资产 (market + book)", len(assetIDs))
	return nil
}

// SubscribeAll 订阅所有消息 (通配符)
func (s *Stream) SubscribeAll(ch chan StreamMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers["*"] = append(s.subscribers["*"], ch)
}

// Unsubscribe 取消订阅
func (s *Stream) Unsubscribe(assetIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range assetIDs {
		delete(s.subscribers, id)
	}

	// 从 assetIDs 列表中移除
	newAssetIDs := make([]string, 0)
	for _, id := range s.assetIDs {
		found := false
		for _, removeID := range assetIDs {
			if id == removeID {
				found = true
				break
			}
		}
		if !found {
			newAssetIDs = append(newAssetIDs, id)
		}
	}
	s.assetIDs = newAssetIDs
}

// ClearSubscriptions 清除所有订阅
func (s *Stream) ClearSubscriptions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscribers = make(map[string][]chan StreamMessage)
	s.assetIDs = nil
}

// pingLoop 定期发送 PING 消息保持连接活跃
func (s *Stream) pingLoop() {
	ticker := time.NewTicker(s.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.pingDone:
			return
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.RLock()
			conn := s.conn
			s.mu.RUnlock()

			if conn != nil {
				// 使用大写 PING (polymarkt-go 格式)
				if err := conn.WriteMessage(websocket.TextMessage, []byte("PING")); err != nil {
					s.logger.Warn("发送 PING 失败: %v", err)
				}
			}
		}
	}
}

// healthCheck 检查连接是否僵死
func (s *Stream) healthCheck() {
	ticker := time.NewTicker(5 * time.Second) // 每 5 秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.lastMessageMu.RLock()
			lastTime := s.lastMessageTime
			s.lastMessageMu.RUnlock()

			if time.Since(lastTime) > s.staleThreshold {
				s.logger.Warn("⚠️ 连接僵死，%.0f秒未收到消息，触发重连", time.Since(lastTime).Seconds())

				// 关闭当前连接触发重连
				s.mu.Lock()
				if s.conn != nil {
					s.conn.Close()
				}
				s.mu.Unlock()
				return
			}
		}
	}
}

// readLoop 持续从 WebSocket 读取消息
func (s *Stream) readLoop() {
	for {
		select {
		case <-s.done:
			return
		default:
		}

		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		if conn == nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			s.logger.Warn("WebSocket 读取错误: %v", err)
			if s.reconnect {
				s.handleReconnect()
			}
			return
		}

		// 更新最后消息时间
		s.lastMessageMu.Lock()
		s.lastMessageTime = time.Now()
		s.lastMessageMu.Unlock()

		// 处理 PONG 响应
		if string(data) == "PONG" {
			continue
		}

		// 尝试解析为单条消息
		var msg StreamMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			// 尝试解析为消息数组 (批量)
			var msgs []StreamMessage
			if err2 := json.Unmarshal(data, &msgs); err2 != nil {
				// 两种都失败，记录错误
				preview := string(data)
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				s.logger.Debug("无法解析消息: %s", preview)
				continue
			}
			// 分发批量消息
			for _, m := range msgs {
				s.dispatch(m)
			}
			continue
		}

		// 分发单条消息
		s.dispatch(msg)
	}
}

// dispatch 将消息发送给所有相关订阅者
func (s *Stream) dispatch(msg StreamMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 发送给资产特定的订阅者
	if msg.AssetID != "" {
		for _, ch := range s.subscribers[msg.AssetID] {
			select {
			case ch <- msg:
			default:
				// 通道满，跳过
			}
		}
	}

	// 对于 price_change 事件，分发给每个资产
	for _, pc := range msg.PriceChanges {
		for _, ch := range s.subscribers[pc.AssetID] {
			select {
			case ch <- msg:
			default:
			}
		}
	}

	// 发送给市场订阅者
	if msg.Market != "" {
		for _, ch := range s.subscribers[msg.Market] {
			select {
			case ch <- msg:
			default:
			}
		}
	}

	// 发送给通配符订阅者
	for _, ch := range s.subscribers["*"] {
		select {
		case ch <- msg:
		default:
		}
	}
}

// handleReconnect 使用指数退避尝试重连
func (s *Stream) handleReconnect() {
	// 停止旧连接的 ping 循环
	if s.pingDone != nil {
		select {
		case <-s.pingDone:
			// 已关闭
		default:
			close(s.pingDone)
		}
	}

	backoff := time.Second
	maxBackoff := MaxReconnectBackoff

	for {
		select {
		case <-s.done:
			return
		default:
		}

		s.logger.Info("🔄 %v 后尝试重连...", backoff)
		time.Sleep(backoff)

		// 1. 先调用重连回调拉取 REST 快照（阻塞等待完成）
		if s.onReconnect != nil {
			s.logger.Info("📥 重连中: 先拉取 REST 快照...")
			s.onReconnect() // 同步调用，确保快照拉取完成
		}

		// 2. 建立连接
		if err := s.Connect(); err != nil {
			s.logger.Error("重连失败: %v", err)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// 3. 重新订阅所有资产
		s.resubscribe()
		s.logger.Info("✅ 重连成功 (快照→连接→订阅)")
		return
	}
}

// resubscribe 重新订阅所有之前订阅的资产
func (s *Stream) resubscribe() {
	s.mu.RLock()
	assetIDs := make([]string, len(s.assetIDs))
	copy(assetIDs, s.assetIDs)
	s.mu.RUnlock()

	if len(assetIDs) > 0 {
		// 重新订阅 market (price_change)
		marketMsg := map[string]interface{}{
			"assets_ids": assetIDs,
			"type":       "market",
		}
		s.mu.Lock()
		if s.conn != nil {
			s.conn.WriteJSON(marketMsg)
		}
		s.mu.Unlock()

		// 重新订阅 book
		bookMsg := map[string]interface{}{
			"assets_ids": assetIDs,
			"type":       "book",
		}
		s.mu.Lock()
		if s.conn != nil {
			s.conn.WriteJSON(bookMsg)
		}
		s.mu.Unlock()

		s.logger.Info("已重新订阅 %d 个资产 (market + book)", len(assetIDs))
	}
}

// Close 关闭 WebSocket 连接
func (s *Stream) Close() error {
	s.reconnect = false

	// 只关闭一次 done
	select {
	case <-s.done:
		// 已关闭
	default:
		close(s.done)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}

// IsConnected 检查是否已连接
func (s *Stream) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conn != nil
}

// GetSubscribedAssets 获取当前订阅的资产列表
func (s *Stream) GetSubscribedAssets() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.assetIDs))
	copy(result, s.assetIDs)
	return result
}
