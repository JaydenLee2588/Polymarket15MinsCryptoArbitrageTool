package src

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
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

