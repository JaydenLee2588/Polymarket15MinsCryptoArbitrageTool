import re
import collections

LOG_FILE = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260129\output_sim.log"

# Strategies with sigmoid_sensitivity: 2.2
TARGET_STRATEGIES = {
    "基础参考", "改进1号", "改进2号", "改进3号", "改进4号", "改进5号",
    "改进6号", "改进7号", "改进8号", "改进9号", "改进12号", "改进13号"
}

def parse_log():
    market_outcomes = {}
    trades = []
    
    # Pre-compiled regex
    # Resolution pattern: ✅ [基础参考][xrp-updown-15m-1769698800] 决议: DOWN
    re_resolution = re.compile(r'✅ \[.*?\]\[(?P<market>.*?)\] 决议: (?P<outcome>UP|DOWN)')
    
    # Trade pattern: 
    # 2026/01/29 15:04:10 ✅ [模拟][改进9号] Leg1买入成功 | 时间=15:04:10 | 市场=sol-updown-15m-1769698800 | 方向=DOWN | Z=7.09 | 价格=0.9100 | 数量=50.00 | 成本=$45.50
    re_trade = re.compile(r'(?P<date>\d{4}/\d{2}/\d{2}) (?P<time>\d{2}:(?P<min>\d{2}):\d{2}) ✅ \[模拟\]\[(?P<strategy>.*?)\] Leg1买入成功 .*? 市场=(?P<market>.*?) .*? 方向=(?P<direction>UP|DOWN) .*? Z=(?P<z>[\d\.]+) .*? 价格=(?P<price>[\d\.]+) .*? 数量=(?P<qty>[\d\.]+)')

    print(f"Reading log file: {LOG_FILE}...")
    try:
        with open(LOG_FILE, 'r', encoding='utf-8', errors='replace') as f:
            for line in f:
                # Check resolution
                res_match = re_resolution.search(line)
                if res_match:
                    market = res_match.group('market')
                    outcome = res_match.group('outcome')
                    if market not in market_outcomes:
                        market_outcomes[market] = outcome
                    continue

                # Check trade
                trade_match = re_trade.search(line)
                if trade_match:
                    trades.append(trade_match.groupdict())
                    
    except Exception as e:
        print(f"Error reading file: {e}")
        return

    print(f"Found {len(market_outcomes)} market outcomes.")
    print(f"Found {len(trades)} trades.")

    # Statistics Containers
    stats_symbol_pnl = collections.defaultdict(float)
    stats_minute_pnl = collections.defaultdict(list) # minute -> list of (count, pnl) - actually just Aggregator {count, pnl}
    stats_minute_agg = collections.defaultdict(lambda: {'count': 0, 'pnl': 0.0})
    
    stats_z_pnl = collections.defaultdict(float) # Z-score -> PnL (rounded Z)
    stats_price_pnl = collections.defaultdict(float) # Price -> PnL (rounded 0.01)

    processed_count = 0
    
    for t in trades:
        market = t['market']
        outcome = market_outcomes.get(market)
        
        # If simulation ended before resolution, we can't calculate PnL 
        # (or we assume 0, or we skip. Let's skip to be accurate to "realized" pnl, 
        # but user asked for PnL analysis of the logs. If log holds resolution, use it.)
        if not outcome:
            continue
            
        processed_count += 1
        
        strategy = t['strategy']
        direction = t['direction']
        price = float(t['price'])
        qty = float(t['qty'])
        z_score = float(t['z'])
        minute_raw = int(t['min'])
        
        # Calculate PnL
        # Cost = Price * Qty
        # Revenue = 1 * Qty if Win, 0 if Lose
        is_win = (direction == outcome)
        revenue = qty if is_win else 0.0
        cost = price * qty
        pnl = revenue - cost
        
        # 1. Symbol PnL
        # market id example: sol-updown-15m-1769698800 -> symbol "sol"
        symbol = market.split('-')[0].upper()
        stats_symbol_pnl[symbol] += pnl
        
        # 2. Minute PnL (0-14)
        minute_idx = minute_raw % 15
        stats_minute_agg[minute_idx]['count'] += 1
        stats_minute_agg[minute_idx]['pnl'] += pnl
        
        # 3. Z-score PnL (Only for TARGET_STRATEGIES)
        if strategy in TARGET_STRATEGIES:
            # Round Z to 1 decimal for grouping? Or just raw? User asked "Each Z value". 
            # Z values are usually continuous. Let's group by 0.1 bin or just distinct values if not too many.
            # Showing 1 decimal place bucket is reasonable.
            z_bucket = round(z_score, 1)
            stats_z_pnl[z_bucket] += pnl
            
        # 4. Price PnL
        price_bucket = round(price, 2)
        stats_price_pnl[price_bucket] += pnl

    # Output to JSON
    import json
    results = {
        "symbol_pnl": stats_symbol_pnl,
        "minute_pnl": stats_minute_agg,
        "z_pnl": stats_z_pnl,
        "price_pnl": stats_price_pnl
    }
    
    with open("analysis_result.json", "w", encoding="utf-8") as f:
        json.dump(results, f, indent=2, sort_keys=True)
    print("Results written to analysis_result.json")

if __name__ == "__main__":
    parse_log()
