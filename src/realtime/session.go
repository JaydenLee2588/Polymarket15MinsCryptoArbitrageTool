package realtime

import (
	"sync"
	"time"
)

// MarketSession 表示一个市场会话
// 每次切换市场时创建新的会话，避免复用旧连接
type MarketSession struct {
	stream      *Stream
	assetIDs    []string
	messageChan chan StreamMessage
	done        chan struct{}
	logger      Logger
	mu          sync.RWMutex
	startTime   time.Time
}

// MarketSessionConfig 会话配置
type MarketSessionConfig struct {
	AssetIDs       []string
	ChannelBuffer  int           // 消息通道缓冲大小
	PingInterval   time.Duration // 心跳间隔
	StaleThreshold time.Duration // 僵死检测阈值
	Logger         Logger
}

// NewMarketSession 创建新的市场会话
// 推荐：每次切换市场时新建会话，而不是复用旧连接
func NewMarketSession(config MarketSessionConfig) *MarketSession {
	bufferSize := config.ChannelBuffer
	if bufferSize == 0 {
		bufferSize = 100
	}

	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	stream := NewStream(StreamConfig{
		PingInterval:   config.PingInterval,
		StaleThreshold: config.StaleThreshold,
		Logger:         logger,
	})

	return &MarketSession{
		stream:      stream,
		assetIDs:    config.AssetIDs,
		messageChan: make(chan StreamMessage, bufferSize),
		done:        make(chan struct{}),
		logger:      logger,
		startTime:   time.Now(),
	}
}

// Start 启动会话：连接 -> 订阅 -> 开始接收
func (s *MarketSession) Start() error {
	// 1. 建立连接
	if err := s.stream.Connect(); err != nil {
		return err
	}

	// 2. 订阅资产
	if err := s.stream.Subscribe(s.assetIDs, s.messageChan); err != nil {
		s.stream.Close()
		return err
	}

	s.logger.Info("📊 市场会话已启动，订阅 %d 个资产", len(s.assetIDs))
	return nil
}

// Stop 停止会话
func (s *MarketSession) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 关闭 done 通道
	select {
	case <-s.done:
		// 已关闭
	default:
		close(s.done)
	}

	// 关闭流
	if s.stream != nil {
		s.stream.Close()
	}

	s.logger.Info("📊 市场会话已停止，运行时长: %v", time.Since(s.startTime))
}

// Messages 返回消息通道
func (s *MarketSession) Messages() <-chan StreamMessage {
	return s.messageChan
}

// Done 返回完成通道
func (s *MarketSession) Done() <-chan struct{} {
	return s.done
}

// IsRunning 检查会话是否正在运行
func (s *MarketSession) IsRunning() bool {
	select {
	case <-s.done:
		return false
	default:
		return s.stream.IsConnected()
	}
}

// GetAssetIDs 获取订阅的资产列表
func (s *MarketSession) GetAssetIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.assetIDs))
	copy(result, s.assetIDs)
	return result
}

// GetStartTime 获取会话开始时间
func (s *MarketSession) GetStartTime() time.Time {
	return s.startTime
}

// GetDuration 获取会话运行时长
func (s *MarketSession) GetDuration() time.Duration {
	return time.Since(s.startTime)
}

// SessionManager 管理市场会话的切换
// 支持每 15 分钟切换市场的策略
type SessionManager struct {
	currentSession *MarketSession
	mu             sync.RWMutex
	logger         Logger

	// 回调
	onSessionStart func(*MarketSession)
	onSessionStop  func(*MarketSession)
	onMessage      func(StreamMessage)
}

// SessionManagerConfig 会话管理器配置
type SessionManagerConfig struct {
	Logger         Logger
	OnSessionStart func(*MarketSession)
	OnSessionStop  func(*MarketSession)
	OnMessage      func(StreamMessage)
}

// NewSessionManager 创建会话管理器
func NewSessionManager(config SessionManagerConfig) *SessionManager {
	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	return &SessionManager{
		logger:         logger,
		onSessionStart: config.OnSessionStart,
		onSessionStop:  config.OnSessionStop,
		onMessage:      config.OnMessage,
	}
}

// SwitchMarket 切换到新市场
// 会关闭旧连接，建立新连接，订阅新市场
func (m *SessionManager) SwitchMarket(assetIDs []string, config MarketSessionConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 停止旧会话
	if m.currentSession != nil {
		m.logger.Info("🔄 正在关闭旧市场会话...")
		m.currentSession.Stop()
		if m.onSessionStop != nil {
			m.onSessionStop(m.currentSession)
		}
	}

	// 2. 创建新会话
	config.AssetIDs = assetIDs
	if config.Logger == nil {
		config.Logger = m.logger
	}

	newSession := NewMarketSession(config)

	// 3. 启动新会话
	if err := newSession.Start(); err != nil {
		m.logger.Error("启动新市场会话失败: %v", err)
		return err
	}

	m.currentSession = newSession

	// 4. 回调
	if m.onSessionStart != nil {
		m.onSessionStart(newSession)
	}

	// 5. 启动消息转发
	if m.onMessage != nil {
		go m.forwardMessages(newSession)
	}

	m.logger.Info("✅ 已切换到新市场，订阅 %d 个资产", len(assetIDs))
	return nil
}

// forwardMessages 转发消息到回调
func (m *SessionManager) forwardMessages(session *MarketSession) {
	for {
		select {
		case <-session.Done():
			return
		case msg := <-session.Messages():
			if m.onMessage != nil {
				m.onMessage(msg)
			}
		}
	}
}

// GetCurrentSession 获取当前会话
func (m *SessionManager) GetCurrentSession() *MarketSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentSession
}

// Stop 停止管理器和当前会话
func (m *SessionManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentSession != nil {
		m.currentSession.Stop()
		if m.onSessionStop != nil {
			m.onSessionStop(m.currentSession)
		}
		m.currentSession = nil
	}
}
