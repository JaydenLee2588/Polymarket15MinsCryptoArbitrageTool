# 学习路线 V2.0
| 阶段 | 内容 | 建议学习日 |
|---|---|---|
| M0 | 系统认知、构建、测试、安全隔离、初始基线 | 3–5 |
| M1 | Go Runtime、GMP、Goroutine、Channel、Context | 4–6 |
| M2 | testing.B、benchstat、延迟分布、吞吐量 | 4–5 |
| M3 | 依赖图、复杂度、职责、可测试性、go vet | 3–5 |
| M4 | pprof、CPU、Heap、Execution Trace | 4–6 |
| M5 | Race Detector、Mutex、背压、共享状态 | 5–7 |
| M6 | WebSocket、HTTP、重连、缓存一致性 | 5–7 |
| M7 | 逃逸分析、分配、GC、对象生命周期 | 4–6 |
| M8 | Hot/Cold Path、故障隔离、状态恢复、故障注入 | 5–8 |
| M9 | 全量回归、性能对比、ADR、总结 | 3–5 |

M0 Day 1 环境与安全；Day 2 结构依赖；Day 3 测试覆盖；Day 4 初始测量；Day 5 基线验收。以上为弹性安排。M0 不优化业务代码。
M0 验收：固定基线；证明离线隔离；可复现构建；模块依赖与主要执行路径；实际测试结果；初始性能指标与方法；风险登记；基线报告。不得以缺失数据冒充通过。
