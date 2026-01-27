# GA 优化工具使用指南

本文档描述如何使用遗传算法(GA)优化框架来优化交易策略参数。

## 工具概览

| 工具 | 命令 | 用途 |
|------|------|------|
| 主程序 | `go run main.go` | 实盘交易 + 数据收集 |
| 回测工具 | `go run cmd/backtest/main.go` | 策略回测验证 |
| GA 优化器 | `go run cmd/ga-optimizer/main.go` | 参数自动优化 |

---

## 操作流程

### 第一步：收集历史数据

运行主程序至少 **15 分钟**以上，系统会自动收集 K 线和 OrderBook 数据：

```bash
go run main.go
```

数据会保存到 `data/history/` 目录，文件格式：
- `YYYYMMDD_HHMMSS_klines.csv` - K 线数据
- `YYYYMMDD_HHMMSS_orderbooks.csv` - 订单簿快照

> **提示**：收集时间越长，回测和优化结果越可靠。建议至少运行 1-2 小时。

---

### 第二步：回测验证

使用回测工具验证当前策略表现：

```bash
# 基础回测
go run cmd/backtest/main.go --data data/history

# 详细输出（显示交易记录）
go run cmd/backtest/main.go --data data/history -v

# 使用指定配置文件
go run cmd/backtest/main.go --data data/history --config strategy_config_2.yaml

# 筛选特定币种
go run cmd/backtest/main.go --data data/history --symbol BTC -v
```

**输出示例**：
```
===============================================
  BACKTEST RESULTS
===============================================
  Total Decisions:  1234
  Trade Count:      45
  Win / Loss:       32 / 13
  Total Volume:     $2500.00
  Total PnL:        $125.4567
  Max Drawdown:     $23.1200
  Sharpe Ratio:     1.2345
  Win Rate:         71.1%
  Avg PnL/Trade:    $2.7879
```

---

### 第三步：GA 参数优化

运行遗传算法优化器寻找最优参数：

```bash
# 默认配置
go run cmd/ga-optimizer/main.go --data data/history

# 自定义种群大小和迭代次数
go run cmd/ga-optimizer/main.go --data data/history --pop 100 --gen 50
```

**参数说明**：
| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--data` | `data/history` | 历史数据目录 |
| `--pop` | 50 | 种群大小（越大搜索越全面，但更慢） |
| `--gen` | 20 | 迭代代数（越多优化越充分） |

**优化的参数**（18 个基因）：
- 时间窗口：`TrendWindowSecStart`, `TrendWindowSecEnd`
- 趋势阈值：`MinTrendPercentUp`, `MinTrendPercentDown`
- Z-Score 层级：`Tier0ZScore`, `Tier1ZScore`, `Tier2ZScore`
- 最小边际：`MinEdgeTier0`, `MinEdgeTier1`, `MinEdgeTier2`
- Sigmoid 敏感度：`SigmoidSensitivity`
- 波动率因子：`VolatilityBTC`, `VolatilityETH`
- 下单量：`ShareAmount1`, `ShareAmount2`, `ShareAmount3`

---

### 第四步：应用优化结果

GA 优化完成后，将输出最佳参数：

```
Optimization Complete!
Best Fitness: 234.5678
Best Parameters:
  TrendWindowSecStart: 342.0000
  Tier0ZScore: 1.2500
  MinEdgeTier0: 0.0450
  ...
```

将这些参数手动更新到 `strategy_config_2.yaml` 中，然后重新运行主程序。

---

## 最佳实践

1. **数据充分**：确保收集足够的历史数据（至少覆盖多个市场周期）
2. **种群规模**：初次优化建议 `--pop 30 --gen 10` 快速验证，正式优化用 `--pop 100 --gen 50`
3. **交叉验证**：用一部分数据优化，另一部分数据验证，避免过拟合
4. **渐进优化**：先优化核心参数（Z-Score、Edge），再微调其他参数

---

## 文件结构

```
├── cmd/
│   ├── backtest/
│   │   └── main.go          # 回测 CLI
│   └── ga-optimizer/
│       └── main.go          # GA 优化器 CLI
├── tools/
│   ├── backtest/
│   │   └── backtest.go      # 回测引擎核心
│   └── ga/
│       └── optimizer.go     # 遗传算法实现
├── internal/
│   └── datarecorder/
│       └── recorder.go      # 数据记录器
├── data/
│   └── history/             # 历史数据存储
└── docs/
    └── ga_usage.md          # 本文档
```
