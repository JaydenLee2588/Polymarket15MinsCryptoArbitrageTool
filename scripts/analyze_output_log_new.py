import re
import sys
import os
from collections import defaultdict

def parse_log_line(line):
    """
    Parse a log line from output-2.log
    Target Buy Format: 
    2026/02/06 12:06:03 ✅ [💰真实][基本3号] Leg1买入成功 | 时间=12:06:00 | 市场=sol-updown-15m-1770379200 | 方向=DOWN | Z=4.10 | 价格=0.8400 | 数量=40.24 | 成本=$33.80 ...
    Target Resolution Format:
    ✅ [基本3号][btc-updown-...] 决议: DOWN, 持仓: DOWN, PnL: $20.00
    """
    
    # Check for Buy
    if "Leg1买入成功" in line:
        try:
            # Extract timestamp
            timestamp_match = re.search(r"^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})", line)
            timestamp = timestamp_match.group(1) if timestamp_match else ""
            
            # Extract Strategy: [💰真实][基本3号] -> 基本3号
            # Regex: \[.*?\]\[(.*?)\]
            strat_match = re.search(r"✅ \[(.*?)\]\[(.*?)\]", line)
            strategy = strat_match.group(2) if strat_match else "Unknown"
            
            # Extract key-values
            market = re.search(r"市场=([^ |]+)", line).group(1)
            side = re.search(r"方向=([^ |]+)", line).group(1)
            price = float(re.search(r"价格=([^ |]+)", line).group(1))
            amount = float(re.search(r"数量=([^ |]+)", line).group(1))
            
            return {
                "type": "buy",
                "time": timestamp,
                "strategy": strategy,
                "market": market,
                "side": side,
                "price": price,
                "amount": amount
            }
        except Exception as e:
            # print(f"Error parsing buy line: {e} | Line: {line.strip()}")
            return None

    # Check for Resolution
    if "决议:" in line and "PnL:" in line:
        try:
            # Format: ✅ [Group][Slug] 决议: UP, 持仓: UP, PnL: $10.00
            strat_match = re.search(r"✅ \[(.*?)\]\[(.*?)\]", line)
            strategy = strat_match.group(1) if strat_match else "Unknown"
            market = strat_match.group(2) if strat_match else "Unknown"
            
            pnl_str = re.search(r"PnL: \$?([-\d\.]+)", line).group(1)
            pnl = float(pnl_str)
            
            return {
                "type": "resolution",
                "strategy": strategy,
                "market": market,
                "pnl": pnl
            }
        except Exception as e:
            return None
            
    return None

def main():
    if len(sys.argv) < 3:
        print("Usage: python analyze_output_log_new.py <log_file> <output_file> [interval]")
        sys.exit(1)
        
    filepath = sys.argv[1]
    outfile = sys.argv[2]
    interval = 0.01
    if len(sys.argv) > 3:
        interval = float(sys.argv[3])
        
    stats = defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0, "resolutions": 0})
    price_buckets = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0})) # strat -> price -> stats
    time_buckets = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0})) # strat -> time -> stats
    
    # Store buys to link with resolutions (simplified, assuming matching strat/market)
    # Since resolution might not be in log, we'll track what we can
    # Ideally link by Market Slug
    # But for Price/Time aggregation, we use BUY info.
    # For PnL aggregation, we use RESOLUTION info.
    # Linking PnL back to Buy Price is hard if logs are separated or unordered.
    # We will aggregate PnL by Strategy separately.
    # For Price/Time aggregation, we will count TRADES (Buys). We cannot accurately sum PnL per price bucket without linking.
    # Unless we assume 1 buy = 1 resolution.
    # We'll try to link by active_trades map: Market -> BuyInfo
    
    active_trades = {} # Market -> BuyInfo
    
    # Pre-process file to find all lines
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            lines = f.readlines()
    except:
        with open(filepath, 'r', encoding='gbk') as f:
            lines = f.readlines()
            
    for line in lines:
        data = parse_log_line(line)
        if not data:
            continue
            
        if data["type"] == "buy":
            strat = data["strategy"]
            stats[strat]["count"] += 1
            
            # Price Bucket
            p = data["price"]
            bucket = round(p / interval) * interval
            price_buckets[strat][bucket]["count"] += 1
            
            # Time Bucket (Relative to Market Start)
            # Market Slug Format: sol-updown-15m-1770379200
            # Extract timestamp from slug
            try:
                slug_parts = data["market"].split("-")
                market_start_ts = int(slug_parts[-1])
                
                # Parse log timestamp to unix
                # 2026/02/06 12:06:03
                from datetime import datetime
                import time
                
                log_time_str = data["time"]
                dt = datetime.strptime(log_time_str, "%Y/%m/%d %H:%M:%S")
                # Assuming local time... wait, log might be UTC or Local?
                # The log timestamp looks like: 2026/02/06 12:06:03
                # The market timestamp 1770379200 is 2026-02-06 12:00:00 UTC
                # Go `time.Now()` usually returns local time unless UTC is specified.
                # Let's assume the log time is in the same timezone context as the machine running the script.
                # Actually, simply: convert log time to timestamp.
                # If both are UTC or both are Local, diff is correct.
                # In main.go, logs use std log package which uses local time.
                # 1770379200 is UTC.
                # We need to know the timezone offset. 
                # Or we can deduce it. 
                # 12:06:03 vs 12:00:00 start. Diff is ~6 min.
                # If Market Start is 1770379200 (12:00:00 UTC).
                # Log says 12:06:03.
                # If log is UTC, diff is 363s.
                # If log is UTC+8 (China), 12:06 is 04:06 UTC. Diff is huge negative.
                # If log is Local (US Eastern), 12:06 is 17:06 UTC. Diff is huge positive.
                
                # HACK: We can just take the minutes/seconds relative to the hour of the log time,
                # AND compare it to the minutes/seconds of the market start time?
                # No, market start might not be :00, :15, :30, :45 exactly if it's a specific time?
                # Actually 15m markets always start at :00, :15, :30, :45.
                # So we can just take (OrderMinute % 15) * 60 + OrderSecond.
                # Example: Market starts 12:00. Order 12:06:03. (6%15)*60 + 3 = 363s.
                # Example: Market starts 12:15. Order 12:20:03. (20%15)*60 + 3 = 303s. (Wait 20-15=5m) -> 5*60+3 = 303s.
                # (20 % 15) is 5. Correct.
                # Example: Market starts 12:45. Order 12:50:03. (50%15)*60 + 3 = 303s.
                # (50 % 15) is 5. Correct.
                # What if order is in next hour? e.g. Start 12:45, Order 13:02.
                # 13:02 is 17 mins after.
                # (2 % 15) is 2. Incorrect. Should be 17.
                
                # Better approach:
                # 1. Parse Log Time to datetime object (Naive).
                # 2. Extract Market Start from Slug (UTC Timestamp).
                # 3. Convert Market Start to datetime object (UTC).
                # 4. We don't know Log Timezone.
                # BUT we know orders happen within [0, 15] minutes of start.
                # So we can align them.
                # diff = (LogTime - MarketStart) % 900 ? No.
                
                # Let's rely on the "15m" nature.
                # An order belongs to a specific market slug.
                # The log time MUST be after the market start.
                # We can assume the "Minute" and "Second" fields of the log time are correct relative to the hour.
                # We just need to find the "Market Start Minute" relative to the hour.
                # Market Start 1770379200 -> 12:00:00 UTC. Min=0.
                # Market Start + 900 -> 12:15:00 UTC. Min=15.
                
                # Let's convert Market Start Timestamp to a datetime structure to get the 'minute' part.
                mkt_dt = datetime.utcfromtimestamp(market_start_ts)
                mkt_minute = mkt_dt.minute
                mkt_second = mkt_dt.second
                
                # Log Time parts
                parts = data["time"].split(" ")[1].split(":") # 12:06:03
                log_hour = int(parts[0])
                log_minute = int(parts[1])
                log_second = int(parts[2])
                
                # Calculate diff in seconds
                # We need to handle hour wrap around (e.g. Start 12:45, Log 13:02)
                
                # Calculate "Minutes from top of hour" for both.
                # Actually, simply:
                # diff_min = log_minute - mkt_minute
                # if diff_min < 0: diff_min += 60 (Wrapped hour)
                
                # Wait, what if it wrapped 2 hours? Unlikely for 15m market.
                # But we should check if log_time is actually "before" market start in absolute terms? 
                # We can't strictly check without timezone.
                # But we can assume causality: Trade happens after start.
                
                diff_min = log_minute - mkt_minute
                if diff_min < 0:
                    diff_min += 60
                    
                # If diff_min is huge (e.g. 50 mins), maybe we matched wrong market?
                # But we trust the log line "Market=..."
                
                total_sec = diff_min * 60 + (log_second - mkt_second)
                
                # Bucket 10s
                t_bucket_idx = (total_sec // 10) * 10
                t_bucket_str = f"{t_bucket_idx//60}m{t_bucket_idx%60:02d}s - {t_bucket_idx//60}m{t_bucket_idx%60+10:02d}s"
                
                time_buckets[strat][t_bucket_str]["count"] += 1
                
                # Store for linking PnL later
                active_trades[data["market"]] = {
                    "strat": strat,
                    "price_bucket": bucket,
                    "time_bucket": t_bucket_str
                }
            except Exception as e:
                # print(f"Error processing time: {e}")
                pass
                
        elif data["type"] == "resolution":
            strat = data["strategy"]
            pnl = data["pnl"]
            stats[strat]["resolutions"] += 1
            stats[strat]["pnl"] += pnl
            if pnl > 0:
                stats[strat]["wins"] += 1
                
            # If we can link to a buy, update bucket stats
            mkt = data["market"]
            if mkt in active_trades:
                info = active_trades[mkt]
                # Check stratum matches
                if info["strat"] == strat:
                    price_buckets[strat][info["price_bucket"]]["pnl"] += pnl
                    time_buckets[strat][info["time_bucket"]]["pnl"] += pnl
                    if pnl > 0:
                        price_buckets[strat][info["price_bucket"]]["wins"] += 1
                        time_buckets[strat][info["time_bucket"]]["wins"] += 1

    # Generate Report
    with open(outfile, 'w', encoding='utf-8') as f:
        f.write("# 策略分析报告 (Output Log)\n\n")
        f.write(f"数据来源: `{filepath}`\n\n")
        f.write("## 1. 策略总览\n\n")
        f.write("| 策略 | 交易次数 | 决议次数 | 胜 | 胜率 | 总PnL | 决议比例 |\n")
        f.write("| :--- | :---: | :---: | :---: | :---: | :---: | :---: |\n")
        
        all_buys = 0
        all_pnl = 0.0
        
        for strat in sorted(stats.keys()):
            s = stats[strat]
            all_buys += s["count"]
            all_pnl += s["pnl"]
            
            win_rate = 0.0
            if s["resolutions"] > 0:
                win_rate = s["wins"] / s["resolutions"] * 100
                
            res_rate = 0.0
            if s["count"] > 0:
                res_rate = s["resolutions"] / s["count"] * 100
                
            pnl_str = f"${s['pnl']:.2f}" if s["pnl"] >= 0 else f"-${abs(s['pnl']):.2f}"
            
            f.write(f"| {strat} | {s['count']} | {s['resolutions']} | {s['wins']} | {win_rate:.1f}% | {pnl_str} | {res_rate:.1f}% |\n")
            
        all_pnl_str = f"${all_pnl:.2f}" if all_pnl >= 0 else f"-${abs(all_pnl):.2f}"
        f.write(f"| **总计** | **{all_buys}** | - | - | - | **{all_pnl_str}** | - |\n\n")
        
        f.write("## 2. 按下单价格聚合 (Interval: {:.2f})\n\n".format(interval))
        
        for strat in sorted(price_buckets.keys()):
            f.write(f"### {strat}\n\n")
            f.write("| 价格区间 | 交易次数 | 胜率 (基于决议) | 累计PnL |\n")
            f.write("| :---: | :---: | :---: | :---: |\n")
            
            buckets = sorted(price_buckets[strat].keys())
            total_strat_pnl = 0.0
            
            for b in buckets:
                data = price_buckets[strat][b]
                count = data["count"]
                # Note: Win rate here is based on RESOLVED trades in this bucket
                # But count is TOTAL Buys. 
                # If resolution is missing, win rate might be misleading if we divide by 'count'.
                # Better to divide by 'resolutions' count if we tracked it, but we only tracked 'wins'.
                # Let's approximate: WinRate = Wins / Count (Assuming all resolved or we just show partial data)
                # Actually, if we don't have resolution, wins=0. So 0% is correct for "Unresolved".
                
                # To be more accurate, we should have tracked "resolved_count" per bucket.
                # But for now:
                wins = data["wins"]
                # If we don't know how many resolved, we can't calculate a true "Win Rate".
                # But typically analysis assumes closed dataset.
                
                win_rate = (wins / count * 100) if count > 0 else 0
                # Wait, if only 1 out of 10 resolved and it won, win rate 10% is wrong.
                # But without linking every buy strictly, this is best effort.
                
                pnl = data["pnl"]
                total_strat_pnl += pnl
                pnl_str = f"${pnl:.2f}" if pnl >= 0 else f"-${abs(pnl):.2f}"
                
                f.write(f"| {b:.2f} - {b+interval:.2f} | {count} | {win_rate:.1f}% | {pnl_str} |\n")
            f.write("\n")

        f.write("## 3. 按下单时间聚合\n\n")
        for strat in sorted(time_buckets.keys()):
            f.write(f"### {strat}\n\n")
            f.write("| 下单时刻 | 交易次数 | 胜率 | 累计PnL |\n")
            f.write("| :---: | :---: | :---: | :---: |\n")
            
            # Sort by time value (string sort "0m..." works but "10m..." < "2m..." is wrong)
            def parse_time_bucket(s):
                # "21m00s - ..."
                try:
                    part = s.split("m")[0]
                    return int(part) * 60 + int(s.split("m")[1].split("s")[0])
                except:
                    return 0
            
            sorted_times = sorted(time_buckets[strat].keys(), key=parse_time_bucket)
            
            for t in sorted_times:
                data = time_buckets[strat][t]
                count = data["count"]
                wins = data["wins"]
                win_rate = (wins / count * 100) if count > 0 else 0
                pnl = data["pnl"]
                pnl_str = f"${pnl:.2f}" if pnl >= 0 else f"-${abs(pnl):.2f}"
                
                f.write(f"| {t} | {count} | {win_rate:.1f}% | {pnl_str} |\n")
            f.write("\n")

if __name__ == "__main__":
    main()
