
import re
import os
from collections import defaultdict
from datetime import datetime, timezone

LOG_FILE = r"logs/20260125/output_sim.log"
TARGET_MINUTE = 2

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
                    key = (strategy, slug)
                    trades[key] = {
                        "time_str": log_time_str,
                        "slug": slug,
                        "strategy": strategy
                    }
            
            elif "决议:" in line and "PnL:" in line:
                r_match = re.search(res_body_pattern, line)
                if r_match:
                    strategy = r_match.group(1)
                    slug = r_match.group(2)
                    pnl_str = r_match.group(4)
                    try:
                        pnl = float(pnl_str)
                    except:
                        pnl = 0.0
                    key = (strategy, slug)
                    resolutions[key] = {"pnl": pnl}

    analysis_data = defaultdict(lambda: {'count': 0, 'wins': 0, 'pnl': 0.0})

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
                
                if minute_idx == TARGET_MINUTE:
                    strat = trade['strategy']
                    pnl = res['pnl']
                    
                    analysis_data[strat]['count'] += 1
                    analysis_data[strat]['pnl'] += pnl
                    if pnl > 0:
                        analysis_data[strat]['wins'] += 1
                        
            except Exception as e:
                pass
                
    return analysis_data

def main():
    if not os.path.exists(LOG_FILE):
        print(f"Log file not found: {LOG_FILE}")
        return
        
    data = parse_log(LOG_FILE)
    
    print(f"\nAnalysis for Minute {TARGET_MINUTE} (120s - 180s):\n")
    print(f"{'Strategy':<30} | {'Count':<5} | {'Win Rate':<8} | {'PnL ($)':<10}")
    print("-" * 65)
    
    total_count = 0
    total_pnl = 0.0
    
    sorted_items = sorted(data.items(), key=lambda x: x[1]['pnl'], reverse=True)
    
    for strat, stats in sorted_items:
        count = stats['count']
        wins = stats['wins']
        pnl = stats['pnl']
        wr = (wins/count*100) if count > 0 else 0
        
        print(f"{strat:<30} | {count:<5} | {wr:5.1f}% | {pnl:10.2f}")
        
        total_count += count
        total_pnl += pnl
        
    print("-" * 65)
    print(f"{'TOTAL':<30} | {total_count:<5} | {'N/A':<8} | {total_pnl:10.2f}")

if __name__ == "__main__":
    main()
