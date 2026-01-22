package realtime

import (
	"sync"
	"time"
)

// MarketManager 完整的市场管理器
// 实现用户要求的所有稳定性功能：
// 1. 断线重连机制（指数退避）
// 2. 心跳检测
// 3. REST 快照辅助校验
// 4. 市场切换策略（每次新建连接）
type MarketManager struct {
	// 当前会话
	currentStream *Stream

	// 配置
	assetIDs         []string
	pingInterval     time.Duration
	staleThreshold   time.Duration
	snapshotInterval time.Duration // REST 快照校验间隔

	// 组件
	validator *SnapshotValidator
	logger    Logger

	// 本地订单簿缓存
	orderBooks   map[string]*BookUpdate
	orderBooksMu sync.RWMutex

	// 消息通道
	messageChan chan StreamMessage

	// 回调
	onPriceChange func(PriceChange)
	onBookUpdate  func(*BookUpdate)
	onTrade       func(*TradeEvent)
	onError       func(error)
	onReconnect   func()

	// 控制
	done    chan struct{}
	mu      sync.RWMutex
	running bool
}

// MarketManagerConfig 市场管理器配置
type MarketManagerConfig struct {
	// 基础配置
	AssetIDs         []string      // 要订阅的资产 ID
	PingInterval     time.Duration // 心跳间隔 (默认 10s)
	StaleThreshold   time.Duration // 僵死检测阈值 (默认 60s)
	SnapshotInterval time.Duration // REST 快照校验间隔 (默认 2min)
	ChannelBuffer    int           // 消息通道缓冲大小
	Logger           Logger

	// 回调
	OnPriceChange func(PriceChange)
	OnBookUpdate  func(*BookUpdate)
	OnTrade       func(*TradeEvent)
	OnError       func(error)
	OnReconnect   func()
}

// NewMarketManager 创建市场管理器
func NewMarketManager(config MarketManagerConfig) *MarketManager {
	pingInterval := config.PingInterval
	if pingInterval == 0 {
		pingInterval = 10 * time.Second
	}

	staleThreshold := config.StaleThreshold
	if staleThreshold == 0 {
		staleThreshold = 60 * time.Second
	}

	snapshotInterval := config.SnapshotInterval
	if snapshotInterval == 0 {
		snapshotInterval = 2 * time.Minute
	}

	bufferSize := config.ChannelBuffer
	if bufferSize == 0 {
		bufferSize = 100
	}

	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	return &MarketManager{
		assetIDs:         config.AssetIDs,
		pingInterval:     pingInterval,
		staleThreshold:   staleThreshold,
		snapshotInterval: snapshotInterval,
		validator:        NewSnapshotValidator(logger, snapshotInterval),
		logger:           logger,
		orderBooks:       make(map[string]*BookUpdate),
		messageChan:      make(chan StreamMessage, bufferSize),
		onPriceChange:    config.OnPriceChange,
		onBookUpdate:     config.OnBookUpdate,
		onTrade:          config.OnTrade,
		onError:          config.OnError,
		onReconnect:      config.OnReconnect,
		done:             make(chan struct{}),
	}
}

// Start 启动市场管理器
// 流程：
// 1. 获取 REST 快照初始化订单簿
// 2. 建立 WS 连接
// 3. 订阅市场数据
// 4. 启动消息处理和定期校验
func (m *MarketManager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	m.logger.Info("📊 启动市场管理器，订阅 %d 个资产", len(m.assetIDs))

	// 1. 获取初始 REST 快照
	for _, assetID := range m.assetIDs {
		snapshot, err := m.validator.FetchInitialSnapshot(assetID)
		if err != nil {
			m.logger.Warn("获取初始快照失败 (%s): %v", assetID, err)
		} else {
			m.orderBooksMu.Lock()
			m.orderBooks[assetID] = &BookUpdate{
				AssetID:   snapshot.AssetID,
				Market:    snapshot.Market,
				Bids:      snapshot.Bids,
				Asks:      snapshot.Asks,
				Hash:      snapshot.Hash,
				Timestamp: snapshot.Timestamp,
			}
			m.orderBooksMu.Unlock()
		}
	}

	// 2. 建立 WS 连接
	if err := m.connect(); err != nil {
		return err
	}

	// 3. 启动消息处理
	go m.processMessages()

	// 4. 启动定期 REST 校验
	go m.periodicValidation()

	return nil
}

// connect 建立连接并订阅
func (m *MarketManager) connect() error {
	stream := NewStream(StreamConfig{
		PingInterval:   m.pingInterval,
		StaleThreshold: m.staleThreshold,
		Logger:         m.logger,
		OnReconnect:    m.fetchAllSnapshots, // 重连时拉取 REST 快照
	})

	if err := stream.Connect(); err != nil {
		return err
	}

	// 订阅资产
	if err := stream.Subscribe(m.assetIDs, m.messageChan); err != nil {
		stream.Close()
		return err
	}

	m.mu.Lock()
	m.currentStream = stream
	m.mu.Unlock()

	return nil
}

// SwitchMarket 切换到新市场
// 策略：关闭旧连接 → 获取新快照 → 建立新连接 → 订阅新市场
func (m *MarketManager) SwitchMarket(newAssetIDs []string) error {
	m.logger.Info("🔄 切换市场: %d → %d 个资产", len(m.assetIDs), len(newAssetIDs))

	// 1. 关闭旧连接
	m.mu.Lock()
	if m.currentStream != nil {
		m.currentStream.Close()
		m.currentStream = nil
	}
	m.assetIDs = newAssetIDs
	m.mu.Unlock()

	// 2. 清空旧订单簿和校验状态
	m.orderBooksMu.Lock()
	m.orderBooks = make(map[string]*BookUpdate)
	m.orderBooksMu.Unlock()
	m.validator.Reset()

	// 3. 获取新市场的 REST 快照
	for _, assetID := range newAssetIDs {
		snapshot, err := m.validator.FetchInitialSnapshot(assetID)
		if err != nil {
			m.logger.Warn("获取初始快照失败 (%s): %v", assetID, err)
		} else {
			m.orderBooksMu.Lock()
			m.orderBooks[assetID] = &BookUpdate{
				AssetID:   snapshot.AssetID,
				Market:    snapshot.Market,
				Bids:      snapshot.Bids,
				Asks:      snapshot.Asks,
				Hash:      snapshot.Hash,
				Timestamp: snapshot.Timestamp,
			}
			m.orderBooksMu.Unlock()

			// 触发回调
			if m.onBookUpdate != nil {
				m.onBookUpdate(m.orderBooks[assetID])
			}
		}
	}

	// 4. 建立新连接
	if err := m.connect(); err != nil {
		return err
	}

	m.logger.Info("✅ 市场切换完成")
	return nil
}

// processMessages 处理消息
func (m *MarketManager) processMessages() {
	for {
		select {
		case <-m.done:
			return
		case msg := <-m.messageChan:
			m.handleMessage(msg)
		}
	}
}

// handleMessage 处理单条消息
func (m *MarketManager) handleMessage(msg StreamMessage) {
	switch msg.EventType {
	case "book":
		// 订单簿快照
		book := ParseBookUpdate(msg)
		m.logger.Debug("📚 收到 book 事件: asset=%s, %d bids, %d asks, hash=%s",
			book.AssetID, len(book.Bids), len(book.Asks), book.Hash)
		m.orderBooksMu.Lock()
		m.orderBooks[book.AssetID] = book
		m.orderBooksMu.Unlock()

		if m.onBookUpdate != nil {
			m.onBookUpdate(book)
		}

	case "price_change":
		// 价格变动
		for _, item := range msg.PriceChanges {
			pc, err := ParsePriceChange(msg, item)
			if err != nil {
				m.logger.Warn("解析 price_change 失败: %v", err)
				continue
			}

			// 更新本地订单簿（简化处理，实际应该应用增量更新）
			m.orderBooksMu.Lock()
			if book, exists := m.orderBooks[pc.AssetID]; exists {
				book.Timestamp = msg.Timestamp
			}
			m.orderBooksMu.Unlock()

			if m.onPriceChange != nil {
				m.onPriceChange(*pc)
			}
		}

	case "last_trade_price":
		// 最新成交
		trade, err := ParseTradeEvent(msg)
		if err != nil {
			m.logger.Warn("解析 last_trade_price 失败: %v", err)
			return
		}

		if m.onTrade != nil {
			m.onTrade(trade)
		}
	}
}

// periodicValidation 定期 REST 校验
func (m *MarketManager) periodicValidation() {
	ticker := time.NewTicker(m.snapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.validateAllBooks()
		}
	}
}

// validateAllBooks 校验所有订单簿
func (m *MarketManager) validateAllBooks() {
	m.mu.RLock()
	assetIDs := make([]string, len(m.assetIDs))
	copy(assetIDs, m.assetIDs)
	m.mu.RUnlock()

	diffCount := 0
	for _, assetID := range assetIDs {
		if !m.validator.ShouldValidate(assetID) {
			continue
		}

		m.orderBooksMu.RLock()
		localBook := m.orderBooks[assetID]
		m.orderBooksMu.RUnlock()

		snapshot, hasDiff, err := m.validator.ValidateOrderBook(assetID, localBook)
		if err != nil {
			m.logger.Warn("校验订单簿失败 (%s): %v", assetID, err)
			continue
		}

		if hasDiff {
			diffCount++
			// m.logger.Info("🔄 订单簿有差异，使用 REST 快照覆盖: %s", assetID) // 移除单条日志
			m.orderBooksMu.Lock()
			m.orderBooks[assetID] = &BookUpdate{
				AssetID:   snapshot.AssetID,
				Market:    snapshot.Market,
				Bids:      snapshot.Bids,
				Asks:      snapshot.Asks,
				Hash:      snapshot.Hash,
				Timestamp: snapshot.Timestamp,
			}
			m.orderBooksMu.Unlock()

			if m.onBookUpdate != nil {
				m.onBookUpdate(m.orderBooks[assetID])
			}
		}
	}

	if diffCount > 0 {
		m.logger.Info("✅ 和 REST 快照进行订单簿同步成功 (更新了 %d 个资产)", diffCount)
	}
}

// GetOrderBook 获取指定资产的本地订单簿
func (m *MarketManager) GetOrderBook(assetID string) *BookUpdate {
	m.orderBooksMu.RLock()
	defer m.orderBooksMu.RUnlock()

	if book, exists := m.orderBooks[assetID]; exists {
		// 返回副本
		return &BookUpdate{
			Market:    book.Market,
			AssetID:   book.AssetID,
			Bids:      book.Bids,
			Asks:      book.Asks,
			Timestamp: book.Timestamp,
			Hash:      book.Hash,
		}
	}
	return nil
}

// GetAllOrderBooks 获取所有本地订单簿
func (m *MarketManager) GetAllOrderBooks() map[string]*BookUpdate {
	m.orderBooksMu.RLock()
	defer m.orderBooksMu.RUnlock()

	result := make(map[string]*BookUpdate)
	for k, v := range m.orderBooks {
		result[k] = &BookUpdate{
			Market:    v.Market,
			AssetID:   v.AssetID,
			Bids:      v.Bids,
			Asks:      v.Asks,
			Timestamp: v.Timestamp,
			Hash:      v.Hash,
		}
	}
	return result
}

// SetAssetIDs 设置资产 ID 列表 (在 Start 之前调用)
func (m *MarketManager) SetAssetIDs(assetIDs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assetIDs = assetIDs
}

// IsRunning 检查是否正在运行
func (m *MarketManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// Stop 停止市场管理器
func (m *MarketManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false

	// 关闭 done 通道
	select {
	case <-m.done:
	default:
		close(m.done)
	}

	// 关闭当前流
	if m.currentStream != nil {
		m.currentStream.Close()
		m.currentStream = nil
	}

	m.logger.Info("📊 市场管理器已停止")
}

// fetchAllSnapshots 获取所有订阅资产的 REST 快照
// 用于重连时确保订单簿状态正确
func (m *MarketManager) fetchAllSnapshots() {
	m.mu.RLock()
	assetIDs := make([]string, len(m.assetIDs))
	copy(assetIDs, m.assetIDs)
	m.mu.RUnlock()

	m.logger.Info("📥 开始拉取 %d 个资产的 REST 快照...", len(assetIDs))

	for _, assetID := range assetIDs {
		snapshot, err := m.validator.FetchInitialSnapshot(assetID)
		if err != nil {
			m.logger.Warn("获取快照失败 (%s): %v", assetID, err)
			continue
		}

		m.orderBooksMu.Lock()
		m.orderBooks[assetID] = &BookUpdate{
			AssetID:   snapshot.AssetID,
			Market:    snapshot.Market,
			Bids:      snapshot.Bids,
			Asks:      snapshot.Asks,
			Hash:      snapshot.Hash,
			Timestamp: snapshot.Timestamp,
		}
		m.orderBooksMu.Unlock()
	}

	m.logger.Info("✅ REST 快照拉取完成")
}
