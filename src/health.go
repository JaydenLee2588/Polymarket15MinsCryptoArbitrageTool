package src

import (
	"context"
	"fmt"
)

// HealthCheck 对 Polymarket 数据 API 执行健康检查
// 如果 API 健康，返回 "OK"
func (c *Client) HealthCheck(ctx context.Context) (bool, bool, error) {
	dataUrl := DataAPIURL + "/"
	gammaUrl := GammaAPIURL + "/"

	_, dataErr := c.doRequest(ctx, "GET", dataUrl)
	if dataErr != nil {
		return false, false, fmt.Errorf("健康检查请求失败: %w", dataErr)
	}

	_, gammaErr := c.doRequest(ctx, "GET", gammaUrl)
	if gammaErr != nil {
		return false, false, fmt.Errorf("健康检查请求失败: %w", gammaErr)
	}

	return true, true, nil
}

