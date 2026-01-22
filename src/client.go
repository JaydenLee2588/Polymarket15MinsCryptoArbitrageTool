package src

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Client 是 Gamma API 的客户端（用于查询事件和市场元数据）
type Client struct {
	host       string
	httpClient *http.Client
}

// NewGammaClient 创建一个新的 Gamma API 客户端，用于查询事件和市场元数据
func NewGammaClient(httpClient *http.Client) *Client {
	return &Client{
		host:       GammaAPIURL,
		httpClient: httpClient,
	}
}

// NewDataClient 创建一个新的 Data API 客户端
func NewDataClient(httpClient *http.Client) *Client {
	return &Client{
		host:       DataAPIURL,
		httpClient: httpClient,
	}
}

// doRequest 向 Gamma API 或 Data API 发送 HTTP 请求
func (c *Client) doRequest(ctx context.Context, method, path string) ([]byte, error) {
	fullURL := ""
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		fullURL = path
	} else {
		fullURL = c.host + path
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")

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

	return body, nil
}

// Subscription 代表订阅主题和消息类型
type Subscription struct {
	Topic   Topic       `json:"topic"`
	Type    MessageType `json:"type"`
	Filters string      `json:"filters,omitempty"`

	ClobAuth  *ClobAuth  `json:"clob_auth,omitempty"`
	GammaAuth *GammaAuth `json:"gamma_auth,omitempty"`
}

// ClobAuth 包含 CLOB 订阅的认证信息
type ClobAuth struct {
	Key        string `json:"key"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

// GammaAuth 包含 Gamma 订阅的认证信息
type GammaAuth struct {
	Address string `json:"address"`
}

// WsClient 接口用于向后兼容
type WsClient interface {
	// Connect 建立到服务器的 WebSocket 连接
	Connect() error

	// Disconnect 关闭 WebSocket 连接
	Disconnect() error

	// Subscribe 向服务器发送订阅消息
	Subscribe(subscriptions []Subscription) error

	// Unsubscribe 向服务器发送取消订阅消息
	Unsubscribe(subscriptions []Subscription) error
}

// WsClientWrapper 是 baseClient 的轻量级包装器，用于向后兼容
// 它使用支持多个主题的 RealtimeProtocol
type WsClientWrapper struct {
	*baseClient
	*RealtimeTypedSubscriptionHandler
}

// New 使用 baseClient 基础设施创建新客户端
// 这提供了向后兼容性，同时使用改进的 baseClient 实现
func New(opts ...ClientOption) *WsClientWrapper {
	protocol := NewRealtimeProtocol()

	router := NewRealtimeMessageRouter()
	opts = append(opts, withRouter(router))
	base := newBaseClient(protocol, opts...)
	cli := &WsClientWrapper{baseClient: base}

	handler := NewRealtimeTypedSubscriptionHandler(cli, router)

	cli.RealtimeTypedSubscriptionHandler = handler
	return cli
}

// Connect 建立到服务器的 WebSocket 连接
func (c *WsClientWrapper) Connect() error {
	return c.baseClient.connect()
}

// Disconnect 关闭 WebSocket 连接
func (c *WsClientWrapper) Disconnect() error {
	return c.baseClient.disconnect()
}

// Subscribe 向服务器发送订阅请求
func (c *WsClientWrapper) Subscribe(subscriptions []Subscription) error {
	return c.baseClient.subscribe(subscriptions)
}

// Unsubscribe 向服务器发送取消订阅请求
func (c *WsClientWrapper) Unsubscribe(subscriptions []Subscription) error {
	return c.baseClient.unsubscribe(subscriptions)
}

// IsReconnecting 检查是否正在重连 (用于外部判断是否需要触发 ForceReconnect)
func (c *WsClientWrapper) IsReconnecting() bool {
	return atomic.LoadInt32(&c.baseClient.reconnecting) == 1
}

// ErrReconnectInProgress 表示重连已在进行中，ForceReconnect 被跳过
var ErrReconnectInProgress = errors.New("reconnect already in progress")

// ForceReconnect 强制断开连接并等待 SDK 内部自动重连完成 (用于应用层看门狗和 Hot Reconnect)
// ⚠️ 此函数是同步的：触发断开后会等待重连完成才返回
// SDK 内部的 tryAutoReconnect() 会自动处理重连和订阅恢复
func (c *WsClientWrapper) ForceReconnect() error {
	// ⚠️ 如果已经在重连中，返回特定错误以便调用方正确处理
	if c.IsReconnecting() {
		c.baseClient.logger.Info("🔄 重连已在进行中，跳过 ForceReconnect")
		return ErrReconnectInProgress
	}

	c.baseClient.logger.Info("🔄 应用层触发强制断开，等待 SDK 自动重连...")

	// 强制断开连接 (不修改 autoReconnect)
	// readMessages() 会检测到连接关闭错误，自动触发 tryAutoReconnect()
	if err := c.baseClient.forceDisconnect(); err != nil {
		c.baseClient.logger.Warn("断开连接时出错: %v", err)
		return err
	}

	// ⚠️ 等待 SDK 内部重连完成 (最多 30 秒)
	// 分两阶段：1) 等待重连开始 2) 等待重连完成
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	// 阶段 1: 等待重连开始 (IsReconnecting 变为 true)
	c.baseClient.logger.Debug("等待 SDK 内部重连启动...")
	reconnectStarted := false
	for !reconnectStarted {
		select {
		case <-timeout:
			c.baseClient.logger.Error("等待重连启动超时")
			return errors.New("reconnect start timeout")
		case <-ticker.C:
			if c.IsReconnecting() {
				reconnectStarted = true
				c.baseClient.logger.Debug("SDK 内部重连已启动")
			}
		}
	}

	// 阶段 2: 等待重连完成 (连接建立且不再重连中)
	c.baseClient.logger.Debug("等待 SDK 内部重连完成...")
	for {
		select {
		case <-timeout:
			c.baseClient.logger.Error("等待重连完成超时 (30秒)")
			return errors.New("reconnect timeout")
		case <-ticker.C:
			// 检查重连是否完成：不再重连中 && 连接存在 && 连接未关闭
			if !c.IsReconnecting() {
				c.baseClient.connMu.RLock()
				conn := c.baseClient.conn
				c.baseClient.connMu.RUnlock()

				c.baseClient.internal.mu.RLock()
				isClosed := c.baseClient.internal.connClosed
				c.baseClient.internal.mu.RUnlock()

				if conn != nil && !isClosed {
					c.baseClient.logger.Info("✓ SDK 重连完成")
					return nil
				}
			}
		}
	}
}
