import re
from collections import defaultdict

log_path = r'd:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\20260123\output_20260123_Real_2.log'

# 🔍 [基准1号][eth-updown-15m-1769218200] 趋势满足(UP: 0.2319%) 但优势不足: Z=2.04 (Sigmoid价=0.988), 市场价=0.871, 优势=0.117 (要求=0.00), 剩余228秒
edge_pattern = re.compile(r'🔍 \[(.*?)\]\[(.*?)\] 趋势满足\((UP|DOWN): .*?%\) 但优势不足: Z=([\d\.-]+) \(Sigmoid价=([\d\.-]+)\), 市场价=([\d\.-]+), 优势=([\d\.-]+)')

# ✅ [基准1号][sol-updown-15m-1769201100] 决议: DOWN, 持仓: DOWN, PnL: $2.34
# Important: We need resolution lines even for markets we didn't hold positions in. 
# Looking at the code, it logs the resolution for all markets it was tracking.
res_pattern = re.compile(r'✅ \[(.*?)\]\[(.*?)\] 决议: (UP|DOWN)')

signals = {} # (grp, slug) -> (side, market_price)
resolutions = {} # (grp, slug) -> side

with open(log_path, 'r', encoding='utf-8') as f:
    for line in f:
        m_edge = edge_pattern.search(line)
        if m_edge:
            grp, slug, side, z, fair, mkt, edge = m_edge.groups()
            signals[(grp, slug)] = (side, float(mkt), float(edge), float(z))
            
        m_res = res_pattern.search(line)
        if m_res:
            grp, slug, res_side = m_res.groups()
            resolutions[(grp, slug)] = res_side

wins = 0
losses = 0
total_potential_pnl = 0.0

print(f"{'Market (Slug)':<30} | {'Signal':<6} | {'Price':<6} | {'Edge':<7} | {'Outcome':<8} | {'Win?'}")
print("-" * 80)

for (grp, slug), (side, price, edge, z) in signals.items():
    if (grp, slug) in resolutions:
        outcome = resolutions[(grp, slug)]
        is_win = (side == outcome)
        if is_win:
            wins += 1
            # Potential PnL estimation: (1.0 - Price) * 10 - Fee if we bought 10 shares
            # But let's just count wins/losses first mentioned by user.
        else:
            losses += 1
        
        print(f"{slug:<30} | {side:<6} | {price:<6.3f} | {edge:<7.3f} | {outcome:<8} | {'✅' if is_win else '❌'}")

print("-" * 80)
print(f"Total Correlated: {wins + losses} | Wins: {wins} | Losses: {losses} | Win Rate: {wins/(wins+losses)*100:.1f}%" if (wins+losses) > 0 else "No correlations found.")
