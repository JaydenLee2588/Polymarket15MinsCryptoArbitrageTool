import re
import csv
import os

log_path = r'd:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\20260123\output_20260123_Real_2.log'
output_path = r'd:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\20260123\rejected_trades_0.83.csv'

# Patterns
# ⏭️ [基准1号][eth-updown-15m-1769200200] DOWN 价格 0.870 超过阈值 0.83，跳过
rejection_pattern = re.compile(r'(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) ⏭️ \[(.*?)\]\[(.*?)\] (UP|DOWN) 价格 ([\d.]+) 超过阈值 ([\d.]+)')
# 🔍 [基准1号][eth-updown-15m-1769218200] 趋势满足(UP: 0.2319%) 但优势不足: Z=2.04 (Sigmoid价=0.988), 市场价=0.871, 优势=0.117 (要求=0.00), 剩余228秒
context_pattern = re.compile(r'🔍 \[(.*?)\]\[(.*?)\] 趋势满足\(.*?: ([\d\.-]+)%\) 但优势不足: Z=([\d\.-]+) \(Sigmoid价=([\d\.-]+)\), 市场价=([\d\.-]+)')

records = []
context_data = {} # (group, slug) -> last seen context

with open(log_path, 'r', encoding='utf-8') as f:
    for line in f:
        # Update context
        ctx_match = context_pattern.search(line)
        if ctx_match:
            grp, slug, trend, z, fair, mkt_p = ctx_match.groups()
            context_data[(grp, slug)] = {
                'trend': trend,
                'z': z,
                'fair': fair
            }
        
        # Check for rejection
        rej_match = rejection_pattern.search(line)
        if rej_match:
            time_str, grp, slug, side, price, threshold = rej_match.groups()
            
            # Lookup context or fill empty
            ctx = context_data.get((grp, slug), {'trend': '', 'z': '', 'fair': ''})
            
            records.append({
                'Timestamp': time_str,
                'Group': grp,
                'Market': slug,
                'Direction': side,
                'Price': price,
                'Threshold': threshold,
                'Z-Score': ctx['z'],
                'FairPrice': ctx['fair'],
                'Trend%': ctx['trend']
            })

# Write to CSV
with open(output_path, 'w', newline='', encoding='utf-8-sig') as f:
    writer = csv.DictWriter(f, fieldnames=['Timestamp', 'Group', 'Market', 'Direction', 'Price', 'Threshold', 'Z-Score', 'FairPrice', 'Trend%'])
    writer.writeheader()
    writer.writerows(records)

print(f"Successfully extracted {len(records)} records to {output_path}")
