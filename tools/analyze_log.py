import re
import sys
from collections import defaultdict

def analyze_log(log_path):
    stats = defaultdict(lambda: {'trades': 0, 'pnl': 0.0, 'wins': 0, 'losses': 0})
    
    # Regex for PnL lines:
    # 2026/01/23 11:33:54 ✅ [对照6_低波敏感][eth-updown-15m-1769166900] 决议: DOWN, 持仓: DOWN, PnL: $0.27
    pnl_regex = re.compile(r'✅ \[(.*?)\]\[.*?\] 决议: .*, PnL: \$([-0-9\.]+)')
    
    # Regex for Trades (optional, to verify trade counts vs settlements)
    # 2026/01/23 11:31:54 ✅ [模拟][对照7_狂欢] Leg1买入成功 ...
    trade_regex = re.compile(r'✅ \[模拟\]\[(.*?)\] Leg1买入成功')

    with open(log_path, 'r', encoding='utf-8') as f:
        for line in f:
            # Check for PnL (Settlement)
            match = pnl_regex.search(line)
            if match:
                group_id = match.group(1)
                pnl = float(match.group(2))
                stats[group_id]['pnl'] += pnl
                stats[group_id]['trades'] += 1
                if pnl > 0:
                    stats[group_id]['wins'] += 1
                else:
                    stats[group_id]['losses'] += 1
            
            # Note: We rely on settlement logs for PnL. Trade logs just show entry.

    print(f"{'Group ID':<20} | {'Trades':<6} | {'Total PnL':<10} | {'Win Rate':<8} | {'Avg PnL':<8} | {'Avg Win':<8} | {'Avg Loss':<8}")
    print("-" * 100)
    
    sorted_groups = sorted(stats.items(), key=lambda x: x[1]['pnl'], reverse=True)
    
    for group_id, data in sorted_groups:
        trades = data['trades']
        pnl = data['pnl']
        wins = data['wins']
        losses = data['losses']
        
        # Calculate PnL amounts for avg win/loss
        # Since we didn't store individual trade pnl list in 'stats', we need to re-scan or change data structure.
        # Let's change data structure slightly above or just re-read for distribution if needed. 
        # Actually, simpler to just start over with a better structure.
        pass

    # Re-structure for detailed stats
    detailed_stats = defaultdict(lambda: {'pnl_list': []})
    
    # Re-open safely
    with open(log_path, 'r', encoding='utf-8') as f:
        for line in f:
            match = pnl_regex.search(line)
            if match:
                group_id = match.group(1)
                pnl = float(match.group(2))
                detailed_stats[group_id]['pnl_list'].append(pnl)

    for group_id, data in sorted_groups:
        pnl_list = detailed_stats[group_id]['pnl_list']
        trades = len(pnl_list)
        total_pnl = sum(pnl_list)
        
        winning_trades = [x for x in pnl_list if x > 0]
        losing_trades = [x for x in pnl_list if x <= 0]
        
        avg_win = sum(winning_trades) / len(winning_trades) if winning_trades else 0
        avg_loss = sum(losing_trades) / len(losing_trades) if losing_trades else 0
        win_rate = (len(winning_trades) / trades * 100) if trades > 0 else 0
        
        print(f"{group_id:<20} | {trades:<6} | ${total_pnl:<9.2f} | {win_rate:<7.1f}% | ${total_pnl/trades:<7.2f} | ${avg_win:<7.2f} | ${avg_loss:<7.2f}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python analyze_log.py <log_file>")
        sys.exit(1)
    analyze_log(sys.argv[1])
