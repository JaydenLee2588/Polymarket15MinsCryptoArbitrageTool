import re
import os
import sys
from collections import defaultdict
import math

def parse_log_line(line):
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
    except:
        minutes = 0.0

    price = 0.0
    pnl = 0.0
    
    if action == "买入":
        quote_match = re.search(r'\"(.*)\"', line)
        if quote_match:
            brackets = re.findall(r'【(.*?)】', quote_match.group(1))
            # Format: [Action][Dir][Amt][Price][Cost]
            # e.g. 【买入】【UP】【4】【0.6800】【$2.72】
            if len(brackets) >= 4:
                try:
                    price = float(brackets[4])
                except:
                    price = 0.0
    
    elif action in ["止盈", "时间止损", "决议", "决议赎回"]:
        # Find PnL (last dollar amount)
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
        "minutes": minutes,
        "price": price,
        "pnl": pnl
    }

def generate_report(log_path):
    if not os.path.exists(log_path):
        print(f"File not found: {log_path}")
        return

    # Extract info for output filename
    # log_path like .../logs/20260204/trade_history_1528.log -> strategy_analysis_1528.md
    # log_path like .../logs/20260204/trade_history_1528_sim.log -> strategy_analysis_1528_sim.md
    dir_name = os.path.dirname(log_path)
    base_name = os.path.basename(log_path)
    
    name_part = base_name.replace("trade_history_", "").replace(".log", "")
    output_filename = f"strategy_analysis_{name_part}.md"
    output_path = os.path.join(dir_name, output_filename)

    open_trades = {}
    
    # Stats structure
    # stats[strat] = { total_trades, total_pnl, wins, resolution_count, sale_count, price_buckets, time_buckets }
    stats = defaultdict(lambda: {
        'total_trades': 0,
        'total_pnl': 0.0,
        'wins': 0,
        'resolution_count': 0,
        'sale_count': 0,
        'price_buckets': defaultdict(lambda: {'count': 0, 'pnl': 0.0}),
        'time_buckets': defaultdict(lambda: {'count': 0, 'pnl': 0.0})
    })

    PRICE_STEP = 0.10

    with open(log_path, 'r', encoding='utf-8') as f:
        for line in f:
            data = parse_log_line(line)
            if not data: continue
            
            strat = data['strategy']
            mkt = data['market']
            action = data['action']
            
            if action == "买入":
                open_trades[(strat, mkt)] = (data['price'], data['minutes'])
            
            elif action in ["止盈", "时间止损", "决议", "决议赎回"]:
                entry_data = open_trades.get((strat, mkt))
                if entry_data:
                    entry_price, entry_mins = entry_data
                    pnl = data['pnl']
                    
                    s = stats[strat]
                    s['total_trades'] += 1
                    s['total_pnl'] += pnl
                    if pnl > 0: s['wins'] += 1
                    
                    if action in ["决议", "决议赎回"]:
                        s['resolution_count'] += 1
                    else:
                        s['sale_count'] += 1
                    
                    # Price Bucket
                    p_lower = math.floor(entry_price / PRICE_STEP) * PRICE_STEP
                    p_key = f"{p_lower:.1f}0 - {p_lower+PRICE_STEP:.1f}0"
                    s['price_buckets'][p_key]['pnl'] += pnl
                    s['price_buckets'][p_key]['count'] += 1
                    
                    # Time Bucket (int minutes)
                    t_key = int(entry_mins)
                    s['time_buckets'][t_key]['pnl'] += pnl
                    s['time_buckets'][t_key]['count'] += 1
                    
                    del open_trades[(strat, mkt)]

    # Generate Markdown
    lines = []
    lines.append(f"# 📊 策略综合分析报告 ({name_part})")
    lines.append(f"**数据源**: `{base_name}`\n")

    for strat in sorted(stats.keys()):
        s = stats[strat]
        trades = s['total_trades']
        if trades == 0: continue
        
        wr = (s['wins'] / trades * 100)
        res_count = s['resolution_count']
        sale_count = s['sale_count']
        res_sale_ratio = f"{res_count}:{sale_count}"
        res_pct = (res_count / trades * 100)
        
        lines.append(f"## 🏆 {strat}")
        lines.append(f"| 指标 | 数值 |")
        lines.append(f"| :--- | :--- |")
        lines.append(f"| **总交易次数** | {trades} |")
        lines.append(f"| **累计盈亏** | **${s['total_pnl']:.2f}** |")
        lines.append(f"| **胜率** | {wr:.1f}% |")
        lines.append(f"| **决议 vs 出售** | {res_sale_ratio} ({res_pct:.0f}% 决议) |")
        lines.append("")
        
        lines.append("### 💰 价格区间聚合")
        lines.append("| 价格区间 | 次数 | PnL | Avg PnL |")
        lines.append("| :--- | :---: | :---: | :---: |")
        for p_key in sorted(s['price_buckets'].keys()):
            b = s['price_buckets'][p_key]
            avg = b['pnl'] / b['count']
            lines.append(f"| {p_key} | {b['count']} | ${b['pnl']:.2f} | ${avg:.2f} |")
        lines.append("")
        
        lines.append("### ⏰ 下单时间聚合")
        lines.append("| 下单时刻 | 次数 | PnL |")
        lines.append("| :--- | :---: | :---: |")
        for t_key in sorted(s['time_buckets'].keys()):
            b = s['time_buckets'][t_key]
            lines.append(f"| {t_key}m - {t_key+1}m | {b['count']} | ${b['pnl']:.2f} |")
        lines.append("\n---\n")

    # Write to file
    with open(output_path, 'w', encoding='utf-8') as f:
        f.write("\n".join(lines))
    
    print(f"Report generated: {output_path}")
    # Also print to stdout for agent to see
    print("\n".join(lines))

if __name__ == "__main__":
    target_log = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260204\trade_history_1528.log"
    if len(sys.argv) > 1:
        target_log = sys.argv[1]
    
    generate_report(target_log)
