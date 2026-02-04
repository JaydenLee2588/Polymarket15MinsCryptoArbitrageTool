import re
import os
from collections import defaultdict
import math

def parse_log_line(line):
    # Regex to extract key fields
    # Example Buy: 【13:18:54】【Pattern策略4】【btc-updown-15m-1770228900】"【3.9分】【买入】【UP】【7】【0.4500】【$3.15】"...
    # Example Sell: ... "【5.2分】【止盈】【7】【0.6600】【$4.62】【$1.47】" "【】"
    
    prefix_pattern = r"【(.*?)】【(.*?)】【(.*?)】\"【(.*?)分】【(.*?)】"
    match = re.search(prefix_pattern, line)
    if not match:
        return None
    
    strategy = match.group(2)
    market = match.group(3)
    minutes_str = match.group(4)
    action = match.group(5)

    price = 0.0
    pnl = 0.0

    # Extract Price for Buy
    if action == "买入":
        # Format: ...【UP】【7】【0.4500】...
        # We need the 4th item in the first quote block?
        # Let's split usage "】【" inside the first quote block
        # Quote block content is after the prefix match.
        # Let's just Regex for price inside the unique Buy structure
        # Buy structure: "【min】【买入】【side】【amount】【PRICE】【cost】"
        buy_match = re.search(r"【买入】.*?【.*?】.*?【.*?】.*?【([0-9.]+)】", line)
        if buy_match:
            try:
                price = float(buy_match.group(1))
            except:
                price = 0.0
    
    # Extract PnL for Sell
    elif action in ["止盈", "时间止损", "决议", "决议赎回"]:
        # Find last dollar amount
        pnl_matches = re.findall(r"【\$([-0-9.]+)】", line)
        if pnl_matches:
            try:
                pnl = float(pnl_matches[-1])
            except:
                pnl = 0.0

    return {
        "strategy": strategy,
        "market": market,
        "action": action,
        "price": price,
        "pnl": pnl
    }

def process_log_file(filepath):
    if not os.path.exists(filepath):
        print(f"File not found: {filepath}")
        return

    # Key: (strategy, market) -> buy_price
    open_trades = {}
    
    # Aggregation: Strategy -> PriceBucket (string) -> PnL
    agg_data = defaultdict(lambda: defaultdict(float))
    agg_counts = defaultdict(lambda: defaultdict(int))
    
    # Bucket size
    BUCKET_SIZE = 0.10

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
            
            if action == "买入":
                open_trades[(strat, mkt)] = data['price']
            
            elif action in ["止盈", "时间止损", "决议", "决议赎回"]:
                buy_price = open_trades.get((strat, mkt))
                
                if buy_price is not None:
                    # Calculate bucket
                    # 0.00-0.09, 0.10-0.19, etc.
                    lower = math.floor(buy_price / BUCKET_SIZE) * BUCKET_SIZE
                    upper = lower + BUCKET_SIZE
                    bucket_key = f"{lower:.2f} - {upper:.2f}"
                    
                    agg_data[strat][bucket_key] += data['pnl']
                    agg_counts[strat][bucket_key] += 1
                    
                    del open_trades[(strat, mkt)]

    print("### 分策略按【下单价格】聚合 PnL")
    
    all_strats = sorted(agg_data.keys())
    
    for strat in all_strats:
        print(f"\n#### {strat}")
        print("| 下单价格区间 | 交易次数 | 累计盈亏 (PnL) |")
        print("| :---: | :---: | :---: |")
        
        # Sort buckets by price
        # Only works if format is "0.00 - 0.10" string sortable
        buckets = sorted(agg_data[strat].keys())
        
        total_pnl = 0
        total_count = 0
        
        for b in buckets:
            pnl = agg_data[strat][b]
            count = agg_counts[strat][b]
            print(f"| {b} | {count} | ${pnl:.2f} |")
            total_pnl += pnl
            total_count += count
            
        print(f"| **合计** | **{total_count}** | **${total_pnl:.2f}** |")

process_log_file(r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260204\trade_history_1316.log")
