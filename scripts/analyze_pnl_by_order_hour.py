
import re
import pandas as pd
from datetime import datetime, timedelta

LOG_FILE = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260219\trade_history_constract.log"

STRATEGY_MAPPING = {
    "Pattern策略5": "策略5",
    "Pattern策略5+Sniper": "策略5+Sniper",
    "基本2号": "基本2号",
    "基准1号OLD": "基准1号OLD",
    "基本3号": "基本3号",
    "低阈值11": "低阈值11",
    "低阈值13": "低阈值13",
    "基准1号": "基准1号",
    "时段版210-300": "时段版210-300",
    "时段版300-390": "时段版300-390",
    "时段版480-570": "时段版480-570",
    "时段版570-660": "时段版570-660",
    "简化版": "简化版",
    "高阈值17": "高阈值17",
    "高阈值19": "高阈值19",
    "高阈值21": "高阈值21"
}

def parse_pnl_str(s):
    # s format: "$12.34" or "$-12.34"
    return float(s.replace('$','').replace(',',''))

def analyze_by_order_time():
    orders = {} # (strategy, slug) -> timestamp
    pnl_data = [] # {hour: int, pnl: float, strategy: str}
    
    # Track date
    current_date = datetime(2026, 2, 19) # Start date guess, will auto-adjust
    # Actually, we need to handle date keying properly or just use the time string for the hour?
    # User said: "Aggregate 24 hours (different days same time merged)"
    # So we just need the info: "Order placed at HH:MM:SS", we extract HH.
    
    # We need to scan the whole file.
    
    with open(LOG_FILE, 'r', encoding='utf-8') as f:
        # If encoding fails, fallback handled in main usually
        lines = f.readlines()
        
    for line in lines:
        line = line.strip()
        if not line.startswith("【"): continue
        
        # Extract basic info
        # Structure: 【Time】【Session】【Strategy】【Slug】...
        parts = re.findall(r"【([^】]*)】", line)
        if len(parts) < 4: continue
        
        time_str = parts[0] # HH:MM:SS
        raw_strat = parts[2]
        slug = parts[3]
        
        strategy = STRATEGY_MAPPING.get(raw_strat, raw_strat)
        
        content = line
        
        # Check for Buy Order
        if "【买入】" in content and "买入失败" not in content and "模拟失败" not in content:
            # Store order time
            # We assume the first buy is the "start" of the trade logic
            key = (strategy, slug)
            if key not in orders:
                orders[key] = time_str
                
        # Check for PnL
        # "决议赎回" or "Sniper:profit_target_reached" or "Sniper:zscore_reverted"
        if "决议赎回" in content or "Sniper:profit_target_reached" in content or "Sniper:zscore_reverted" in content:
             # Find PnL value
             dollar_matches = re.findall(r"【\$([-]?\d+\.\d+)】", content)
             if not dollar_matches: continue
             pnl = parse_pnl_str(dollar_matches[-1])
             
             key = (strategy, slug)
             order_time_str = orders.get(key)
             
             if order_time_str:
                 # Use Order Time
                 hour = int(order_time_str.split(":")[0])
             else:
                 # Fallback: Use Settlement Time (current line)
                 # print(f"Warning: No order time found for {key}, using settlement time {time_str}")
                 hour = int(time_str.split(":")[0])
                 
             pnl_data.append({
                 "hour": hour,
                 "pnl": pnl,
                 "strategy": strategy
             })

    # Aggregate
    df = pd.DataFrame(pnl_data)
    if df.empty:
        print("No PnL data found.")
        return

    # Pivot table: rows=hour, cols=strategy, values=pnl, aggfunc=sum
    pivot_df = df.pivot_table(index='hour', columns='strategy', values='pnl', aggfunc='sum', fill_value=0.0)
    
    # Add a Total column: (Sum of all strategies)
    pivot_df['总计'] = pivot_df.sum(axis=1)
    
    # Reindex to ensure all 24 hours are present
    pivot_df = pivot_df.reindex(range(24), fill_value=0.0)
    
    output_file = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260219\hourly_pnl_report.txt"
    with open(output_file, "w", encoding="utf-8") as f:
        # Header
        strategies = [col for col in pivot_df.columns if col != '总计'] + ['总计']
        header_str = f"{'Hour':<5} | " + " | ".join(f"{s[:15]:>15}" for s in strategies) + "\n"
        f.write(header_str)
        f.write("-" * 200 + "\n")
        
        for h in range(24):
            row_str = f"{h:02d}:00 | " + " | ".join(f"{pivot_df.loc[h, s]:>15.2f}" for s in strategies) + "\n"
            f.write(row_str)
            
        f.write("-" * 200 + "\n")
        
        totals = pivot_df.sum()
        total_str = f"{'Total':<5} | " + " | ".join(f"{totals[s]:>15.2f}" for s in strategies) + "\n"
        f.write(total_str)

        # Winning Rate calculation
        # Win Rate = (Count of PnL > 0) / (Total count of trades)
        win_rates = {}
        for s in strategies:
            if s == '总计':
                # Total Win Rate across all data points
                all_trades = df['pnl']
                wins = (all_trades > 0).sum()
                total_trades = len(all_trades)
            else:
                strat_trades = df[df['strategy'] == s]['pnl']
                wins = (strat_trades > 0).sum()
                total_trades = len(strat_trades)
            
            rate = (wins / total_trades * 100) if total_trades > 0 else 0
            win_rates[s] = rate
            
        winrate_str = f"{'胜率':<5} | " + " | ".join(f"{win_rates[s]:>14.1f}%" for s in strategies) + "\n"
        f.write(winrate_str)
    
    print(f"Report saved to {output_file}")

if __name__ == "__main__":
    analyze_by_order_time()
