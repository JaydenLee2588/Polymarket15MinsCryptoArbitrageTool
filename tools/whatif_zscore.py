import re
import sys

log_path = r'd:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\20260123\output_20260123_Sim_2.log'

# 2026/01/23 20:30:16 ✅ [模拟][对照1_微敏门槛] Leg1买入成功 | ... | 市场=btc-updown-15m-1769200200 | ... | Z=0.65 |
entry_pattern = re.compile(r'✅ \[模拟\]\[对照1_微敏门槛\] Leg1买入成功 .* 市场=(.*?) .* Z=([\d\.-]+)')

# 2026/01/23 20:45:54 ✅ [对照1_微敏门槛][btc-updown-15m-1769200200] 决议: DOWN, 持仓: DOWN, PnL: $0.27
settle_pattern = re.compile(r'✅ \[对照1_微敏门槛\]\[(.*?)\] 决议: .* PnL: \$([\d\.-]+)')

market_z = {} # slug -> z
results = [] # list of (z, pnl)

with open(log_path, 'r', encoding='utf-8') as f:
    for line in f:
        m_entry = entry_pattern.search(line)
        if m_entry:
            slug, z = m_entry.groups()
            market_z[slug] = float(z)
        
        m_settle = settle_pattern.search(line)
        if m_settle:
            slug, pnl = m_settle.groups()
            if slug in market_z:
                results.append((market_z[slug], float(pnl)))

thresholds = [0.0, 0.6, 0.8, 1.0, 1.2, 1.4, 1.6, 1.8, 2.0]
print(f"{'Z-Threshold':<12} | {'Trades':<8} | {'Total PnL':<10}")
print("-" * 35)

for t in thresholds:
    filtered = [r[1] for r in results if r[0] >= t]
    print(f"Z >= {t:<8.1f} | {len(filtered):<8} | ${sum(filtered):<10.2f}")

print("\n--- Raw Data (Sorted by Z) ---")
for z, pnl in sorted(results, key=lambda x: x[0]):
    print(f"Z={z:<5.2f} -> PnL=${pnl:<6.2f}")
