# 遗传算法 (Genetic Algorithm) 优化参数 - 架构设计建议

针对您希望使用 GA 对交易策略参数进行进化优化的需求，当前的工程架构不能直接支持（依赖实时数据，无法快速回测），需要进行架构分层改造。

遗传算法 (GA) 优化方案建议
这是一个非常好的工程方向。要实现 GA 自动寻优，目前的架构需要进行一些调整，因为 GA 需要在短时间内跑完成千上万次模拟，而目前的系统是为实时运行设计的。

我为您起草了一份详细的设计文档：ga_design.md，核心建议如下：

架构分层: 目前的策略逻辑与 WebSocket 耦合太紧。建议把核心的 Evaluate 函数（输入-输出）剥离出来，变成纯函数。这样 GA 就可以直接调用它，而不需要建立网络连接。
数据先行 (Data Infrastructure):
GA 的燃料是历史数据。您需要先有一个机制把实盘的 Tick 数据（Binance K线 + OrderBook 快照）存下来。
建议先做一个 Data Recorder，把未来几天的数据存成 CSV 或 SQLite。
回测引擎 (Backtester):
基于存下来的数据，写一个极速的回测器。它要在内存里重放这几天的数据，并用不同的参数去跑，算出 PnL。
并行进化:
Go 非常适合做这个。利用 Goroutine，您可以同时跑 100 组参数的回测，效率极高。
建议第一步: 如果您想开始，我们可以先做 "策略逻辑解耦"。把 api/strategy.go 里的核心计算部分提取出来，这样既方便实盘调用，也方便未来接回测引擎。


## ⚠️ 核心挑战
1.  **数据缺失**: GA 需要在短时间内跑数千次模拟，依赖实时数据 (Real-time) 是不可行的。必须有 **历史高频数据** (Binance 15m K线 + Polymarket Orderbook Snapshots)。
2.  **回测引擎**: 目前的 `api/strategy.go` 与 `MarketManager` (WebSocket) 强耦合。需要剥离出一个纯粹的 `StrategyEngine`，它既可以接受 WebSocket 推送（实盘），也可以接受 CSV/DB 历史数据回放（回测）。

## 🗓️ 实施路线图 (Crawl-Walk-Run)

### 第一阶段：数据基础设施 (The Foundation)
**目标**: 建立"练功房"。没有历史数据，GA 就没有训练素材。
*   **动作 1**: 创建 `DataRecorder`。单独运行一个服务，负责记录：
    *   Binance 15m K线 (Open, High, Low, Close, Time)。
    *   Polymarket 每个市场的 OrderBook Snapshot (每 5-10 秒一次)。
    *   Polymarket 的最终决议结果 (Resolution)。
*   **存储**: 使用 SQLite 或 CSV 存储，按日期分片。

### 第二阶段：回测引擎 (Backtest Engine)
**目标**: 实现"快速模拟"。
*   **重构**: 将 `api/strategy.go` 中的策略逻辑提取为 `Evaluate(marketState, config) -> decision` 函数 (无副作用，无网络请求)。
*   **实现**: 编写 `Backtester`。
    *   读取历史数据。
    *   虚拟时间轴 (Replay)。
    *   传入不同的参数配置 (`StrategyConfig`)。
    *   计算 PnL（假设按 OrderBook 价格成交）。
*   **性能**: 这个引擎必须极其快速（纯内存计算），能在几秒内跑完一个月的数据。

### 第三阶段：遗传算法 (GA Optimizer)
**目标**: 开始"进化"。
*   **库**: 使用 Go 现成的 GA 库 (如 `github.com/thoj/go-galib` 或自己写简单的)。
*   **基因 (Genome)**: 对应 `StrategyConfig` 中的字段 (`sigmoid_sensitivity`, `trend_window`, `share_price_levels` 等)。
*   **适应度函数 (Fitness Function)**:
    *   输入: 一组参数。
    *   过程: 调用 **回测引擎** 跑完历史数据。
    *   输出: PnL (净利润) - 风险惩罚 (Drawdown) - 交易次数惩罚 (太少不行)。
*   **流程**:
    1.  **随机生成**: 50 组配置。
    2.  **并行评估**: 利用 Go routine 并发跑回测。
    3.  **选择 (Selection)**: 保留 PnL 最高的 10 组作为精英 (Elites)。
    4.  **交叉 (Crossover)**: 精英之间交换参数（例如 A 的 K线参数 + B 的资金管理参数）。
    5.  **变异 (Mutation)**: 随机微调参数 (±5%)。
    6.  **迭代**: 重复 2-5 步，直到收敛。

## 🛠️ 工程代码建议

### 1. 策略逻辑解耦 (Decoupling)

目前 `checkBinanceArbitrage` 混合了 API 调用。建议重构为：

```go
type StrategyInput struct {
    BinanceKline *Kline
    PolyOrderBook *OrderBook
    TimeLeft      float64
}

type StrategyOutput struct {
    Action     string // "BUY", "SKIP"
    Price      float64
    Amount     float64
}

// 纯函数：不涉及网络，只做计算。GA 和 实盘 都可以调用。
func EvaluateStrategy(input StrategyInput, config StrategyConfig) StrategyOutput {
    // ... 具体的 Z-Score 计算逻辑 ...
}
```

### 2. 定义基因结构 (Gene Definition)

```go
type Genome struct {
    SigmoidK      float64 `min:"1.0" max:"3.0"`
    TrendWindow   int     `min:"60" max:"600"`
    Tier1Edge     float64 `min:"0.0" max:"0.1"`
    // ...
}

func (g Genome) ToConfig() StrategyConfig {
    // 映射回配置对象
}
```

### 3. 并发运行器 (Parallel Runner)

利用 Go 强大的并发能力加速 GA：

```go
func RunGeneration(population []Genome) []Result {
    results := make([]Result, len(population))
    var wg sync.WaitGroup
    
    for i, gene := range population {
        wg.Add(1)
        go func(idx int, g Genome) {
            defer wg.Done()
            // 独立的 Backtester 实例，避免数据竞争
            engine := NewBacktester(historyData) 
            results[idx] = engine.Run(g.ToConfig())
        }(i, gene)
    }
    
    wg.Wait()
    return results
}
```

## 🚀 立即可以做的事

如果您现在没有历史数据，GA 跑不起来。建议先做 **第一阶段**：
1.  **数据落盘**: 确保现在的程序由于实盘运行，顺便把所有用到的 K线和 OrderBook 存下来 (Log 或 CSV)。
2.  **手动搜集**: 如果有以前的 Log，可以写脚本解析 Log 还原出历史数据 ("清洗数据")。

您觉得这个方向可行吗？如果需要，我们可以先从 "解耦策略逻辑" 开始。


# 遗传算法优化框架 - 实施计划
基于 本文上方 的设计，本计划分 3 个阶段实现。

## 第一阶段：策略逻辑解耦 (Strategy Decoupling) ✅ 已完成
目标: 将核心策略决策逻辑提取为纯函数，使其可被回测引擎和实盘系统共同调用。

### 提议的代码变更
[NEW] api/strategy_engine.go
创建新文件，包含：

1. StrategyInput: 策略输入结构体 (K线、价格、剩余时间)
2. StrategyDecision: 策略输出结构体 (Action, Side, Price, Amount, ZScore, FairPrice)
3. EvaluateStrategy(input, config) -> decision: 纯函数，执行 Z-Score 计算和决策逻辑
4. 将 CalculateSigmoidFairPrice 和 getVolatilityFactor 移动到此文件

[MODIFY] api/strategy.go

- 简化 checkBinanceArbitrage，让其调用新的 EvaluateStrategy 函数
- 原函数仅负责：获取实时数据 + 调用 EvaluateStrategy + 执行下单

### 第一阶段验证计划
单元测试 (自动化)
新建 api/strategy_engine_test.go，测试：

- EvaluateStrategy 在给定输入下是否返回正确的 BUY / SKIP 决策
- CalculateSigmoidFairPrice 对边界值（时间=0, 时间=900）的计算是否正确
- Z-Score tier 判断逻辑

```bash
# 运行测试
cd d:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool
go test ./api/... -v -run TestEvaluateStrategy
```

### 手动验证
1. 启动应用 (go run main.go)，观察日志中策略决策输出是否与重构前一致。

## 第二阶段：数据记录器 (Data Recorder) ✅ 已完成
目标: 在实盘运行时，将 K线和 OrderBook 数据落盘到 SQLite，为回测提供素材。

### 提议的代码变更
[NEW] internal/datarecorder/recorder.go
创建历史数据记录服务：

1. KlineRecord: K线数据结构 (Symbol, Open, High, Low, Close, Time)
2. OrderBookRecord: OrderBook 快照结构 (TokenID, BestBid, BestAsk, Time)
3. MarketResolutionRecord: 决议结果 (Slug, Winner, Time)
4. RecordKline(), RecordOrderBook(), RecordResolution() 写入函数
5. LoadHistory(startTime, endTime): 读取历史数据函数

[MODIFY] src/binance_kline.go
- 在收到 K线更新时，调用 datarecorder.RecordKline() 保存数据

[MODIFY] 
src/realtime/manager.go(或订阅回调处)
- 在收到 OrderBook 更新时，调用 datarecorder.RecordOrderBook() 保存快照

### 第二阶段验证计划
手动验证
1. 启动应用，运行 15+ 分钟。
2. 检查 data/history.db 是否生成，查询是否有 K线和 OrderBook 记录。

```
sqlite3 data/history.db "SELECT COUNT(*) FROM klines; SELECT COUNT(*) FROM orderbooks;"
```

## 第三阶段：回测引擎 + 遗传算法 (Backtest + GA) ✅ 已完成
目标: 创建独立的回测命令，以及 GA 优化器。

提议的代码变更
[NEW] tools/backtest/backtest.go
回测引擎：

1. 加载历史数据 (datarecorder.LoadHistory)
2. 虚拟时间轴回放
3. 调用 EvaluateStrategy 做决策
4. 模拟成交，计算 PnL

[NEW] tools/ga/optimizer.go
遗传算法优化器：

1. Genome: 基因结构体，映射 StrategyConfig 核心参数
2. RandomGenome(): 随机生成基因
3. Crossover(a, b Genome): 交叉
4. Mutate(g Genome): 变异
5. Fitness(g Genome): 适应度函数 (调用回测引擎)
6. RunGA(populationSize, generations): 主循环

[NEW] cmd/backtest/main.go
命令行入口：
```bash
go run cmd/backtest/main.go --data data/history.db --config strategy_config.yaml
```
[NEW] cmd/ga/main.go
命令行入口：
```bash
go run cmd/ga/main.go --data data/history.db --population 50 --generations 100
```

### 第三阶段验证计划
回测引擎验证
1. 确保第二阶段有历史数据
2. 运行回测命令，检查 PnL 输出是否合理
```bash
go run cmd/backtest/main.go --data data/history.db --config strategy_config.yaml
```
GA 优化器验证
1. 运行 GA 命令，观察每代 (Generation) 的最佳适应度是否提升
```bash
go run cmd/ga/main.go --data data/history.db --population 20 --generations 10
```
### 实施顺序建议

| 阶段 | 文件 | 预计工作量
| 1.1 | api/strategy_engine.go | 中 |
1.2 | api/strategy.go 重构 | 低 |
1.3 | api/strategy_engine_test.go | 低 |
2.1 | internal/datarecorder/recorder.go | 中 |
2.2 | 集成到 Binance/Realtime 模块 | 低 |
3.1 | tools/backtest/backtest.go | 中 |
3.2 | tools/ga/optimizer.go | 高
3.3 | cmd/backtest/main.go, cmd/ga/main.go | 低 |

> 建议: 先完成第一阶段并验证，再开始第二阶段。这样可以确保基础稳固。