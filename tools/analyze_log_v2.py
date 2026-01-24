import re
import sys
from collections import defaultdict

def analyze_log(log_path):
    # Data structure: group -> list of PnL values
    group_pnl = defaultdict(list)
    
    # Regex: Look for Group, Market (optional), and PnL
    # Matches: ... [GroupID][MarketID] 决议: ..., PnL: $1.23
    # Matches: ... [GroupID] 决议: ..., PnL: $1.23
    regex = re.compile(r'\[(.*?)\](?:\[.*?\])? 决议:.*?, PnL: \$([-0-9\.]+)')

    try:
        with open(log_path, 'r', encoding='utf-8') as f:
            for line in f:
                match = regex.search(line)
                if match:
                    group_id = match.group(1)
                    try:
                        val = float(match.group(2))
                        group_pnl[group_id].append(val)
                    except ValueError:
                        pass
    except Exception as e:
        print(f"Error reading file: {e}")
        return

    # Print Table
    print(f"{'Group ID':<25} | {'Cnt':<3} | {'PnL':<8} | {'Win%':<5} | {'Avg':<5} | {'Best':<5} | {'Worst':<5}")
    print("-" * 85)

    # Sort by Total PnL desc
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
    if len(sys.argv) < 2:
        print("Usage: python analyze.py <file>")
    else:
        analyze_log(sys.argv[1])
