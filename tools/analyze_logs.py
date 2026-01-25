import re
import csv
import os
import sys
from collections import defaultdict
from datetime import datetime

# Configuration
LOG_FILE = r"logs/20260125/output_sim.log"
OUTPUT_CSV = "analysis_data.csv"
OUTPUT_MD = "analysis_report_generated.md"

def parse_log(file_path):
    print(f"Reading log file: {file_path}")
    trades = []
    resolutions = []
    
    trade_pattern = re.compile(r"(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) ✅ \[模拟\]\[(.*?)\] Leg1买入成功 \| .*? \| 市场=(.*?) \| 方向=(.*?) \| Z=(.*?) \| 价格=(.*?) \| 数量=(.*?) \| 成本=\$(.*?) \| 趋势=(.*?)% \| 公允价=(.*)")
    res_pattern = re.compile(r"(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) ✅ \[(.*?)\]\[(.*?)\] 决议: (.*?), 持仓: (.*?), PnL: \$(.*)")

    with open(file_path, 'r', encoding='utf-8') as f:
        for line in f:
            t_match = trade_pattern.search(line)
            if t_match:
                trades.append({
                    "trade_time": t_match.group(1),
                    "strategy": t_match.group(2),
                    "market": t_match.group(3),
                    "direction": t_match.group(4),
                    "z_score": float(t_match.group(5)),
                    "price": float(t_match.group(6)),
                    "qty": float(t_match.group(7)),
                    "cost": float(t_match.group(8)),
                    "trend_pct": float(t_match.group(9)),
                    "fair_value": float(t_match.group(10))
                })
                continue

            r_match = res_pattern.search(line)
            if r_match:
                resolutions.append({
                    "res_time": r_match.group(1),
                    "strategy": r_match.group(2),
                    "market": r_match.group(3),
                    "resolution": r_match.group(4),
                    "pnl": float(r_match.group(6))
                })

    # Merge Trades and Resolutions (Naive merge by Strategy + Market)
    # Since simulated trades are sequential per market/strategy in this context, 
    # we can try to match them. But to strictly match, we might need time window.
    # For now, let's create a lookup.
    
    # Key: (Strategy, Market) -> List of resolutions
    res_lookup = defaultdict(list)
    for r in resolutions:
        res_lookup[(r['strategy'], r['market'])].append(r)
    
    # Sort resolutions by time to pop them?
    # Simplification: Assume 1-to-1 or close enough for stats
    
    merged_data = []
    
    # We need to track used resolutions to comply with FIFO?
    # But dictionary is just list.
    
    for t in trades:
        key = (t['strategy'], t['market'])
        candidates = res_lookup[key]
        
        # Find the first resolution that is AFTER the trade time
        matched_res = None
        t_time = datetime.strptime(t['trade_time'], "%Y/%m/%d %H:%M:%S")
        
        best_idx = -1
        for i, res in enumerate(candidates):
            r_time = datetime.strptime(res['res_time'], "%Y/%m/%d %H:%M:%S")
            if r_time > t_time:
                matched_res = res
                # Remove used resolution? Optional.
                # candidates.pop(i) 
                break
        
        row = t.copy()
        if matched_res:
            row['resolution'] = matched_res['resolution']
            row['pnl'] = matched_res['pnl']
            row['res_time'] = matched_res['res_time']
        else:
            row['resolution'] = 'PENDING'
            row['pnl'] = 0.0
            row['res_time'] = ''
            
        merged_data.append(row)
        
    return merged_data

def analyze_data(data):
    # Dimensions: 
    # 1. Strategy Performance
    strategy_stats = defaultdict(lambda: {'pnl': 0.0, 'wins': 0, 'total': 0, 'win_accum_z': 0.0})
    
    # Asset Performance
    asset_stats = defaultdict(lambda: {'pnl': 0.0, 'count': 0})
    
    # Z-Score buckets
    z_stats = defaultdict(lambda: {'wins': 0, 'total': 0})
    
    # Trend buckets
    trend_stats = defaultdict(lambda: {'pnl': 0.0, 'count': 0})
    
    for row in data:
        strat = row['strategy']
        pnl = row['pnl']
        z = row['z_score']
        market = row['market']
        trend = abs(row['trend_pct'])
        
        # Strategy
        strategy_stats[strat]['pnl'] += pnl
        strategy_stats[strat]['total'] += 1
        if pnl > 0:
            strategy_stats[strat]['wins'] += 1
            strategy_stats[strat]['win_accum_z'] += z
            
        # Asset
        asset = market.split('-')[0].upper()
        asset_stats[asset]['pnl'] += pnl
        asset_stats[asset]['count'] += 1
        
        # Z-Bucket
        z_bucket = f"{int(z*2)/2:.1f}" # 0.0, 0.5, 1.0, 1.5...
        z_stats[z_bucket]['total'] += 1
        if pnl > 0:
            z_stats[z_bucket]['wins'] += 1
            
        # Trend Bucket
        t_bucket = f"{int(trend*1000)/1000:.3f}" # 0.001, 0.002
        trend_stats[t_bucket]['pnl'] += pnl
        trend_stats[t_bucket]['count'] += 1

    return strategy_stats, asset_stats, z_stats, trend_stats

def write_report(strategy_stats, asset_stats, z_stats, trend_stats, raw_count):
    with open(OUTPUT_MD, 'w', encoding='utf-8') as f:
        f.write("# Analysis Report\n\n")
        f.write(f"**Total Trades Analyzed**: {raw_count}\n\n")
        
        # 1. Strategy Ranking
        f.write("## 1. Strategy Leaderboard\n")
        f.write("| Rank | Strategy | Total PnL ($) | Win Rate | Trades |\n")
        f.write("|---|---|---|---|---|\n")
        
        sorted_strats = sorted(strategy_stats.items(), key=lambda x: x[1]['pnl'], reverse=True)
        for i, (strat, stats) in enumerate(sorted_strats):
            win_rate = (stats['wins'] / stats['total'] * 100) if stats['total'] > 0 else 0
            f.write(f"| {i+1} | {strat} | **{stats['pnl']:.2f}** | {win_rate:.1f}% | {stats['total']} |\n")
        f.write("\n")
        
        # 2. Asset Performance
        f.write("## 2. Asset Performance\n")
        f.write("| Asset | Total PnL ($) | Trade Count |\n")
        f.write("|---|---|---|\n")
        for asset, stats in sorted(asset_stats.items(), key=lambda x: x[1]['pnl'], reverse=True):
             f.write(f"| {asset} | {stats['pnl']:.2f} | {stats['count']} |\n")
        f.write("\n")
        
        # 3. Z-Score Impact
        f.write("## 3. Signal Quality (Z-Score)\n")
        f.write("| Z-Score Range | Win Rate | Trades |\n")
        f.write("|---|---|---|\n")
        for z, stats in sorted(z_stats.items()):
            wr = (stats['wins'] / stats['total'] * 100) if stats['total'] > 0 else 0
            f.write(f"| {z}+ | {wr:.1f}% | {stats['total']} |\n")
        f.write("\n")

def main():
    if not os.path.exists(LOG_FILE):
        print("Log file not found.")
        return
        
    data = parse_log(LOG_FILE)
    if not data:
        print("No matches.")
        return
        
    # Write CSV
    keys = data[0].keys()
    with open(OUTPUT_CSV, 'w', newline='', encoding='utf-8') as f:
        dict_writer = csv.DictWriter(f, keys)
        dict_writer.writeheader()
        dict_writer.writerows(data)
    print(f"CSV saved to {OUTPUT_CSV}")
    
    # Analyze
    s_stats, a_stats, z_stats, t_stats = analyze_data(data)
    write_report(s_stats, a_stats, z_stats, t_stats, len(data))
    print(f"Report saved to {OUTPUT_MD}")

if __name__ == "__main__":
    main()
