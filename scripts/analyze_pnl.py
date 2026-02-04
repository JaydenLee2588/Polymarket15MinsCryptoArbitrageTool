import re
import os
from collections import defaultdict

def parse_log_line(line):
    # Regex to extract key fields
    # Example: 【13:18:54】【Pattern策略4】【btc-updown-15m-1770228900】"【3.9分】【买入】【UP】...
    # Example Sell: ... "【5.2分】【止盈】【7】【0.6600】【$4.62】【$1.47】" "【】"
    
    # Base pattern for the prefix parts
    prefix_pattern = r"【(.*?)】【(.*?)】【(.*?)】\"【(.*?)分】【(.*?)】"
    match = re.search(prefix_pattern, line)
    if not match:
        return None
    
    timestamp = match.group(1)
    strategy = match.group(2)
    market = match.group(3)
    minutes_str = match.group(4)
    action = match.group(5)
    
    try:
        minutes = float(minutes_str)
    except ValueError:
        minutes = 0.0

    pnl = 0.0
    if action in ["止盈", "时间止损", "决议", "决议赎回"]:
        # Extract PnL for sell lines. It's usually the 6th bracketed item in the quotes
        # Format: "【min】【action】【amt】【price】【rev】【pnl】"
        # Let's use a simpler approach: split by "】【"
        parts = line.split("】【")
        # The PnL part usually looks like "$1.47】" or "$-1.40】" or "$1.47"
        # We need to find the part with "$"
        for p in parts:
            if "$" in p:
                # There might be Cost ($3.15) and PnL ($1.47). PnL is usually the last dollar amount in Sell line.
                # Actually, in Sell line: amount, price, revenue, pnl.
                # Let's strictly parse the Sell line structure from the previous step's code analysis
                pass
        
        # Regex for PnL: 【$1.47】 or 【$-1.40】
        # Find all occurrences of \$([-0-9.]+)
        pnl_matches = re.findall(r"【\$([-0-9.]+)】", line)
        if pnl_matches:
            # In sell line: Revenue, then PnL. So PnL is the last one?
            # Code: fmt.Sprintf(..., entry.Revenue, entry.PnL, ...)
            # So the last monetary value is PnL? 
            # Actually, let's look at the log line: "【...】【$4.62】【$1.47】" "【】"
            # Yes, the last one before the resolution tag seems to be PnL.
            try:
                pnl = float(pnl_matches[-1])
            except ValueError:
                pnl = 0.0

    return {
        "strategy": strategy,
        "market": market,
        "action": action,
        "minutes": minutes,
        "pnl": pnl
    }

def process_log_file(filepath):
    if not os.path.exists(filepath):
        print(f"File not found: {filepath}")
        return

    # Tracking open trades to match Sell back to Buy Entry Time
    # Key: (strategy, market) -> entry_minutes
    open_trades = {}
    
    # Aggregation: Strategy -> EntryMinute (int) -> PnL
    agg_data = defaultdict(lambda: defaultdict(float))
    agg_counts = defaultdict(lambda: defaultdict(int))

    with open(filepath, 'r', encoding='utf-8') as f:
        for line in f:
            if not line.strip():
                continue
            
            data = parse_log_line(line)
            if not data:
                continue
                
            strat = data['strategy']
            mkt = data['market']
            action = data['action']
            mins = data['minutes']
            
            if action == "买入":
                # Store entry time
                open_trades[(strat, mkt)] = mins
            
            elif action in ["止盈", "时间止损", "决议", "决议赎回"]:
                # Match with entry time
                entry_mins = open_trades.get((strat, mkt))
                
                # If we can't find the entry match (maybe log started mid-trade), use current mins as fallback?
                # But user wants "Entry Time" aggregation. If we miss entry time, we skip or use bucket -1.
                if entry_mins is None:
                    # Try to deduce? No, just skip or log.
                    # Actually, for this specific request, accuracy is key.
                    # Let's assume the log file is complete enough for the analysis.
                    entry_bucket = int(mins) # Fallback to exit time if needed? NO.
                    # Let's assume missing entry means it was opened before this log file started.
                    # But wait, looking at the logs provided in chat, they seem continuous.
                    # However, there might be duplicate market entries (e.g. Strat 4 bought, sold, bought again).
                    # My simple dict key (strat, mkt) overwrites the previous one.
                    # Is this safe?
                    # Line 6: Sell. Line 7: Buy. 
                    # Yes, sequential parsing allows overwrite after sell is processed.
                    # But we MUST process the Sell *before* the next Buy creates a new entry in the dict.
                    pass
                else:
                    entry_bucket = int(entry_mins)
                    
                    agg_data[strat][entry_bucket] += data['pnl']
                    agg_counts[strat][entry_bucket] += 1
                    
                    # Remove from open trades to define "closed" and allow next trade on same market
                    del open_trades[(strat, mkt)]

    # Print Report
    print("### 分策略按【下单时间(开局第几分钟)】聚合 PnL")
    
    all_strats = sorted(agg_data.keys())
    
    for strat in all_strats:
        print(f"\n#### {strat}")
        print("| 下单时刻 (分钟) | 交易次数 | 累计盈亏 (PnL) |")
        print("| :---: | :---: | :---: |")
        
        # Sort buckets
        buckets = sorted(agg_data[strat].keys())
        total_pnl = 0
        total_count = 0
        
        for b in buckets:
            pnl = agg_data[strat][b]
            count = agg_counts[strat][b]
            print(f"| {b}m - {b+1}m | {count} | ${pnl:.2f} |")
            total_pnl += pnl
            total_count += count
            
        print(f"| **合计** | **{total_count}** | **${total_pnl:.2f}** |")

process_log_file(r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260204\trade_history_1316.log")
