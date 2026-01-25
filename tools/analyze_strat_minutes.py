
import re
import os
from collections import defaultdict
from datetime import datetime, timezone

LOG_FILE = r"logs/20260125/output_sim.log"
TARGET_STRATEGY = "全面对比10-强趋势突破"

def parse_log(file_path):
    print(f"Reading log file: {file_path}")
    
    trades = {}
    resolutions = {}

    prefix_pattern = r"^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})"
    trade_body_pattern = r"✅ \[(.*?)\]\[(.*?)\] Leg1买入成功 \| .*? \| 市场=(.*?) \|"
    res_body_pattern = r"✅ \[(.*?)\]\[(.*?)\] 决议: (.*?), .*? PnL: \$(.*)"

    with open(file_path, 'r', encoding='utf-8') as f:
        for line in f:
            ts_match = re.match(prefix_pattern, line)
            if not ts_match:
                continue
            log_time_str = ts_match.group(1)
            
            if "Leg1买入成功" in line:
                t_match = re.search(trade_body_pattern, line)
                if t_match:
                    strategy = t_match.group(2)
                    slug = t_match.group(3)
                    
                    if strategy != TARGET_STRATEGY:
                        continue

                    trades[(strategy, slug)] = {
                        "time_str": log_time_str,
                        "slug": slug,
                        "strategy": strategy
                    }
            
            elif "决议:" in line and "PnL:" in line:
                r_match = re.search(res_body_pattern, line)
                if r_match:
                    strategy = r_match.group(1)
                    slug = r_match.group(2)
                    
                    if strategy != TARGET_STRATEGY:
                        continue

                    try:
                        pnl = float(r_match.group(4))
                    except:
                        pnl = 0.0
                    
                    resolutions[(strategy, slug)] = {"pnl": pnl}

    minute_stats = defaultdict(lambda: {'count': 0, 'wins': 0, 'pnl': 0.0})

    for key, trade in trades.items():
        if key in resolutions:
            res = resolutions[key]
            try:
                log_dt = datetime.strptime(trade['time_str'], "%Y/%m/%d %H:%M:%S")
                log_ts = log_dt.replace(tzinfo=timezone.utc).timestamp()
                
                parts = trade['slug'].split('-')
                start_ts = int(parts[-1])
                
                diff_sec = log_ts - start_ts
                minute_idx = int(diff_sec // 60)
                
                pnl = res['pnl']
                minute_stats[minute_idx]['count'] += 1
                minute_stats[minute_idx]['pnl'] += pnl
                if pnl > 0:
                    minute_stats[minute_idx]['wins'] += 1
                        
            except Exception as e:
                pass
                
    return minute_stats

def main():
    if not os.path.exists(LOG_FILE):
        print(f"Log file not found: {LOG_FILE}")
        return
        
    stats = parse_log(LOG_FILE)
    
    print(f"\nAnalysis for Strategy: {TARGET_STRATEGY}\n")
    print(f"{'Minute':<8} | {'Win Rate':<8} | {'PnL ($)':<10} | {'Avg PnL':<8} | {'Trades':<6}")
    print("-" * 60)
    
    total_trades = 0
    total_wins = 0
    total_pnl = 0.0
    
    for m in sorted(stats.keys()):
        d = stats[m]
        count = d['count']
        wins = d['wins']
        pnl = d['pnl']
        
        wr = (wins / count * 100) if count > 0 else 0
        avg = pnl / count if count > 0 else 0
        
        label = str(m)
        if m < 0: label = f"Pre{m}"
        elif m > 15: label = f"Post{m}"
        
        print(f"{label:<8} | {wr:5.1f}% | {pnl:10.2f} | {avg:8.2f} | {count:<6}")
        
        total_trades += count
        total_pnl += pnl
        total_wins += wins
        
    print("-" * 60)
    final_wr = (total_wins/total_trades*100) if total_trades else 0
    print(f"{'TOTAL':<8} | {final_wr:5.1f}% | {total_pnl:10.2f} | {(total_pnl/total_trades if total_trades else 0):8.2f} | {total_trades:<6}")

if __name__ == "__main__":
    main()
