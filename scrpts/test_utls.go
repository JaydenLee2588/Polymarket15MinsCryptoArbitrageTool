package main

import (
	"fmt"
	"os"
	"time"

	FKGoPolyTools "Polymarket15MinsCryptoArbitrageTool/src"
)

func main() {
	// 创建一个控制台输出的 logger
	logger := FKGoPolyTools.NewLogger(FKGoPolyTools.LogLevelInfo)

	fmt.Println("========================================")
	fmt.Println("uTLS WebSocket 连接测试")
	fmt.Println("========================================")

	// 测试 1: Binance WebSocket (应该能连接成功)
	fmt.Println("\n📡 测试 1: Binance WebSocket")
	fmt.Println("----------------------------------------")
	binanceURL := "wss://stream.binance.com:9443/ws/btcusdt@trade"
	err := FKGoPolyTools.TestUTLSConnectionExported(binanceURL, logger)
	if err != nil {
		fmt.Printf("❌ Binance 连接失败: %v\n", err)
	} else {
		fmt.Println("✅ Binance 连接成功!")
	}

	time.Sleep(2 * time.Second)

	// 测试 2: Polymarket 旧端点 (可能被弃用)
	fmt.Println("\n📡 测试 2: Polymarket WebSocket (旧端点)")
	fmt.Println("----------------------------------------")
	polymarketURL := "wss://ws-live-data.polymarket.com"
	err = FKGoPolyTools.TestUTLSConnectionExported(polymarketURL, logger)
	if err != nil {
		fmt.Printf("❌ Polymarket (旧端点) 连接失败: %v\n", err)
	} else {
		fmt.Println("✅ Polymarket (旧端点) 连接成功!")
	}

	time.Sleep(2 * time.Second)

	// 测试 3: Polymarket 官方 CLOB 端点 (文档推荐)
	fmt.Println("\n📡 测试 3: Polymarket CLOB WebSocket (官方文档)")
	fmt.Println("----------------------------------------")
	clobURL := "wss://ws-subscriptions-clob.polymarket.com/ws/market"
	err = FKGoPolyTools.TestUTLSConnectionExported(clobURL, logger)
	if err != nil {
		fmt.Printf("❌ Polymarket CLOB 连接失败: %v\n", err)
		fmt.Println("⚠️  如果看到 504 或 403 错误，说明 Cloudflare 仍在拦截")
	} else {
		fmt.Println("✅ Polymarket CLOB 连接成功!")
	}

	fmt.Println("\n========================================")
	fmt.Println("测试完成!")
	fmt.Println("========================================")

	os.Exit(0)
}
