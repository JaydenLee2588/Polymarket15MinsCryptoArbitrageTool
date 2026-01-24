import sys
from collections import defaultdict

def analyze_log(log_path):
    print(f"Scanning {log_path}...")
    group_pnl = defaultdict(list)
    
    with open(log_path, 'r', encoding='utf-8', errors='ignore') as f:
        for line in f:
            if "决议:" in line and "PnL:" in line:
                # 2026/01/24 06:35:21 ✅ [基准1号][sol-updown-15m-1769235300] 决议: DOWN, 持仓: DOWN, PnL: $1.08
                try:
                    # Find PnL
                    pnl_part = line.split("PnL: $")[1]
                    pnl_val = float(pnl_part.split()[0].strip())
                    
                    # Find Group
                    # Assuming format: ... [Group][Market] ...
                    # Find first '[' and matching ']'
                    start = line.find('[')
                    end = line.find(']', start)
                    if start != -1 and end != -1:
                        group_id = line[start+1:end]
                        # Handling "[模拟][Group]" case if present
                        if "模拟" in group_id:
                            # Try next bracket
                            start = line.find('[', end)
                            end = line.find(']', start)
                            if start != -1 and end != -1:
                                group_id = line[start+1:end]
                        
                        group_pnl[group_id].append(pnl_val)
                except Exception as e:
                    # print(f"Skipping line: {line.strip()} | Error: {e}")
                    pass

    # Print Table
    print(f"{'Group ID':<25} | {'Cnt':<3} | {'PnL':<8} | {'Win%':<5} | {'Avg':<5} | {'Best':<5} | {'Worst':<5}")
    print("-" * 85)

    sorted_groups = sorted(group_pnl.items(), key=lambda x: sum(x[1]), reverse=True)

    for group, diffs in sorted_groups:
        count = len(diffs)
        total = sum(diffs)
        wins = [x for x in diffs if x > 0]
        win_rate = (len(wins) / count * 100) if count > 0 else 0.0
        avg = total / count if count > 0 else 0.0
        best = max(diffs) if diffs else 0.0
        worst = min(diffs) if diffs else 0.0

        print(f"{group:<25} | {count:<3} | ${total:<7.2f} | {win_rate:<4.0f}% | ${avg:<4.2f} | ${best:<4.2f} | ${worst:<4.2f}")

if __name__ == "__main__":
    analyze_log(sys.argv[1])
