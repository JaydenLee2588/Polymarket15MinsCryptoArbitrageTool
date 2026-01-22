// 测试新的 realtime 包的实际连接
//
// 用法: go run test_realtime.go
package main

import (
	"fmt"
	"time"

	realtime "Polymarket15MinsCryptoArbitrageTool/src/realtime"
)

// 测试用的 token ID（BTC市场的一个真实 token）
// 这些是 Polymarket 上的实际市场 token
var testTokenIDs = []string{
	"21742633143463906290569050155826241533067272736897614950488156847949938836455", // BTC 价格市场的一个 token
}

func main() {
	fmt.Println("========================================")
	fmt.Println("Realtime 包连接测试")
	fmt.Println("========================================")
	fmt.Println()

	// 创建一个简单的日志器
	logger := &simpleLogger{}

	// 创建流
	stream := realtime.NewStream(realtime.StreamConfig{
		PingInterval:   10 * time.Second,
		StaleThreshold: 60 * time.Second,
		Logger:         logger,
	})

	// 连接
	fmt.Println("正在连接...")
	if err := stream.Connect(); err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer stream.Close()

	fmt.Println("✅ 连接成功")

	// 创建消息通道
	ch := make(chan realtime.StreamMessage, 100)

	// ⚠️ 关键：必须发送实际的 token ID 订阅，不能只用通配符
	// polymarkt-go 格式: {"assets_ids": [...], "type": "market"}
	fmt.Printf("订阅 %d 个资产...\n", len(testTokenIDs))
	if err := stream.Subscribe(testTokenIDs, ch); err != nil {
		fmt.Printf("❌ 订阅失败: %v\n", err)
		return
	}

	// 等待消息
	fmt.Println("等待消息 (60 秒)...")
	timeout := time.After(60 * time.Second)
	msgCount := 0

	for {
		select {
		case <-timeout:
			fmt.Printf("\n超时，共收到 %d 条消息\n", msgCount)
			if msgCount > 0 {
				fmt.Println("✅ 测试成功：收到了数据消息")
			} else {
				fmt.Println("⚠️ 未收到数据消息，可能是市场不活跃或 token ID 无效")
			}
			return
		case msg := <-ch:
			msgCount++
			fmt.Printf("[%d] EventType=%s Market=%s AssetID=%s\n",
				msgCount, msg.EventType, msg.Market, msg.AssetID)
			if len(msg.PriceChanges) > 0 {
				for i, pc := range msg.PriceChanges {
					fmt.Printf("    PriceChange[%d]: AssetID=%s Price=%s Side=%s\n",
						i, pc.AssetID, pc.Price, pc.Side)
				}
			}
			if msgCount >= 10 {
				fmt.Println("\n✅ 收到足够消息，测试成功!")
				return
			}
		}
	}
}

type simpleLogger struct{}

func (l *simpleLogger) Debug(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}
func (l *simpleLogger) Info(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}
func (l *simpleLogger) Warn(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}
func (l *simpleLogger) Error(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}
