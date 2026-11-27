# 初步架构基线
固定审计版本 f7cfa2e0e8adac782ff2c55efc9e77c40e3e1d84。已读取文件树及核心模块；未执行动态验证。Go 1.24.1（go.mod）；SQLite、WebSocket、HTTP、YAML、Telegram、Ethereum 签名及 Python 辅助分析。
- main.go：启动、环境变量、SQLite、HTTP、TUI、Telegram、Pattern 生命周期。
- api/strategy.go：市场与策略编排；EvaluateStrategy 调用见约 L1523。
- api/strategy_engine.go：策略纯计算入口 EvaluateStrategy；api/strategy_engine_test.go 已有相关测试。
- api/pattern_strategy.go、src/sniper.go：形态及狙击策略。
- src/binance_ws.go、src/binance_rest.go、src/realtime/manager.go：实时行情、历史行情及订单簿。
- src/simulator.go、src/clob_order.go：模拟及真实订单相关能力，具体运行时路径待验证。
- internal/database、internal/datarecorder、internal/backtest：存储、记录、回测。
逻辑数据流：行情 → 缓存与状态 → 指标/策略 → 模拟或订单执行 → 持久化与观测。此图式不等同已验证调用链。
