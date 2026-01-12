package src

import (
	"net/url"
	"time"
)

const (
	defaultPingInterval         = 5 * time.Second
	defaultAutoReconnect        = true
	defaultMaxReconnectAttempts = 10 // 0 means infinite retries
	defaultReconnectBackoffInit = 1 * time.Second
	defaultReconnectBackoffMax  = 30 * time.Second
	defaultWriteTimeout         = 5 * time.Second
	defaultReadTimeout          = 30 * time.Second
)

// Config 包含 WebSocket 客户端的所有配置选项
type Config struct {
	Host                 string
	Logger               Logger
	PingInterval         time.Duration
	AutoReconnect        bool
	MaxReconnectAttempts int
	ReconnectBackoffInit time.Duration
	ReconnectBackoffMax  time.Duration
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	ProxyURL             *url.URL // 可选的 HTTP/HTTPS 代理 URL

	OnConnectCallback    func()
	OnNewMessage         func([]byte)
	OnDisconnectCallback func(error)
	OnReconnectCallback  func()
}

type ClientOption func(*Config)

// WithLogger 设置客户端的日志记录器
func WithLogger(l Logger) ClientOption {
	return func(c *Config) {
		c.Logger = l
	}
}

// WithPingInterval 设置客户端的 ping 间隔
func WithPingInterval(interval time.Duration) ClientOption {
	return func(c *Config) {
		c.PingInterval = interval
	}
}

// WithHost 设置客户端的主机
func WithHost(host string) ClientOption {
	return func(c *Config) {
		c.Host = host
	}
}

// WithOnConnect 设置客户端的 onConnect 回调
func WithOnConnect(f func()) ClientOption {
	return func(c *Config) {
		c.OnConnectCallback = f
	}
}

func withOnNewMessage(f func([]byte)) ClientOption {
	return func(c *Config) {
		c.OnNewMessage = f
	}
}

// WithAutoReconnect 启用或禁用连接失败时的自动重连
func WithAutoReconnect(enabled bool) ClientOption {
	return func(c *Config) {
		c.AutoReconnect = enabled
	}
}

// WithMaxReconnectAttempts 设置最大重连尝试次数
// 设置为 0 表示无限重试
func WithMaxReconnectAttempts(max int) ClientOption {
	return func(c *Config) {
		c.MaxReconnectAttempts = max
	}
}

// WithReconnectBackoff 设置重连尝试的初始和最大回退时间
func WithReconnectBackoff(initial, max time.Duration) ClientOption {
	return func(c *Config) {
		c.ReconnectBackoffInit = initial
		c.ReconnectBackoffMax = max
	}
}

// WithReadTimeout Set read timeout
func WithReadTimeout(timeout time.Duration) ClientOption {
	return func(c *Config) {
		c.ReadTimeout = timeout
	}
}

// WithWriteTimeout Set write timeout
func WithWriteTimeout(timeout time.Duration) ClientOption {
	return func(c *Config) {
		c.WriteTimeout = timeout
	}
}

// WithOnDisconnect 设置当连接丢失时调用的回调
func WithOnDisconnect(f func(error)) ClientOption {
	return func(c *Config) {
		c.OnDisconnectCallback = f
	}
}

// WithOnReconnect 设置当重连成功时调用的回调
func WithOnReconnect(f func()) ClientOption {
	return func(c *Config) {
		c.OnReconnectCallback = f
	}
}

// WithProxyURL 设置 WebSocket 连接的 HTTP/HTTPS 代理 URL
// 代理 URL 格式应为: http://host:port 或 https://host:port
// 如果为 nil，则不使用代理（默认行为）
//
// 示例:
//
//	proxyURL, _ := url.Parse("http://127.0.0.1:7897")
//	client := FKGoPolyTools.New(
//		FKGoPolyTools.WithProxyURL(proxyURL),
//	)
func WithProxyURL(proxyURL *url.URL) ClientOption {
	return func(c *Config) {
		c.ProxyURL = proxyURL
	}
}

// MessageRouter 定义将消息路由到处理程序的接口
type MessageRouter interface {
	RouteMessage(data []byte) error
}

// withRouter 设置自动路由所有传入消息的消息路由器
// 这消除了在 withOnNewMessage 中手动调用 router.RouteMessage() 的需要
//
// 示例用法:
//
//	typedSub := FKGoPolyTools.NewRealtimeTypedSubscriptionHandler(nil)
//	client := FKGoPolyTools.New(
//	    FKGoPolyTools.WithRouter(typedSub.GetRouter()),
//	    FKGoPolyTools.WithLogger(FKGoPolyTools.NewLogger()),
//	)
//	typedSub.SetClient(client)
func withRouter(router MessageRouter) ClientOption {
	return func(c *Config) {
		if router != nil {
			// 包装现有的 OnNewMessage 回调
			existingCallback := c.OnNewMessage
			c.OnNewMessage = func(data []byte) {
				// 首先调用路由器
				if err := router.RouteMessage(data); err != nil {
					// 记录错误但不失败
					if c.Logger != nil {
						c.Logger.Error("路由消息失败: %v", err)
					}
				}

				// 然后调用现有的回调（如果有）
				if existingCallback != nil {
					existingCallback(data)
				}
			}
		}
	}
}

