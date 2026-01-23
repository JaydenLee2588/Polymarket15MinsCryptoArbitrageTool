import re
import csv
import datetime
import pandas as pd
from datetime import datetime as dt, timedelta, timezone

LOG_FILE = r"d:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\output_20260123.log"
OUTPUT_FILE = r"d:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\logs\trade_analysis_20260123.csv"

# Regex Patterns
# Trade Line: 2026/01/22 19:32:51 ✅ [💰真实][基准1号] Leg1买入成功 | 时间=19:32:48 | 市场=eth-updown-15m-1769110200 | 方向=UP | Z=0.95 | 价格=0.7800 | 数量=5.27 | 成本=$4.11 | 趋势=0.1711% | 公允价=0.8059, OrderID=...
trade_pattern = re.compile(r"(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}).*Leg1买入成功 \| 时间=(\d{2}:\d{2}:\d{2}) \| 市场=([\w-]+) \| 方向=(UP|DOWN) \| Z=([\d\.-]+) \| 价格=([\d\.]+) \| 数量=([\d\.]+) \| .*? \| 趋势=([\d\.-]+)% \| 公允价=([\d\.]+)")

# Resolution Line: 2026/01/22 19:50:47 ✅ [基准1号][eth-updown-15m-1769110200] 决议: UP, 持仓: UP, PnL: $1.16
# Resolution Line: 2026/01/22 19:50:47 ✅ [基准1号][eth-updown-15m-1769110200] 决议: UP, 持仓: UP, PnL: $1.16
# Regex to capture the second bracket content as market_slug
resolution_pattern = re.compile(r"\[.*?\]\[([\w-]+)\].*决议: (UP|DOWN),.*PnL: \$([\d\.\-]+)")

trades = []
pnl_map = {} # map market_slug -> pnl_value

print("Scanning log file...")

with open(LOG_FILE, 'r', encoding='utf-8') as f:
    lines = f.readlines()

# 1. First Pass: Parse Resolutions to get accurate PnL
for line in lines:
    res_match = resolution_pattern.search(line)
    if res_match:
        market_slug = res_match.group(1)
        pnl_val = float(res_match.group(3))
        pnl_map[market_slug] = pnl_val

# 2. Second Pass: Parse Trades
for line in lines:
    match = trade_pattern.search(line)
    if match:
        log_full_time_str = match.group(1) # e.g., 2026/01/22 19:32:51
        trade_time_str = match.group(2) # e.g. 19:32:48 (Actual trade time from log details)
        
        # Use the date from the LOG line, but time from the TRADE detail
        log_date_str = log_full_time_str.split(' ')[0] 
        full_trade_datetime_str = f"{log_date_str} {trade_time_str}"
        trade_dt = dt.strptime(full_trade_datetime_str, "%Y/%m/%d %H:%M:%S")
        trade_dt = trade_dt.replace(tzinfo=timezone.utc)
        
        slug = match.group(3)
        side = match.group(4)
        z_score = float(match.group(5))
        price = float(match.group(6))
        qty = float(match.group(7))
        trend = match.group(8) + "%"
        fair_price = float(match.group(9))
        
        # Calculate Time Since Open
        try:
            # Slug format: asset-updown-15m-TIMESTAMP (TIMESTAMP is START time)
            parts = slug.split('-')
            
            # Find the part that looks like a timestamp (last part usually)
            start_timestamp = int(parts[-1])
            # start_timestamp is directly the timestamp in the slug for 15m markets usually
             
            
            # Trade timestamp
            trade_ts = trade_dt.timestamp()
            
            seconds_diff = trade_ts - start_timestamp
            
            # Format as MM:SS
            minutes = int(seconds_diff // 60)
            seconds = int(seconds_diff % 60)
            time_since_open_str = f"{minutes}m {seconds}s"
            
        except Exception as e:
            time_since_open_str = "Error"
            print(f"Error parsing time for {slug}: {e}")

        # Get PnL from map (if available)
        # Note: If multiple trades for same market, PnL log might be aggregated or separate?
        # The log shows "决议... PnL: $1.16", this seems to be per-market total PnL.
        # If we have multiple trades in one market, we should probably distribute it or just show the market PnL?
        # The user wants "记录其 PnL", for the trade.
        # If there's only 1 trade per market, easy.
        # If multiple, the resolution log PnL is likely the sum.
        # Let's assume 1 trade per market or mapping is direct.
        # Actually in the log provided:
        # One trade for eth-updown-15m-1769110200 at 19:48:35
        # Resolution at 19:50:47
        
        realized_pnl = pnl_map.get(slug, 0.0) # Default to 0 if not resolved yet or not found

        trades.append({
            "Market": slug,
            "Side": side,
            "Time Since Open": time_since_open_str,
            "Trend": trend,
            "Z-Score": z_score,
            "Fair Price": fair_price,
            "Entry Price": price,
            "Quantity": qty,
            "PnL ($)": realized_pnl
        })

# Write to CSV
if not trades:
    print("No trades found.")
else:
    keys = trades[0].keys()
    with open(OUTPUT_FILE, 'w', newline='', encoding='utf-8-sig') as output_file:
        dict_writer = csv.DictWriter(output_file, keys)
        dict_writer.writeheader()
        dict_writer.writerows(trades)
    print(f"Successfully exported {len(trades)} trades to {OUTPUT_FILE}")
