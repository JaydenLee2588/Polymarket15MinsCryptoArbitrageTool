
import re
import os
from collections import defaultdict
from datetime import datetime, timezone

LOG_FILE = r"logs/20260125/output_sim.log"
REPORT_FILE = r"logs/20260125/minute_analysis_report.md"

def parse_log(file_path):
    print(f"Reading log file: {file_path}")
    
    # Store trades: key = (strategy, slug), value = {trade_time, ...}
    trades = {} 
    # Store resolutions: key = (strategy, slug), value = {pnl, result}
    resolutions = {}

    # Regex patterns
    # Line prefix: 2026/01/25 02:24:12 ...
    prefix_pattern = r"^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})"
    
    # Trade pattern
    # ✅ [模拟][全面对比4-微利刷单] Leg1买入成功 | 时间=02:24:12 | 市场=sol-updown-15m-1769307300 | ...
    trade_body_pattern = r"✅ \[(.*?)\]\[(.*?)\] Leg1买入成功 \| .*? \| 市场=(.*?) \|"
    
    # Resolution pattern
    # ✅ [全面对比4-微利刷单][eth-updown-15m-1769307300] 决议: DOWN, 持仓: DOWN, PnL: $2.60
    # Note: Sometimes the strategy name might have brackets or special chars, so non-greedy matching is important.
    # Pattern: ✅ [Strategy][Slug] 决议: ..., PnL: $Value
    res_body_pattern = r"✅ \[(.*?)\]\[(.*?)\] 决议: (.*?), .*? PnL: \$(.*)"

    with open(file_path, 'r', encoding='utf-8') as f:
        for line in f:
            # Extract timestamp
            ts_match = re.match(prefix_pattern, line)
            if not ts_match:
                continue
            log_time_str = ts_match.group(1)
            
            # Check for Trade
            if "Leg1买入成功" in line:
                t_match = re.search(trade_body_pattern, line)
                if t_match:
                    # mode = t_match.group(1) # e.g. 模拟
                    strategy = t_match.group(2)
                    slug = t_match.group(3)
                    
                    # Store trade info
                    key = (strategy, slug)
                    # Assuming only one trade per strategy per market for simplicity of PnL matching
                    # If multiple, this will overwrite, but resolution usually matches the aggregate or last one.
                    trades[key] = {
                        "time_str": log_time_str,
                        "slug": slug,
                        "strategy": strategy
                    }
            
            # Check for Resolution
            elif "决议:" in line and "PnL:" in line:
                r_match = re.search(res_body_pattern, line)
                if r_match:
                    strategy = r_match.group(1)
                    slug = r_match.group(2)
                    resolution = r_match.group(3)
                    pnl_str = r_match.group(4)
                    
                    try:
                        pnl = float(pnl_str)
                    except:
                        pnl = 0.0
                        
                    key = (strategy, slug)
                    resolutions[key] = {
                        "pnl": pnl,
                        "resolution": resolution
                    }

    # Merge data
    merged_data = []
    
    for key, trade in trades.items():
        if key in resolutions:
            res = resolutions[key]
            
            # Parse times
            # Log time format: 2026/01/25 02:24:12
            # We assume this is effectively UTC or consistent with the slug timestamp
            try:
                log_dt = datetime.strptime(trade['time_str'], "%Y/%m/%d %H:%M:%S")
                # Assume log time is already in appropriate timezone (or UTC)
                # We need to treat it as UTC to match the slug timestamp which is Unix Epoch
                log_ts = log_dt.replace(tzinfo=timezone.utc).timestamp()
                
                # Parse Slug for Start Time
                # Format: symbol-updown-15m-TIMESTAMP
                parts = trade['slug'].split('-')
                start_ts = int(parts[-1])
                
                # Calculate minute offset
                # 15m market = 900 seconds
                diff_sec = log_ts - start_ts
                minute_idx = int(diff_sec // 60)
                
                merged_data.append({
                    "minute": minute_idx,
                    "pnl": res['pnl'],
                    "strategy": trade['strategy'],
                    "slug": trade['slug']
                })
            except Exception as e:
                print(f"Error parsing trade {key}: {e}")
                
    return merged_data

def analyze_minutes(data):
    # Bucket by minute
    buckets = defaultdict(lambda: {'pnl': 0.0, 'wins': 0, 'total': 0})
    
    for row in data:
        m = row['minute']
        pnl = row['pnl']
        
        buckets[m]['pnl'] += pnl
        buckets[m]['total'] += 1
        if pnl > 0:
            buckets[m]['wins'] += 1
            
    return buckets

def generate_report(buckets):
    sorted_minutes = sorted(buckets.keys())
    
    lines = []
    lines.append("# Minute-by-Minute Analysis (15m Market)")
    lines.append("")
    lines.append("| Minute | Win Rate | Total PnL ($) | Avg PnL ($) | Trades |")
    lines.append("|---|---|---|---|---|")
    
    total_trades = 0
    total_pnl = 0.0
    total_wins = 0
    
    for m in sorted_minutes:
        stats = buckets[m]
        count = stats['total']
        wins = stats['wins']
        pnl = stats['pnl']
        
        win_rate = (wins / count * 100) if count > 0 else 0
        avg_pnl = (pnl / count) if count > 0 else 0
        
        # Format minute label
        label = f"{m}"
        if m < 0:
            label = f"Pre ({m})"
        elif m >= 15:
            label = f"Post ({m})"
            
        lines.append(f"| {label} | {win_rate:.1f}% | {pnl:.2f} | {avg_pnl:.3f} | {count} |")
        
        total_trades += count
        total_pnl += pnl
        total_wins += wins
        
    lines.append("")
    total_wr = (total_wins / total_trades * 100) if total_trades > 0 else 0
    lines.append(f"**Overall**: {total_wr:.1f}% Win Rate, ${total_pnl:.2f} PnL, {total_trades} Trades")
    
    return "\n".join(lines)

def main():
    if not os.path.exists(LOG_FILE):
        print(f"Log file not found: {LOG_FILE}")
        return
        
    data = parse_log(LOG_FILE)
    if not data:
        print("No trade/resolution pairs found.")
        return
        
    buckets = analyze_minutes(data)
    report_md = generate_report(buckets)
    
    with open(REPORT_FILE, 'w', encoding='utf-8') as f:
        f.write(report_md)
        
    print(f"Analysis complete. Report saved to: {REPORT_FILE}")
    print("-" * 30)
    print(report_md)

if __name__ == "__main__":
    main()
