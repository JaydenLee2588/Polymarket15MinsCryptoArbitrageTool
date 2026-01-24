import re

log_path = r'd:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\20260123\output_20260123_Real_2.log'

edge_pattern = re.compile(r'🔍 \[(.*?)\]\[(.*?)\]') # Simplified
res_pattern = re.compile(r'✅ \[(.*?)\]\[(.*?)\] 决议')

signals = set()
resolutions = set()

with open(log_path, 'r', encoding='utf-8') as f:
    for line in f:
        m_e = edge_pattern.search(line)
        if m_e: signals.add(m_e.group(2))
        
        m_r = res_pattern.search(line)
        if m_r: resolutions.add(m_r.group(2))

print(f"Signal Slugs: {sorted(list(signals))[:5]}")
print(f"Res Slugs: {sorted(list(resolutions))[:5]}")
common = signals.intersection(resolutions)
print(f"Common: {len(common)}")
