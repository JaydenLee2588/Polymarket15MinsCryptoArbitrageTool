package src

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// writeRequest 代表写入消息到 WebSocket 的请求
type writeRequest struct {
	messageType int
	data        []byte
}

// baseClient 包含所有客户端类型共享的通用 WebSocket 基础设施
type baseClient struct {
	// 协议特定行为
	protocol Protocol

	// WebSocket 连接
	conn   *websocket.Conn
	connMu sync.RWMutex

	// 配置
	host                 string
	pingInterval         time.Duration
	autoReconnect        bool
	maxReconnectAttempts int
	reconnectBackoffInit time.Duration
	reconnectBackoffMax  time.Duration
	readTimeout          time.Duration
	writeTimeout         time.Duration
	proxyURL             *url.URL // 可选的 HTTP/HTTPS 代理 URL

	// 回调
	onConnectCallback    func()
	onNewMessage         func([]byte)
	onDisconnectCallback func(error)
	onReconnectCallback  func()

	// 日志记录器
	logger Logger

	// 内部状态
	internal struct {
		mu                sync.RWMutex
		connClosed        bool
		subscriptions     []Subscription
		reconnectAttempts int
		isReconnecting    bool
	}

	// 控制流通道
	closeChan chan struct{}
	closeOnce sync.Once

	writeLoopOnce sync.Once
	writeChan     chan writeRequest // 序列化 WebSocket 写入的通道
}

// newBaseClient 使用给定的协议和选项创建一个新的基础客户端
func newBaseClient(protocol Protocol, opts ...ClientOption) *baseClient {
	// 创建带有默认值的配置
	config := &Config{
		Host:                 protocol.GetDefaultHost(),
		Logger:               NewSilentLogger(),
		PingInterval:         defaultPingInterval,
		AutoReconnect:        defaultAutoReconnect,
		MaxReconnectAttempts: defaultMaxReconnectAttempts,
		ReconnectBackoffInit: defaultReconnectBackoffInit,
		ReconnectBackoffMax:  defaultReconnectBackoffMax,
		ReadTimeout:          defaultReadTimeout,
		WriteTimeout:         defaultWriteTimeout,
	}

	// 应用用户选项到配置
	for _, opt := range opts {
		opt(config)
	}

	// 使用配置创建基础客户端
	c := &baseClient{
		protocol:             protocol,
		closeChan:            make(chan struct{}),
		writeChan:            make(chan writeRequest, 100), // 写入缓冲通道
		host:                 config.Host,
		logger:               config.Logger,
		pingInterval:         config.PingInterval,
		autoReconnect:        config.AutoReconnect,
		maxReconnectAttempts: config.MaxReconnectAttempts,
		reconnectBackoffInit: config.ReconnectBackoffInit,
		reconnectBackoffMax:  config.ReconnectBackoffMax,
		readTimeout:          config.ReadTimeout,
		writeTimeout:         config.WriteTimeout,
		proxyURL:             config.ProxyURL,

		onConnectCallback:    config.OnConnectCallback,
		onNewMessage:         config.OnNewMessage,
		onDisconnectCallback: config.OnDisconnectCallback,
		onReconnectCallback:  config.OnReconnectCallback,
	}

	return c
}

// connect 建立 WebSocket 连接
func (c *baseClient) connect() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	// 检查由于是否已连接
	if c.conn != nil {
		c.internal.mu.RLock()
		isClosed := c.internal.connClosed
		c.internal.mu.RUnlock()

		if !isClosed {
			return errors.New("已连接")
		}
	}

	// 检查是否正在重连 - 如果不是，这必须是一个新的连接
	c.internal.mu.RLock()
	isReconnecting := c.internal.isReconnecting
	c.internal.mu.RUnlock()

	// 如果没有重连且连接之前已关闭，则拒绝
	if !isReconnecting {
		c.internal.mu.RLock()
		wasClosed := c.internal.connClosed
		c.internal.mu.RUnlock()

		if wasClosed && c.conn != nil {
			return errors.New("连接已关闭，请创建新客户端")
		}
	}

	c.logger.Debug("正在连接到 %s (%s)", c.host, c.protocol.GetProtocolName())

	// 建立 WebSocket 连接
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// 配置代理（如果提供）
	if c.proxyURL != nil {
		dialer.Proxy = http.ProxyURL(c.proxyURL)
		c.logger.Debug("使用代理: %s", c.proxyURL.String())
	}

	// 添加必要的 Header 以绕过 Cloudflare 或服务器限制
	headers := http.Header{}
	headers.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	headers.Add("Origin", "https://polymarket.com")

	conn, _, err := dialer.Dial(c.host, headers)
	if err != nil {
		return err
	}

	c.conn = conn

	// 重置连接状态
	c.internal.mu.Lock()
	c.internal.connClosed = false
	c.internal.mu.Unlock()

	// 启动 goroutine
	go c.pingLoop()
	go c.readMessages()
	go c.writeLoop()

	// 调用连接回调
	if c.onConnectCallback != nil {
		c.onConnectCallback()
	}

	c.logger.Info("成功连接到 %s", c.protocol.GetProtocolName())

	return nil
}

// disconnect 关闭 WebSocket 连接
func (c *baseClient) disconnect() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	// 显式断开连接时禁用自动重连
	c.internal.mu.Lock()
	c.autoReconnect = false
	c.internal.mu.Unlock()

	if c.conn == nil {
		return errors.New("未连接")
	}

	c.internal.mu.RLock()
	isClosed := c.internal.connClosed
	c.internal.mu.RUnlock()

	if isClosed {
		return errors.New("连接已关闭")
	}

	c.logger.Debug("正在断开与 %s 的连接", c.protocol.GetProtocolName())

	// 关闭连接
	c.closeOnce.Do(func() {
		close(c.closeChan)
	})

	// 标记为已关闭
	c.internal.mu.Lock()
	c.internal.connClosed = true
	c.internal.mu.Unlock()

	// 发送关闭消息
	c.writeChan <- writeRequest{websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")}

	// 关闭连接
	err := c.conn.Close()
	if err != nil {
		return err
	}

	c.logger.Info("成功断开与 %s 的连接", c.protocol.GetProtocolName())

	return nil
}

// subscribe 向服务器发送订阅请求
func (c *baseClient) subscribe(subscriptions []Subscription) error {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	if c.conn == nil {
		return errors.New("未连接")
	}

	c.internal.mu.RLock()
	isClosed := c.internal.connClosed
	c.internal.mu.RUnlock()

	if isClosed {
		return errors.New("连接已关闭")
	}

	// 添加新订阅到存储列表 (带去重)
	c.internal.mu.Lock()

	// 创建现有订阅的映射以快速查找
	existing := make(map[string]bool)
	for _, sub := range c.internal.subscriptions {
		key := string(sub.Topic) + "|" + string(sub.Type) + "|" + sub.Filters
		existing[key] = true
	}

	// 仅添加不存在的订阅
	newSubs := []Subscription{}
	for _, sub := range subscriptions {
		key := string(sub.Topic) + "|" + string(sub.Type) + "|" + sub.Filters
		if !existing[key] {
			c.internal.subscriptions = append(c.internal.subscriptions, sub)
			newSubs = append(newSubs, sub)
			existing[key] = true
		}
	}

	totalCount := len(c.internal.subscriptions)
	c.internal.mu.Unlock()

	// 逐个发送订阅
	// 注意: 某些主题 (如 crypto_prices, crypto_prices_chainlink, equity_prices) 仅支持
	// 每个连接一个代码。对于这些主题，最后的订阅将替换之前的订阅。
	// 如果你需要这些主题的多个代码，请创建单独的客户端连接。
	//
	// 其他主题 (如 clob_market, activity, comments) 可能支持多个订阅。
	for _, sub := range newSubs {
		// 为单个订阅格式化订阅消息
		message, err := c.protocol.FormatSubscription([]Subscription{sub})
		if err != nil {
			return err
		}

		c.logger.Debug("发送订阅消息: %s", string(message))

		// 通过写入通道发送订阅
		select {
		case c.writeChan <- writeRequest{messageType: websocket.TextMessage, data: message}:
			// 成功
		case <-time.After(5 * time.Second):
			return errors.New("发送订阅消息超时")
		}
	}

	c.logger.Info("已订阅 %d 个新频道 (共 %d 个)", len(newSubs), totalCount)
	return nil
}

// unsubscribe 向服务器发送取消订阅请求
func (c *baseClient) unsubscribe(subscriptions []Subscription) error {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	if c.conn == nil {
		return errors.New("未连接")
	}

	c.internal.mu.RLock()
	isClosed := c.internal.connClosed
	c.internal.mu.RUnlock()

	if isClosed {
		return errors.New("连接已关闭")
	}

	// 首先为要移除的订阅发送取消订阅消息
	unsubMessage, err := c.protocol.FormatSubscription(subscriptions)
	if err != nil {
		return err
	}

	// 将消息中的 "subscribe" 替换为 "unsubscribe"
	messageStr := string(unsubMessage)
	if strings.Contains(messageStr, `"action":"subscribe"`) {
		messageStr = strings.Replace(messageStr, `"action":"subscribe"`, `"action":"unsubscribe"`, 1)
	} else if strings.Contains(messageStr, `"type":"MARKET"`) {
		// CLOB 协议使用不同的格式
		messageStr = strings.Replace(messageStr, `"type":"MARKET"`, `"type":"UNSUBSCRIBE"`, 1)
	}
	unsubMessage = []byte(messageStr)

	c.logger.Debug("发送取消订阅消息: %s", string(unsubMessage))

	// 通过写入通道发送取消订阅
	select {
	case c.writeChan <- writeRequest{messageType: websocket.TextMessage, data: unsubMessage}:
		c.logger.Debug("取消订阅消息已发送")
	case <-time.After(5 * time.Second):
		return errors.New("发送取消订阅消息超时")
	}

	// 从存储列表中移除订阅
	c.internal.mu.Lock()
	// 创建快速查找映射
	toRemove := make(map[string]bool)
	for _, sub := range subscriptions {
		key := string(sub.Topic) + "|" + string(sub.Type) + "|" + sub.Filters
		toRemove[key] = true
	}

	// 过滤要移除的订阅
	newSubs := make([]Subscription, 0)
	for _, sub := range c.internal.subscriptions {
		key := string(sub.Topic) + "|" + string(sub.Type) + "|" + sub.Filters
		if !toRemove[key] {
			newSubs = append(newSubs, sub)
		}
	}
	c.internal.subscriptions = newSubs
	c.internal.mu.Unlock()

	c.logger.Info("已取消订阅 %d 个频道 (剩余 %d 个)", len(subscriptions), len(newSubs))
	return nil
}

// pingLoop 定期发送 pingLoop 消息以保持连接活跃
func (c *baseClient) pingLoop() {
	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.closeChan:
			c.logger.Debug("Ping 例程已停止")
			return
		case <-ticker.C:
			c.connMu.RLock()
			conn := c.conn
			c.connMu.RUnlock()

			if conn == nil {
				continue
			}

			c.internal.mu.RLock()
			isClosed := c.internal.connClosed
			c.internal.mu.RUnlock()

			if isClosed {
				return
			}
			c.ping()
		}
	}
}

func (c *baseClient) ping() {
	// 尝试所有这些以检查连接是否会异常关闭。
	c.writeChan <- writeRequest{websocket.TextMessage, []byte("ping")}
	c.writeChan <- writeRequest{websocket.PingMessage, []byte("ping")}
	c.writeChan <- writeRequest{websocket.PingMessage, nil}

	// c.writeChan <- writeRequest{websocket.PongMessage, []byte("pong")}

	// 服务器端不支持的数据
	// c.writeChan <- writeRequest{websocket.BinaryMessage, []byte("pong")}

	// c.logger.Debug("Ping sent")
}

// writeLoop 通过通道序列化所有 WebSocket 写入
func (c *baseClient) writeLoop() {
	c.writeLoopOnce.Do(func() {
		defer func() {
			c.logger.Debug("写入循环已停止")
		}()

		for {
			select {
			case <-c.closeChan:
				return
			case req := <-c.writeChan:
				c.connMu.RLock()
				conn := c.conn
				c.connMu.RUnlock()

				if conn == nil {
					c.logger.Debug("跳过写入，连接为 nil")
					continue
				}

				c.internal.mu.RLock()
				isClosed := c.internal.connClosed
				c.internal.mu.RUnlock()

				if isClosed {
					c.logger.Debug("跳过写入，连接已关闭")
					continue
				}

				c.logger.Debug("写入消息 %v: %v", req.messageType, string(req.data))

				conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
				err := conn.WriteMessage(req.messageType, req.data)
				if err != nil {
					c.logger.Error("写入消息错误 (类型=%d): %v", req.messageType, err)
					// 检查是否为可恢复的连接错误
					if c.isRecoverableError(err) {
						c.logger.Info("写入错误可恢复，触发重连")
						c.tryAutoReconnect(err)
						return
					}
				}
			}
		}
	})
}

// readMessages 从 WebSocket 连接读取消息
func (c *baseClient) readMessages() {
	defer func() {
		c.logger.Debug("读取消息例程已停止")
	}()

	for {
		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()

		if conn == nil {
			c.logger.Warn("readMessage 因 nil ws 连接而退出")
			return
		}

		c.internal.mu.RLock()
		isClosed := c.internal.connClosed
		c.internal.mu.RUnlock()

		if isClosed {
			c.logger.Warn("readMessage 因 ws 连接已关闭而退出")
			return
		}

		// 读取消息

		conn.SetReadDeadline(time.Now().Add(c.readTimeout))
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				c.logger.Warn("读取消息超时")
			} else {
				c.logger.Error("读取消息错误: %v", err)
			}

			// 检查错误是否可恢复
			c.tryAutoReconnect(err)

			return
		}

		c.logger.Debug("收到原始消息 (长度=%d,类型=%v): %s", len(message), messageType, string(message))

		// 处理不同的消息类型
		switch messageType {
		case websocket.TextMessage:
			// 跳过空消息
			if len(message) == 0 {
				c.logger.Debug("收到空消息，跳过")
				continue
			}

			// 跳过 ping/pong 消息
			msgStr := string(message)
			if msgStr == "PING" || msgStr == "PONG" || msgStr == "ping" || msgStr == "pong" {
				c.logger.Debug("收到 PING/PONG，跳过")
				continue
			}

			// // Skip non-JSON messages
			// if message[0] != '{' && message[0] != '[' {
			// 	c.logger.Debug("Received non-JSON message (len=%d, data=%q), skipping", len(message), msgStr)
			// 	continue
			// }

			// c.logger.Debug("Received valid message (len=%d): %s", len(message), msgStr)

			// 调用消息回调
			if c.onNewMessage != nil {
				c.onNewMessage(message)
			}

		case websocket.BinaryMessage:
			c.logger.Debug("收到二进制消息 (长度: %d)", len(message))

			// 调用消息回调
			if c.onNewMessage != nil {
				c.onNewMessage(message)
			}

		case websocket.PingMessage:
			c.logger.Debug("收到 ping")

		case websocket.PongMessage:
			c.logger.Debug("收到 pong")

		case websocket.CloseMessage:
			c.logger.Info("收到服务器关闭消息")
			c.tryAutoReconnect(nil)

			return

		default:
			c.logger.Debug("收到未知消息类型: %d", messageType)
		}
	}
}

func (c *baseClient) tryAutoReconnect(err error) {
	if !c.isRecoverableError(err) {
		c.logger.Error("发生不可恢复错误: %v，不尝试重连", err)
		return
	}

	if !c.autoReconnect {
		c.logger.Info("自动重连已禁用，连接将不会恢复")
		return
	}

	c.logger.Info("连接丢失原因: %v，正在尝试重连...", err)

	// 标记连接已关闭
	c.internal.mu.Lock()
	c.internal.connClosed = true
	c.internal.mu.Unlock()

	// 调用断开连接回调
	if c.onDisconnectCallback != nil {
		c.onDisconnectCallback(err)
	}

	// 触发重连
	go c.reconnect()
}

// isRecoverableError 确定错误是否可恢复并应触发重连
func (c *baseClient) isRecoverableError(err error) bool {
	if err == nil {
		return true
	}

	// 检查常见网络错误
	if errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.ETIMEDOUT) {
		c.logger.Info("检测到可恢复网络错误: %v", err)
		return true
	}

	// 检查 net.OpError (涵盖大多数网络相关错误)
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		c.logger.Info("检测到网络操作错误: %v", netErr)
		return true
	}

	// 检查 WebSocket 关闭错误
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway,
		websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
		c.logger.Info("检测到 WebSocket 关闭错误: %v", err)
		return true
	}

	// 检查常用连接问题的错误消息
	errMsg := strings.ToLower(err.Error())
	recoverableMessages := []string{
		"broken pipe",
		"connection reset",
		"connection reset by peer",
		"connection refused",
		"connection aborted",
		"i/o timeout",
		"timeout",
		"use of closed network connection",
		"connection closed",
		"eof",
		"unexpected eof",
		"network is unreachable",
		"no route to host",
	}

	for _, msg := range recoverableMessages {
		if strings.Contains(errMsg, msg) {
			c.logger.Info("检测到可恢复连接错误: %s", err.Error())
			return true
		}
	}

	// 对于未知错误，记录警告并仍尝试恢复
	c.logger.Warn("尝试从未知错误中恢复: %v", err)

	return true
}

// reconnect 尝试以指数回退方式重连到 WebSocket 服务器
func (c *baseClient) reconnect() {
	c.internal.mu.Lock()

	// 检查是否并未重连
	if c.internal.isReconnecting {
		c.logger.Debug("重连已在进行中，跳过重复尝试")
		c.internal.mu.Unlock()
		return
	}

	// 标记为正在重连
	c.internal.isReconnecting = true
	c.internal.reconnectAttempts = 0
	c.internal.mu.Unlock()

	maxAttempts := c.maxReconnectAttempts
	if maxAttempts == 0 {
		c.logger.Info("开始重连过程 (启用无限重试)")
	} else {
		c.logger.Info("开始重连过程 (最大 %d 次尝试)", maxAttempts)
	}

	// 指数回退参数
	backoff := c.reconnectBackoffInit

	for {
		// 检查是否应停止重连
		c.internal.mu.RLock()
		if !c.autoReconnect {
			c.internal.mu.RUnlock()

			c.logger.Info("自动重连已禁用，停止重连尝试")
			c.internal.mu.Lock()
			c.internal.isReconnecting = false
			c.internal.mu.Unlock()

			time.Sleep(100 * time.Millisecond)
			continue
		}

		// 检查最大尝试次数
		if c.maxReconnectAttempts > 0 && c.internal.reconnectAttempts >= c.maxReconnectAttempts {
			c.internal.mu.RUnlock()
			c.logger.Error("达到最大重连尝试次数 (%d)，放弃", c.maxReconnectAttempts)
			c.internal.mu.Lock()
			c.internal.isReconnecting = false
			c.internal.mu.Unlock()
			return
		}
		c.internal.mu.RUnlock()

		// 增加尝试计数器
		c.internal.mu.Lock()
		c.internal.reconnectAttempts++
		attempt := c.internal.reconnectAttempts
		c.internal.mu.Unlock()

		if c.maxReconnectAttempts > 0 {
			c.logger.Info("重连尝试 %d/%d (等待 %v 后重试)", attempt, c.maxReconnectAttempts, backoff)
		} else {
			c.logger.Info("重连尝试 %d (等待 %v 后重试)", attempt, backoff)
		}

		// 尝试前等待
		time.Sleep(backoff)

		// 尝试重连
		c.logger.Debug("正在尝试建立连接...")
		err := c.connect()
		if err != nil {
			// 指数增加回退
			nextBackoff := backoff * 2
			if nextBackoff > c.reconnectBackoffMax {
				nextBackoff = c.reconnectBackoffMax
			}

			c.logger.Error("重连尝试 %d 失败: %v (下次重试在 %v 后)", attempt, err, nextBackoff)
			backoff = nextBackoff

			continue
		}

		// 重连成功
		c.logger.Info("✓ 重连成功，经过 %d 次尝试", attempt)

		// 恢复订阅
		c.restoreSubscriptions()

		// 调用重连回调
		if c.onReconnectCallback != nil {
			c.onReconnectCallback()
		}

		// 重置重连状态
		c.internal.mu.Lock()
		c.internal.isReconnecting = false
		c.internal.reconnectAttempts = 0
		c.internal.connClosed = false
		c.internal.mu.Unlock()

		c.logger.Info("连接已完全恢复并就绪")
		return
	}
}

// restoreSubscriptions 在重连后恢复所有之前的订阅
func (c *baseClient) restoreSubscriptions() {
	c.internal.mu.RLock()
	subscriptions := c.internal.subscriptions
	c.internal.mu.RUnlock()

	if len(subscriptions) == 0 {
		c.logger.Warn("没有要恢复的订阅")
		return
	}

	c.logger.Info("正在恢复 %d 个订阅", len(subscriptions))

	// 临时存储订阅
	subs := make([]Subscription, len(subscriptions))
	copy(subs, subscriptions)

	// 在重新订阅前清除订阅
	c.internal.mu.Lock()
	c.internal.subscriptions = make([]Subscription, 0)
	c.internal.mu.Unlock()

	// 重新订阅
	err := c.subscribe(subs)
	if err != nil {
		c.logger.Error("恢复订阅失败: %v", err)
	} else {
		c.logger.Info("成功恢复订阅")
	}
}

// subscriptionsToJSON 将订阅转换为 JSON 的辅助函数 (用于实时协议)
func subscriptionsToJSON(subscriptions []Subscription) ([]byte, error) {
	type subscriptionMessage struct {
		Action        string         `json:"action"`
		Subscriptions []Subscription `json:"subscriptions"`
	}

	msg := subscriptionMessage{
		Action:        "subscribe",
		Subscriptions: subscriptions,
	}

	return json.Marshal(msg)
}
