
import re
import os
from collections import defaultdict

LOG_FILE = r"logs/20260125/output_sim.log"
OUTPUT_FILE = r"logs/20260125/price_granularity_report_full.txt"

def parse_log(file_path):
    print(f"Reading log file: {file_path}")
    
    trades = {}
    resolutions = {}

    with open(file_path, 'r', encoding='utf-8') as f:
        for line in f:
            if "Leg1买入成功" in line:
                header_match = re.search(r"✅ \[(.*?)\]\[(.*?)\] Leg1买入成功", line)
                if not header_match:
                    continue
                
                strategy = header_match.group(2)
                
                slug_match = re.search(r"市场=(.*?) \|", line)
                price_match = re.search(r"价格=([\d\.]+)", line)
                
                if slug_match and price_match:
                    slug = slug_match.group(1)
                    try:
                        price = float(price_match.group(1))
                        key = (strategy, slug)
                        trades[key] = {
                            "price": price,
                            "strategy": strategy
                        }
                    except:
                        pass
            
            elif "决议:" in line and "PnL:" in line:
                res_match = re.search(r"✅ \[(.*?)\]\[(.*?)\] 决议: .*? PnL: \$([\d\.\-]+)", line)
                if res_match:
                    strategy = res_match.group(1)
                    slug = res_match.group(2)
                    pnl_str = res_match.group(3)
                    
                    try:
                        pnl = float(pnl_str)
                        key = (strategy, slug)
                        resolutions[key] = {"pnl": pnl}
                    except:
                        pass

    price_stats = defaultdict(lambda: {'count': 0, 'wins': 0, 'pnl': 0.0})

    for key, trade in trades.items():
        if key in resolutions:
            res = resolutions[key]
            price = trade['price']
            pnl = res['pnl']
            
            bucket = f"{price:.2f}"
            
            price_stats[bucket]['count'] += 1
            price_stats[bucket]['pnl'] += pnl
            if pnl > 0:
                price_stats[bucket]['wins'] += 1
                
    return price_stats

def main():
    if not os.path.exists(LOG_FILE):
        return
        
    stats = parse_log(LOG_FILE)
    
    lines = []
    lines.append("\nPrice Analysis (Granularity: 0.01)\n")
    lines.append(f"{'Price':<8} | {'Win Rate':<8} | {'Total PnL':<10} | {'Avg PnL':<8} | {'Trades':<6}")
    lines.append("-" * 60)
    
    total_trades = 0
    total_pnl = 0.0
    total_wins = 0
    
    for price_str in sorted(stats.keys(), key=lambda x: float(x)):
        d = stats[price_str]
        count = d['count']
        wins = d['wins']
        pnl = d['pnl']
        
        wr = (wins / count * 100) if count > 0 else 0
        avg = pnl / count if count > 0 else 0
        
        lines.append(f"{price_str:<8} | {wr:5.1f}% | {pnl:10.2f} | {avg:8.2f} | {count:<6}")
        
        total_trades += count
        total_pnl += pnl
        total_wins += wins
        
    lines.append("-" * 60)
    overall_wr = (total_wins/total_trades*100) if total_trades else 0
    lines.append(f"{'TOTAL':<8} | {overall_wr:5.1f}% | {total_pnl:10.2f} | {(total_pnl/total_trades if total_trades else 0):8.2f} | {total_trades:<6}")

    with open(OUTPUT_FILE, 'w', encoding='utf-8') as f:
        f.write("\n".join(lines))
    
    print(f"Report saved to {OUTPUT_FILE}")

if __name__ == "__main__":
    main()
