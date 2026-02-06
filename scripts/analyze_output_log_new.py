import re
import sys
import os
from collections import defaultdict
import math
from datetime import datetime

def parse_line(line):
    # Timestamp: 2026/02/05 18:20:35
    ts_match = re.search(r'^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})', line)
    if not ts_match: return None
    timestamp_str = ts_match.group(1)
    
    # Parse timestamp to minutes from start (or just raw minutes of day)
    # For simplicity, we'll store the object
    try:
        dt = datetime.strptime(timestamp_str, "%Y/%m/%d %H:%M:%S")
    except:
        return None

    # Check for [TradeHistory]
    if "📝 [TradeHistory]" not in line:
        return None
        
    # Extract message after first colon that follows [TradeHistory] ....
    # Actually, the format is: "2026/02/05 18:20:35 📝 [TradeHistory] 记录卖出(已写入): Pattern策略2 | ..."
    # Let's split by ": "
    parts = line.split(": ", 1)
    if len(parts) < 2: return None
    
    content = parts[1].strip()
    
    # Determine Action Type
    if "记录买入" in line:
        action = "BUY"
    elif "记录卖出" in line:
        action = "SELL"
    elif "决议" in line:
        action = "RESOLUTION"
    else:
        return None
    
    # Split content by |
    fields = [f.strip() for f in content.split("|")]
    
    # Buy: Strategy | Market | Side | Price
    if action == "BUY":
        if len(fields) < 4: return None
        try:
            return {
                "type": "BUY",
                "timestamp": dt,
                "strategy": fields[0],
                "market": fields[1],
                "side": fields[2],
                "price": float(fields[3])
            }
        except:
            return None

    # Sell/Resolution: Strategy | Market | Reason/Res | PnL=$0.60
    elif action in ["SELL", "RESOLUTION"]:
        if len(fields) < 4: return None
        pnl_str = fields[3].replace("PnL=", "").replace("$", "")
        try:
            pnl = float(pnl_str)
        except:
            pnl = 0.0
            
        return {
            "type": action,
            "timestamp": dt,
            "strategy": fields[0],
            "market": fields[1],
            "reason": fields[2],
            "pnl": pnl
        }
        
    return None

def analyze_log(filepath, output_path):
    if not os.path.exists(filepath):
        print(f"File not found: {filepath}")
        return

    # Tracking
    # open_trades[(strategy, market)] = { entry_price, entry_time }
    # To handle multiple trades in same market/strategy (if possible?), we might need a list, 
    # but the log seems to unique key by market slug? 
    # Actually, one strategy can buy multiple times? The code uses `state.Positions[cfg.ID]`, so only one active position per strategy per market.
    open_trades = {} 
    
    stats = defaultdict(lambda: {
        'count': 0, 
        'pnl': 0.0, 
        'wins': 0, 
        'resolutions': 0, 
        'sells': 0,
        'price_dist': defaultdict(lambda: {'count': 0, 'pnl': 0.0}),
        'time_dist': defaultdict(lambda: {'count': 0, 'pnl': 0.0})
    })

    with open(filepath, 'r', encoding='utf-8') as f:
        for line in f:
            data = parse_line(line)
            if not data: continue
            
            strat = data['strategy']
            mkt = data['market']
            
            if data['type'] == "BUY":
                # Record entry
                # Calculate "Minutes into the hour" or "Minutes from start of day" for Time Aggregation?
                # User asked for "下单时间进行一下聚合". 
                # Let's use "Minute of Hour" (0-59) or maybe "Hour of Day"?
                # Actually, in Polymarket 15m context, usually "Minutes from Market Start" is interesting,
                # but we don't have market start time easily here (it's in the slug but parsing is messy).
                # Let's use simple "Minute of Hour" (0-59) as a proxy for "Time in cycle".
                minutes = data['timestamp'].minute
                
                open_trades[(strat, mkt)] = {
                    'price': data['price'],
                    'minutes': minutes
                }
                
            elif data['type'] in ["SELL", "RESOLUTION"]:
                entry = open_trades.get((strat, mkt))
                if not entry:
                    # Maybe bought in previous log or simulation start?
                    # If we can't find entry price, we can't do Price Aggregation, but we have PnL.
                    # We'll skip Price/Time aggregation if missing entry, but count PnL.
                    pass
                
                pnl = data['pnl']
                s = stats[strat]
                s['count'] += 1
                s['pnl'] += pnl
                if pnl > 0: s['wins'] += 1
                
                if data['type'] == "RESOLUTION":
                    s['resolutions'] += 1
                else:
                    s['sells'] += 1
                
                if entry:
                    # Price Bucket (0.05 step)
                    price = entry['price']
                    p_bucket = math.floor(price / 0.05) * 0.05
                    p_key = f"{p_bucket:.2f}-{p_bucket+0.05:.2f}"
                    s['price_dist'][p_key]['count'] += 1
                    s['price_dist'][p_key]['pnl'] += pnl
                    
                    # Time Bucket (1 minute buckets)
                    t_bucket = entry['minutes']
                    s['time_dist'][t_bucket]['count'] += 1
                    s['time_dist'][t_bucket]['pnl'] += pnl
                    
                    # Remove from open trades
                    del open_trades[(strat, mkt)]

    # Generate Markdown
    lines = []
    lines.append(f"# 📊 Strategy Analysis Report")
    lines.append(f"**Source**: `{os.path.basename(filepath)}`")
    lines.append(f"**Generated**: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
    
    sorted_strats = sorted(stats.keys())
    
    for strat in sorted_strats:
        s = stats[strat]
        if s['count'] == 0: continue
        
        win_rate = (s['wins'] / s['count']) * 100
        res_ratio = (s['resolutions'] / s['count']) * 100
        
        lines.append(f"## 🔹 {strat}")
        lines.append(f"- **Trades**: {s['count']}")
        lines.append(f"- **Total PnL**: `${s['pnl']:.2f}`")
        lines.append(f"- **Win Rate**: {win_rate:.1f}%")
        lines.append(f"- **Resolution/Sale**: {s['resolutions']} / {s['sells']} ({res_ratio:.1f}% Resolution)")
        lines.append("")
        
        # Price Aggregation
        lines.append("### 🏷️ PnL by Order Price")
        lines.append("| Price Range | Count | PnL | Avg PnL |")
        lines.append("|---|---|---|---|")
        for p_key in sorted(s['price_dist'].keys()):
            d = s['price_dist'][p_key]
            avg = d['pnl'] / d['count'] if d['count'] > 0 else 0
            lines.append(f"| {p_key} | {d['count']} | ${d['pnl']:.2f} | ${avg:.2f} |")
        lines.append("")
        
        # Time Aggregation
        lines.append("### ⏰ PnL by Order Time (Minute of Hour)")
        lines.append("| Minute | Count | PnL |")
        lines.append("|---|---|---|")
        for t_key in sorted(s['time_dist'].keys()):
            d = s['time_dist'][t_key]
            lines.append(f"| {t_key} | {d['count']} | ${d['pnl']:.2f} |")
        lines.append("\n---\n")

    with open(output_path, 'w', encoding='utf-8') as f:
        f.write("\n".join(lines))
        
    print(f"✅ Report saved to: {output_path}")
    print("\n".join(lines[:20])) # Preview

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python analyze_output_log_new.py <log_file> <output_file>")
        sys.exit(1)
        
    analyze_log(sys.argv[1], sys.argv[2])
