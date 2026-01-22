// 使用 uTLS 测试 Polymarket WebSocket 端点
//
// 用法: go run test_endpoints_utls.go
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
)

// 测试的端点列表
var endpoints = []string{
	"wss://ws-live-data.polymarket.com",                    // 官方库默认端点
	"wss://ws-subscriptions-clob.polymarket.com/ws/market", // CLOB 市场端点
}

func main() {
	fmt.Println("========================================")
	fmt.Println("Polymarket WebSocket 端点 uTLS 连接测试")
	fmt.Println("========================================")
	fmt.Println()

	for _, endpoint := range endpoints {
		testEndpointWithUTLS(endpoint)
		fmt.Println()
	}
}

func testEndpointWithUTLS(endpoint string) {
	fmt.Printf("测试端点 (uTLS Chrome): %s\n", endpoint)
	fmt.Println("----------------------------------------")

	dialer := websocket.Dialer{
		HandshakeTimeout:  15 * time.Second,
		NetDialTLSContext: dialTLSWithUTLS,
	}

	// 使用完整的浏览器 Header
	headers := http.Header{}
	headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36")
	headers.Set("Origin", "https://polymarket.com")
	headers.Set("Accept-Language", "en-US,en;q=0.9")

	startTime := time.Now()
	conn, resp, err := dialer.Dial(endpoint, headers)
	elapsed := time.Since(startTime)

	if err != nil {
		fmt.Printf("❌ 连接失败 (耗时 %v)\n", elapsed)
		fmt.Printf("   错误: %v\n", err)
		if resp != nil {
			fmt.Printf("   HTTP 状态: %s\n", resp.Status)
			if resp.Body != nil {
				defer resp.Body.Close()
				bodyBytes, _ := io.ReadAll(resp.Body)
				if len(bodyBytes) > 0 && len(bodyBytes) < 500 {
					fmt.Printf("   响应体: %s\n", string(bodyBytes))
				}
			}
		}
		return
	}
	defer conn.Close()

	fmt.Printf("✅ 连接成功 (耗时 %v)\n", elapsed)

	// 测试1: 发送小写 ping (官方库格式)
	fmt.Println("   发送 'ping'...")
	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		fmt.Printf("   ⚠️ 发送 ping 失败: %v\n", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		fmt.Printf("   ⚠️ ping 响应失败: %v\n", err)
		// 继续尝试订阅
	} else {
		fmt.Printf("   ping 响应: %s\n", string(message))
	}

	// 测试2: 发送订阅
	fmt.Println("   发送 activity 订阅...")
	subMsg := `{"action":"subscribe","subscriptions":[{"topic":"activity","type":"trades"}]}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(subMsg)); err != nil {
		fmt.Printf("   ⚠️ 订阅失败: %v\n", err)
		return
	}

	// 读取订阅响应
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err = conn.ReadMessage()
	if err != nil {
		fmt.Printf("   ⚠️ 订阅响应失败: %v\n", err)

		// 尝试 CLOB 格式的订阅
		fmt.Println("   尝试 CLOB 格式订阅...")
		clobSubMsg := `{"assets_ids":[],"type":"market"}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(clobSubMsg)); err != nil {
			fmt.Printf("   ⚠️ CLOB 订阅失败: %v\n", err)
			return
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, message, err = conn.ReadMessage()
		if err != nil {
			fmt.Printf("   ⚠️ CLOB 订阅响应失败: %v\n", err)
			return
		}
	}
	if len(message) > 200 {
		fmt.Printf("   订阅响应: %s...\n", string(message[:200]))
	} else {
		fmt.Printf("   订阅响应: %s\n", string(message))
	}

	// 等待数据消息
	fmt.Println("   等待数据消息...")
	for i := 0; i < 3; i++ {
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		_, message, err = conn.ReadMessage()
		if err != nil {
			fmt.Printf("   ⚠️ 等待数据超时: %v\n", err)
			break
		}
		if len(message) > 100 {
			fmt.Printf("   数据 #%d: %s...\n", i+1, string(message[:100]))
		} else {
			fmt.Printf("   数据 #%d: %s\n", i+1, string(message))
		}
	}
}

// dialTLSWithUTLS 使用 uTLS 模拟 Chrome TLS 指纹
func dialTLSWithUTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	rawConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("TCP dial failed: %w", err)
	}

	spec, err := utls.UTLSIdToSpec(utls.HelloChrome_Auto)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("failed to get Chrome spec: %w", err)
	}

	// 修改 ALPN 只保留 http/1.1
	for i, ext := range spec.Extensions {
		if alpnExt, ok := ext.(*utls.ALPNExtension); ok {
			alpnExt.AlpnProtocols = []string{"http/1.1"}
			spec.Extensions[i] = alpnExt
			break
		}
	}

	uTLSConfig := &utls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	uTLSConn := utls.UClient(rawConn, uTLSConfig, utls.HelloCustom)
	if err := uTLSConn.ApplyPreset(&spec); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("failed to apply custom spec: %w", err)
	}

	if err := uTLSConn.Handshake(); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("uTLS handshake failed: %w", err)
	}

	return uTLSConn, nil
}
