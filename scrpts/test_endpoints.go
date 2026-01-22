// 测试 Polymarket WebSocket 端点连接性
//
// 用法: go run test_endpoints.go
package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// 测试的端点列表
var endpoints = []string{
	"wss://ws-live-data.polymarket.com",                    // 官方库默认端点
	"wss://ws-subscriptions-clob.polymarket.com/ws/market", // CLOB 市场端点
}

func main() {
	fmt.Println("========================================")
	fmt.Println("Polymarket WebSocket 端点连接测试")
	fmt.Println("========================================")
	fmt.Println()

	for _, endpoint := range endpoints {
		testEndpoint(endpoint)
		fmt.Println()
	}
}

func testEndpoint(endpoint string) {
	fmt.Printf("测试端点: %s\n", endpoint)
	fmt.Println("----------------------------------------")

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	// 使用浏览器 User-Agent
	headers := http.Header{}
	headers.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

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
				} else if len(bodyBytes) >= 500 {
					fmt.Printf("   响应体: %s... (截断)\n", string(bodyBytes[:500]))
				}
			}
		}
		return
	}
	defer conn.Close()

	fmt.Printf("✅ 连接成功 (耗时 %v)\n", elapsed)

	// 发送 ping 并等待响应
	fmt.Println("   发送 ping...")
	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		fmt.Printf("   ⚠️ 发送 ping 失败: %v\n", err)
		return
	}

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// 读取响应
	_, message, err := conn.ReadMessage()
	if err != nil {
		fmt.Printf("   ⚠️ 读取响应失败: %v\n", err)
		return
	}

	fmt.Printf("   收到响应: %s\n", string(message))

	// 测试订阅
	fmt.Println("   发送订阅请求...")
	subMsg := `{"action":"subscribe","subscriptions":[{"topic":"activity","type":"trades"}]}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(subMsg)); err != nil {
		fmt.Printf("   ⚠️ 发送订阅请求失败: %v\n", err)
		return
	}

	// 等待订阅响应
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err = conn.ReadMessage()
	if err != nil {
		fmt.Printf("   ⚠️ 读取订阅响应失败: %v\n", err)
		return
	}
	fmt.Printf("   订阅响应: %s\n", string(message))

	// 等待一条数据消息
	fmt.Println("   等待数据消息 (最多 30 秒)...")
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	for i := 0; i < 5; i++ {
		_, message, err = conn.ReadMessage()
		if err != nil {
			fmt.Printf("   ⚠️ 等待数据超时或失败: %v\n", err)
			break
		}
		if len(message) > 100 {
			fmt.Printf("   数据消息 #%d: %s... (截断)\n", i+1, string(message[:100]))
		} else {
			fmt.Printf("   数据消息 #%d: %s\n", i+1, string(message))
		}
	}
}
