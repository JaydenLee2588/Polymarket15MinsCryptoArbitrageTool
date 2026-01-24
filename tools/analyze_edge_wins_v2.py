import re

log_path = r'd:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\20260123\output_20260123_Real_2.log'

# 🔍 [基准1号][slug] 趋势满足...
edge_pattern = re.compile(r'🔍 \[(.*?)\]\[(.*?)\] 趋势满足\((UP|DOWN):.*?%\) 但优势不足: Z=([\d\.-]+) \(Sigmoid价=([\d\.-]+)\), 市场价=([\d\.-]+), 优势=([\d\.-]+)')

# ✅ [基准1号][slug] 决议: UP
res_pattern = re.compile(r'✅ \[(.*?)\]\[(.*?)\] 决议: (UP|DOWN)')

signals = []
resolutions = {}

with open(log_path, 'r', encoding='utf-8') as f:
    for line in f:
        # Check for Signal (Rejection)
        m_edge = edge_pattern.search(line)
        if m_edge:
            grp, slug, side, z, fair, mkt, edge = m_edge.groups()
            signals.append({
                'slug': slug,
                'grp': grp,
                'side': side,
                'price': float(mkt),
                'edge': float(edge),
                'z': float(z)
            })
            
        # Check for Resolution
        m_res = res_pattern.search(line)
        if m_res:
            grp, slug, res_side = m_res.groups()
            resolutions[(grp, slug)] = res_side

results = []
for s in signals:
    key = (s['grp'], s['slug'])
    if key in resolutions:
        outcome = resolutions[key]
        is_win = (s['side'] == outcome)
        results.append({
            'slug': s['slug'],
            'side': s['side'],
            'price': s['price'],
            'edge': s['edge'],
            'outcome': outcome,
            'is_win': is_win
        })

if not results:
    print("No results found. Checking resolutions keys...")
    print(f"Sample resolution keys: {list(resolutions.keys())[:2]}")
    print(f"Sample signal keys: {[(s['grp'], s['slug']) for s in signals[:2]]}")
else:
    wins = sum(1 for r in results if r['is_win'])
    print(f"Matched: {len(results)} | Wins: {wins} | Win Rate: {wins/len(results)*100:.1f}%")
    for r in results:
        print(f"{r['slug']:<30} | {r['side']}/{r['outcome']} | {'✅' if r['is_win'] else '❌'}")
