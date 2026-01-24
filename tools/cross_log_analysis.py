import re

real_log = r'd:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\20260123\output_20260123_Real_2.log'
sim_log = r'd:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\20260123\output_20260123_Sim_2.log'

# Extract rejected slugs from Real_2
skipped_slugs = {} # slug -> (side, type)

# 🔍 [基准1号][slug] 趋势满足...
edge_pattern = re.compile(r'🔍 \[基准1号\]\[(.*?)\] 趋势满足\((UP|DOWN):.*?%\) 但优势不足')
# ⏭️ [基准1号][slug] (UP|DOWN) 价格...
cap_pattern = re.compile(r'⏭️ \[基准1号\]\[(.*?)\] (UP|DOWN) 价格 .* 超过阈值')

with open(real_log, 'r', encoding='utf-8') as f:
    for line in f:
        me = edge_pattern.search(line)
        if me: skipped_slugs[me.group(1)] = (me.group(2), 'edge')
        
        mp = cap_pattern.search(line)
        if mp: skipped_slugs[mp.group(1)] = (mp.group(2), 'price_cap')

# Find resolutions in Sim_2
# ✅ [对照X][slug] 决议: UP
res_pattern = re.compile(r'✅ \[.*?\]\[(.*?)\] 决议: (UP|DOWN)')
resolutions = {}

with open(sim_log, 'r', encoding='utf-8') as f:
    for line in f:
        mr = res_pattern.search(line)
        if mr:
            resolutions[mr.group(1)] = mr.group(2)

print(f"{'Slug':<30} | {'Type':<10} | {'Signal':<6} | {'Outcome':<8} | {'Win?'}")
print("-" * 80)

wins = 0
total = 0

for slug, (side, t) in skipped_slugs.items():
    if slug in resolutions:
        outcome = resolutions[slug]
        is_win = (side == outcome)
        total += 1
        if is_win: wins += 1
        print(f"{slug:<30} | {t:<10} | {side:<6} | {outcome:<8} | {'✅' if is_win else '❌'}")

print("-" * 80)
print(f"Total Correlated: {total} | Wins: {wins} | Win Rate: {wins/total*100:.1f}%" if total > 0 else "No correlations found in Sim_2.")
