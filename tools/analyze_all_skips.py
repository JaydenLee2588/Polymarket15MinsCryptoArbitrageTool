import re

log_path = r'd:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\20260123\output_20260123_Real_2.log'

# Case 1: 趋势满足 但是优势不足 (We didn't buy)
sig_met_pattern = re.compile(r'🔍 \[(.*?)\]\[(.*?)\] 趋势满足\((UP|DOWN):.*?%\) 但优势不足')

# Case 2: 超过阈值 (We didn't buy)
price_cap_pattern = re.compile(r'⏭️ \[(.*?)\]\[(.*?)\] (UP|DOWN) 价格 .* 超过阈值')

# Case 3: 购买成功 (We DID buy)
bought_pattern = re.compile(r'✅ \[💰真实\]\[(.*?)\] Leg1买入成功 .* 市场=(.*?) \| 方向=(UP|DOWN)')

# Case 4: Resolution (The ground truth)
res_pattern = re.compile(r'✅ \[(.*?)\]\[(.*?)\] 决议: (UP|DOWN)')

signals = {} # slug -> {'type': 'edge/price', 'side': 'UP', 'grp': ''}
resolutions = {} # slug -> side

# Secondary check: Bought signals
bought = set()

with open(log_path, 'r', encoding='utf-8') as f:
    for line in f:
        # Check Signals (The ones we skipped)
        m_e = sig_met_pattern.search(line)
        if m_e:
            grp, slug, side = m_e.groups()
            signals[slug] = {'type': 'edge', 'side': side, 'grp': grp}
            
        m_p = price_cap_pattern.search(line)
        if m_p:
            grp, slug, side = m_p.groups()
            signals[slug] = {'type': 'price_cap', 'side': side, 'grp': grp}
            
        # Check Bought (To exclude from skipped analysis)
        m_b = bought_pattern.search(line)
        if m_b:
            grp, slug, side = m_b.groups()
            bought.add(slug)
            
        # Check Resolutions
        m_r = res_pattern.search(line)
        if m_r:
            grp, slug, outcome = m_r.groups()
            resolutions[slug] = outcome

# Analyze SKIPPED signals that were resolved
edge_wins = 0
edge_total = 0
cap_wins = 0
cap_total = 0

print(f"{'Slug':<30} | {'Type':<10} | {'Signal':<6} | {'Outcome':<8} | {'Win?'}")
print("-" * 80)

for slug, sig in signals.items():
    if slug in bought: continue # Skip ones we actually bought
    
    if slug in resolutions:
        outcome = resolutions[slug]
        is_win = (sig['side'] == outcome)
        
        if sig['type'] == 'edge':
            edge_total += 1
            if is_win: edge_wins += 1
        else:
            cap_total += 1
            if is_win: cap_wins += 1
            
        print(f"{slug:<30} | {sig['type']:<10} | {sig['side']:<6} | {outcome:<8} | {'✅' if is_win else '❌'}")

print("-" * 80)
print(f"Edge Restricted: Found {edge_total} cases, Wins: {edge_wins}, Win Rate: {edge_wins/edge_total*100:.1f}%" if edge_total > 0 else "No edge-restricted cases found.")
print(f"Price Cap Restricted: Found {cap_total} cases, Wins: {cap_wins}, Win Rate: {cap_wins/cap_total*100:.1f}%" if cap_total > 0 else "No cap-restricted cases found.")
