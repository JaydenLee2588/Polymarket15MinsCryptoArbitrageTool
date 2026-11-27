# 初步风险登记（非动态确认）
基线 f7cfa2e0e8adac782ff2c55efc9e77c40e3e1d84；所有路径相对该版本。
| ID | 观察 | 证据 | 后续 |
|---|---|---|---|
| F01 | 数据管理访问控制需优先核查（安全细节不在公开文档展开） | 私下审计记录 | 隔离临时数据库验证；核查部署边界；负责人审查 |
| F02 | Binance WS 合约、REST 现货，文档描述现货 | src/binance_ws.go、src/binance_rest.go、docs/project_comprehensive_intro.md | 追踪数据流及混用条件 |
| F03 | 策略编排文件职责较多 | api/strategy.go（约 2318 行）、api/pattern_strategy.go（约 1736 行） | 复杂度、耦合及可测试性分析；不以行数判定性能 |
| F04 | 共享状态与锁、消息通道 | src/realtime/manager.go 等 | Race Detector、Mutex/Block Profile |
| F05 | Data Recorder 初始化/关闭被注释 | main.go L85–90 | 查找其他初始化路径 |
| F06 | Pattern 延迟启动依赖固定等待 | main.go | 核查就绪、取消、退出与资源释放 |
所有风险均需证据复核；不自动修复。尚无实测性能瓶颈。
