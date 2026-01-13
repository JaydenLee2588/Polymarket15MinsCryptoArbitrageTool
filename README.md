# Polymarket 15分钟加密货币套利工具

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)]()

基于 Binance 趋势数据的 Polymarket 15分钟市场自动化交易工具

[功能特性](#-功能特性) • [快速开始](#-快速开始) • [配置说明](#-配置说明) • [API文档](#-api-文档) • [策略优化](#-策略优化)

</div>

---

## 📖 项目简介

这是一个针对 Polymarket 15分钟加密货币市场的自动化交易工具。通过实时监控 Binance 现货市场的 K线趋势，在 Polymarket 的 BTC/ETH/SOL/XRP 15分钟预测市场中寻找套利机会。

### 核心策略

1. **趋势检测**: 基于 Binance 15分钟K线计算趋势百分比
2. **公允价格计算**: `fairPrice = 0.5 + (trendPercent × 100 × trendFactor)`
3. **价格优势判断**: `priceEdge = fairPrice - marketPrice ≥ minPriceEdge`
4. **动态仓位管理**: 根据入场价格区间自动调整购买数量

### 支持模式

- **模拟交易**: 基于实时市场数据进行模拟交易，无需资金
- **真实交易**: 通过 Polymarket CLOB API 执行真实订单

---

## ✨ 功能特性

### 交易功能

- ✅ **多配置策略**: 支持同时运行多个策略配置进行 A/B 测试
- ✅ **双交易模式**: 模拟交易（回测）+ 真实交易（需私钥）
- ✅ **智能下单**: 支持市价单（FOK）和限价单（GTC）
- ✅ **订单验证**: 真实交易后自动查询订单状态确认成交
- ✅ **动态仓位**: 根据入场价格自动调整仓位大小

### 数据与监控

- 📊 **实时 WebSocket**: 连接 Polymarket 和 Binance WebSocket
- 📈 **交易统计**: 按配置组统计盈亏、胜率、交易次数
- 📝 **完整日志**: 记录每笔交易的时间、价格、数量、成本、趋势等
- 💾 **SQLite 存储**: 本地数据库存储所有交易记录

### 集成与接口

- 🤖 **Telegram Bot**: 通过 Telegram 查询统计和控制策略
- 🌐 **RESTful API**: 完整的 HTTP API 接口
- 🎨 **Web Dashboard**: 前端页面查看实时数据和测试 API

---

## 🚀 快速开始

### 环境要求

- Go 1.24+
- SQLite 3.x
- （可选）Telegram Bot Token

### 安装步骤

1. **克隆项目**
```bash
git clone https://github.com/yourusername/Polymarket15MinsCryptoArbitrageTool.git
cd Polymarket15MinsCryptoArbitrageTool
```

2. **安装依赖**
```bash
go mod download
```

3. **配置环境变量**
```bash
cp .env.example .env
# 编辑 .env 文件，配置必要参数
```

4. **启动服务**
```bash
go run main.go
```

服务默认运行在 `http://localhost:8090`

### Docker 部署（可选）

```bash
docker build -t polymarket-arbitrage .
docker run -p 8090:8090 --env-file .env polymarket-arbitrage
```

---

## ⚙️ 配置说明

### 环境变量 (.env)

```bash
# 服务器配置
HOST=0.0.0.0
PORT=8090

# 数据库路径
DB_PATH=data/trades.db
BOT_DB_PATH=data/bot.db

# Telegram Bot Token (可选)
TELEGRAM_BOT_TOKEN=your_telegram_bot_token

# 交易模式: simulation (模拟) / real (真实)
TRADING_MODE=simulation

# 下单类型: FOK (全部成交或取消) / FAK (部分成交) / GTC (限价挂单)
ORDER_TYPE=GTC

# Polymarket 钱包私钥 (真实交易时必填)
POLYMARKET_PRIVATE_KEY=0x...

# 策略配置文件
CONFIG_FILE=strategy_config.yaml

# 市场扫描间隔 (秒)
SCAN_INTERVAL=5
```

### 策略配置 (strategy_config.yaml)

```yaml
groups:
  - id: "F1.85_450"  # 配置组ID
    # Binance 趋势检测
    min_trend_percent_up: 0.001      # 做多最小趋势阈值 (0.1%)
    min_trend_percent_down: 0.001    # 做空最小趋势阈值 (0.1%)
    min_price_edge: 0.08             # 最小价格优势
    fair_price_trend_factor: 1.85    # 趋势转换系数
    
    # 检测时间窗口
    trend_window_sec_start: 118      # 开盘后 118秒 开始检测
    trend_window_sec_end: 450        # 开盘后 450秒 停止检测
    
    # 动态仓位配置
    leg0_share_price_0: 0.54         # 最低价格阈值
    leg1_share_price_1: 0.66         # 第一价格阈值
    leg1_share_price_2: 0.78         # 第二价格阈值  
    leg1_share_price_3: 0.90         # 第三价格阈值
    
    leg1_share_amount_1: 40          # 价格 ∈ (0.54, 0.66] → 买入 40份
    leg1_share_amount_2: 33          # 价格 ∈ (0.66, 0.78] → 买入 33份
    leg1_share_amount_3: 28          # 价格 ∈ (0.78, 0.90] → 买入 28份
    
    fee_rate: 0.003                  # 费率 0.3%
```

---

## 🌐 API 文档

### 启动策略

**POST** `/api/v1/copy_trading?type=2`

启动多配置策略交易。

**响应**:
```json
{
  "status": "success",
  "message": "Started Multi-Config Simulation (Background)",
  "type": "2",
  "config_count": 8,
  "timestamp": "2026-01-13T05:00:00Z"
}
```

### 查询统计

**GET** `/api/v1/stats?type=2`

获取策略统计数据。

**响应**:
```json
{
  "success": true,
  "data": {
    "2": {
      "trade_count": 365,
      "total_pnl": 2611.62,
      "win_count": 333,
      "win_rate": 0.9123
    }
  },
  "config_breakdown": {
    "F1.85_450": {
      "trade_count": 45,
      "total_pnl": 363.35,
      "win_count": 42,
      "win_rate": 0.9333
    }
  },
  "uptime": "13小时54分钟"
}
```

### 清空数据

**DELETE** `/api/v1/trades`

删除所有模拟交易记录。

**响应**:
```json
{
  "success": true,
  "message": "All trades deleted successfully"
}
```

### 健康检查

**GET** `/health`

检查服务状态。

---

## 📊 策略优化

### 优化建议

1. **首选配置**: `fair_price_trend_factor = 1.85`
   - 胜率最高（93.3%）
   - 盈利与交易频率平衡最佳
   
2. **检测窗口**: `118s - 600s`
   - 过早入场（<118s）趋势不稳定
   - 延长至 600s 可捕获中期趋势

---

## 🤖 Telegram Bot 使用

### 启动 Bot

1. 向 [@BotFather](https://t.me/BotFather) 申请 Token
2. 配置 `.env` 中的 `TELEGRAM_BOT_TOKEN`
3. 重启服务

### 可用命令

- `/start` - 初始化 Bot
- `/stats` - 查询策略统计
- `/help` - 帮助信息

---

## 📁 项目结构

```
Polymarket15MinsCryptoArbitrageTool/
├── api/                    # API 层
│   ├── strategy.go         # 核心策略逻辑
│   ├── simulation_stats.go # 统计分析
│   ├── data_management.go  # 数据管理
│   └── routes.go           # 路由注册
├── bot/                    # Telegram Bot
│   └── telegram.go
├── internal/               # 内部包
│   └── database/           # 数据库操作
│       └── sqlite.go
├── src/                    # Polymarket SDK
│   ├── clob_order.go       # 订单下单
│   ├── clob_order_status.go # 订单查询
│   ├── binance_ws.go       # Binance WebSocket
│   ├── auth.go             # L1/L2 认证
│   └── ...
├── front/                  # 前端页面
│   ├── index.html
│   └── api-test.html
├── main.go                 # 入口文件
├── strategy_config.yaml    # 策略配置
└── .env                    # 环境变量
```

---

## ⚠️ 风险提示

1. **市场风险**: 加密货币市场波动剧烈，存在本金损失风险
2. **技术风险**: 网络延迟、API限流可能影响交易执行
3. **流动性风险**: Polymarket 15分钟市场流动性有限
4. **合规风险**: 请确保你所在地区允许参与预测市场交易

**建议**:
- 先使用**模拟模式**充分测试
- 真实交易前进行**小额测试**
- 设置合理的**风险敞口**
- 定期监控**策略表现**

---

## 🛠️ 技术栈

- **后端**: Go 1.24
- **数据库**: SQLite 3
- **WebSocket**: gorilla/websocket
- **区块链**: go-ethereum (EIP-712 签名)
- **API**: RESTful HTTP
- **Bot**: telegram-bot-api

---

## 📝 开发路线图

- [x] 核心交易策略
- [x] 多配置 A/B 测试
- [x] 真实交易支持
- [x] 订单状态查询
- [x] Telegram Bot 集成
- [ ] Web Dashboard 优化

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

---

## 📄 许可证

本项目采用 [MIT License](LICENSE) 开源协议。

---

## 👤 作者

**duzhi5368**

- GitHub: [@duzhi5368](https://github.com/duzhi5368)

---

## 🙏 鸣谢

- [Polymarket](https://polymarket.com/) - 去中心化预测市场平台
- [0xNetuser/Polymarket-golang](https://github.com/0xNetuser/Polymarket-golang) - Polymarket Go SDK 参考

---

<div align="center">

⭐ 如果这个项目对你有帮助，请给一个 Star！

Made with ❤️ in Go

</div>
