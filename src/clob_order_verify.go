package src

import (
	"context"
	"fmt"
	"log"
	"time"
)

// VerifyOrderStatusResult 订单验证结果
type VerifyOrderStatusResult struct {
	Success      bool                 // 是否验证成功
	Status       *OrderStatusResponse // 订单状态（成功时）
	ErrorMessage string               // 错误信息（失败时）
	Attempts     int                  // 尝试次数
}

// VerifyOrderCompletion 验证订单是否成交（带重试）
//
// 参数:
//   - ctx: 上下文
//   - orderID: 订单ID
//   - maxRetries: 最大重试次数（默认3次）
//   - retryInterval: 重试间隔（默认3秒）
//
// 返回:
//   - VerifyOrderStatusResult: 验证结果
func (c *ClobOrderClient) VerifyOrderCompletion(
	ctx context.Context,
	orderID string,
	maxRetries int,
	retryInterval time.Duration,
) *VerifyOrderStatusResult {
	// 设置默认值
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if retryInterval <= 0 {
		retryInterval = 3 * time.Second
	}

	result := &VerifyOrderStatusResult{
		Success:  false,
		Attempts: 0,
	}

	for i := 0; i < maxRetries; i++ {
		result.Attempts++

		// 第一次之后才等待
		if i > 0 {
			log.Printf("⏳ 等待 %v 后进行第 %d 次订单状态查询...", retryInterval, i+1)
			time.Sleep(retryInterval)
		} else {
			// 第一次查询前等待
			log.Printf("⏳ 等待 %v 后查询订单状态...", retryInterval)
			time.Sleep(retryInterval)
		}

		// 查询订单状态
		status, err := c.GetOrderStatus(ctx, orderID)
		if err != nil {
			log.Printf("⚠️ 第 %d 次查询失败: %v", i+1, err)
			result.ErrorMessage = fmt.Sprintf("查询失败: %v", err)
			continue
		}

		// 记录状态
		log.Printf("🔍 第 %d 次查询: 状态=%s, 成交=%s/%s",
			i+1, status.Status, status.SizeMatched, status.OriginalSize)

		// 检查是否完全成交
		if status.Status == "MATCHED" {
			log.Printf("✅ 订单已完全成交！")
			result.Success = true
			result.Status = status
			return result
		}

		// 检查是否被取消或失败
		if status.Status == "CANCELLED" || status.Status == "EXPIRED" {
			log.Printf("❌ 订单已%s", status.Status)
			result.ErrorMessage = fmt.Sprintf("订单状态: %s", status.Status)
			result.Status = status
			return result
		}

		// 部分成交或仍在挂单中
		if status.Status == "LIVE" {
			log.Printf("⏳ 订单仍在挂单中...")
		}
	}

	// 达到最大重试次数仍未成交
	if result.ErrorMessage == "" {
		result.ErrorMessage = fmt.Sprintf("达到最大重试次数 %d 次，订单仍未完全成交", maxRetries)
	}

	log.Printf("❌ 订单验证失败: %s", result.ErrorMessage)
	return result
}
