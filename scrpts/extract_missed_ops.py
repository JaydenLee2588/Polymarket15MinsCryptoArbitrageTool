import re
import csv
import os

# Configuration
LOG_FILE = r"d:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\output_20260123.log"
OUTPUT_FILE = r"d:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\missed_opportunities_20260123.csv"

# Pattern
# 2026/01/22 19:32:48 🔍 [基准1号][sol-updown-15m-1769110200] 趋势满足(UP: 0.2262%) 但优势不足: Z=0.84 (Sigmoid价=0.778), 市场价=0.800, 优势=-0.022 (要求=0.10), 剩余732秒
log_pattern = re.compile(
    r"(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}).*?\[(.*?)\]\[([\w-]+)\] 趋势满足\((UP|DOWN): ([\d\.\-]+)%\) 但优势不足: Z=([\d\.\-]+) \(Sigmoid价=([\d\.]+)\), 市场价=([\d\.]+), 优势=([\d\.\-]+) \(要求=([\d\.]+)\), 剩余(\d+)秒"
)

def main():
    if not os.path.exists(LOG_FILE):
        print(f"Log file not found: {LOG_FILE}")
        return

    data = []
    print(f"Scanning {LOG_FILE}...")
    
    with open(LOG_FILE, 'r', encoding='utf-8') as f:
        for line in f:
            if "趋势满足" in line and "但优势不足" in line:
                match = log_pattern.search(line)
                if match:
                    data.append({
                        "Timestamp": match.group(1),
                        "Strategy": match.group(2),
                        "Market": match.group(3),
                        "Direction": match.group(4),
                        "Trend (%)": match.group(5),
                        "Z-Score": match.group(6),
                        "Fair Price": match.group(7),
                        "Market Price": match.group(8),
                        "Edge": match.group(9),
                        "Required Edge": match.group(10),
                        "Remaining (s)": match.group(11)
                    })

    if not data:
        print("No matching records found.")
        return

    print(f"Found {len(data)} records. Writing to CSV...")
    
    # Write to CSV
    keys = data[0].keys()
    with open(OUTPUT_FILE, 'w', newline='', encoding='utf-8-sig') as f:
        writer = csv.DictWriter(f, fieldnames=keys)
        writer.writeheader()
        writer.writerows(data)
        
    print(f"Done! Exported to {OUTPUT_FILE}")

if __name__ == "__main__":
    main()
