package src

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
)

// uTLSConn 包装 utls.UConn 以实现 net.Conn 接口
type uTLSConn struct {
	*utls.UConn
}

// dialTLSWithUTLS 使用 uTLS 建立模拟 Chrome 浏览器 TLS 指纹的连接
func dialTLSWithUTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	// 解析主机名用于 SNI
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	// 创建基础 TCP 连接
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}
	rawConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("TCP dial failed: %w", err)
	}

	// 从 Chrome 指纹获取 spec，然后修改 ALPN
	spec, err := utls.UTLSIdToSpec(utls.HelloChrome_Auto)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("failed to get Chrome spec: %w", err)
	}

	// 修改 ALPN 扩展，只保留 http/1.1 (WebSocket 需要)
	for i, ext := range spec.Extensions {
		if alpnExt, ok := ext.(*utls.ALPNExtension); ok {
			alpnExt.AlpnProtocols = []string{"http/1.1"}
			spec.Extensions[i] = alpnExt
			break
		}
	}

	// 创建 uTLS 配置
	uTLSConfig := &utls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	// 使用 HelloCustom 和修改后的 spec
	uTLSConn := utls.UClient(rawConn, uTLSConfig, utls.HelloCustom)

	// 应用自定义的 ClientHello spec
	if err := uTLSConn.ApplyPreset(&spec); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("failed to apply custom spec: %w", err)
	}

	// 执行 TLS 握手
	if err := uTLSConn.Handshake(); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("uTLS handshake failed: %w", err)
	}

	return uTLSConn, nil
}

// dialTLSWithUTLSViaProxy 使用 uTLS 通过 HTTP 代理建立连接
func dialTLSWithUTLSViaProxy(ctx context.Context, network, addr string, proxyURL *url.URL) (net.Conn, error) {
	// 解析主机名用于 SNI
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	// 连接到代理
	proxyAddr := proxyURL.Host
	if proxyURL.Port() == "" {
		if proxyURL.Scheme == "https" {
			proxyAddr = net.JoinHostPort(proxyURL.Hostname(), "443")
		} else {
			proxyAddr = net.JoinHostPort(proxyURL.Hostname(), "80")
		}
	}

	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}
	proxyConn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("proxy dial failed: %w", err)
	}

	// 发送 CONNECT 请求
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", addr, addr)
	if _, err := proxyConn.Write([]byte(connectReq)); err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("CONNECT write failed: %w", err)
	}

	// 读取代理响应 (简单解析，只检查 200)
	buf := make([]byte, 1024)
	n, err := proxyConn.Read(buf)
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("CONNECT response read failed: %w", err)
	}

	response := string(buf[:n])
	if len(response) < 12 || response[9:12] != "200" {
		proxyConn.Close()
		return nil, fmt.Errorf("CONNECT failed: %s", response)
	}

	// 从 Chrome 指纹获取 spec，然后修改 ALPN
	spec, specErr := utls.UTLSIdToSpec(utls.HelloChrome_Auto)
	if specErr != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to get Chrome spec: %w", specErr)
	}

	// 修改 ALPN 扩展，只保留 http/1.1 (WebSocket 需要)
	for i, ext := range spec.Extensions {
		if alpnExt, ok := ext.(*utls.ALPNExtension); ok {
			alpnExt.AlpnProtocols = []string{"http/1.1"}
			spec.Extensions[i] = alpnExt
			break
		}
	}

	// 创建 uTLS 配置
	uTLSConfig := &utls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	// 使用 HelloCustom 和修改后的 spec
	uTLSConn := utls.UClient(proxyConn, uTLSConfig, utls.HelloCustom)

	// 应用自定义的 ClientHello spec
	if applyErr := uTLSConn.ApplyPreset(&spec); applyErr != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to apply uTLS spec via proxy: %w", applyErr)
	}

	if err := uTLSConn.Handshake(); err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("uTLS handshake via proxy failed: %w", err)
	}

	return uTLSConn, nil
}

// createUTLSWebSocketDialer 创建使用 uTLS Chrome 指纹的 WebSocket Dialer
func createUTLSWebSocketDialer(proxyURL *url.URL) websocket.Dialer {
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	if proxyURL != nil {
		// 使用代理时的自定义 TLS dial
		dialer.NetDialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialTLSWithUTLSViaProxy(ctx, network, addr, proxyURL)
		}
	} else {
		// 直连时的自定义 TLS dial
		dialer.NetDialTLSContext = dialTLSWithUTLS
	}

	return dialer
}

// testUTLSConnection 测试 uTLS 连接到指定的 WebSocket 端点
// 这是一个用于调试的辅助函数
func testUTLSConnection(wsURL string, logger Logger) error {
	return TestUTLSConnectionExported(wsURL, logger)
}

// TestUTLSConnectionExported 测试 uTLS 连接到指定的 WebSocket 端点 (导出版本)
func TestUTLSConnectionExported(wsURL string, logger Logger) error {
	if logger == nil {
		logger = NewSilentLogger()
	}

	logger.Info("🧪 测试 uTLS 连接到: %s", wsURL)

	dialer := createUTLSWebSocketDialer(nil)

	// 使用完整的浏览器请求头 (从 Chrome DevTools 捕获)
	headers := http.Header{}
	headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36")
	headers.Set("Origin", "https://polymarket.com")
	headers.Set("Accept-Language", "en-US,zh-CN;q=0.9,en;q=0.8,es-US;q=0.7,es;q=0.6,ja;q=0.5,zh;q=0.4,zh-TW;q=0.3")
	headers.Set("Pragma", "no-cache")
	headers.Set("Cache-Control", "no-cache")
	// Sec-WebSocket-* 头由 gorilla/websocket 自动设置

	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		if resp != nil {
			logger.Error("❌ uTLS WebSocket 握手失败: Status=%s", resp.Status)
		}
		return fmt.Errorf("uTLS dial failed: %w", err)
	}
	defer conn.Close()

	logger.Info("✅ uTLS WebSocket 连接成功!")

	// 尝试读取一条消息作为验证
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		logger.Warn("⚠️ 读取消息超时或失败 (这可能是正常的): %v", err)
	} else {
		logger.Info("📨 收到消息: %s", string(message))
	}

	return nil
}
