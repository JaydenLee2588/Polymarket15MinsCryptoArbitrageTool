import re
import os
from collections import defaultdict

def parse_log_line(line):
    """解析单行日志，提取关键字段"""
    # 格式: 【时间】【策略】【市场】"【分钟】【操作】【方向/数量】【价格】【成本/收入】【PnL?】" "【决议方向?】"
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

    # 提取价格 (第4个【】中的值)
    price = 0.0
    # 买入格式: 【方向】【数量】【价格】【成本】
    # 决议格式: 【数量】【结算价】【收入】【PnL】
    brackets = re.findall(r"【(.*?)】", line)
    
    if action == "买入":
        # brackets: timestamp, strategy, market, minutes, action, side, amount, price, cost, ...
        if len(brackets) >= 8:
            try:
                price = float(brackets[7])  # 价格位置
            except:
                pass
    
    # 提取 PnL (卖出/决议行)
    pnl = 0.0
    if action in ["止盈", "时间止损", "决议", "决议赎回"]:
        # 找所有 $xxx 格式，最后一个是 PnL
        pnl_matches = re.findall(r"【\$([-0-9.]+)】", line)
        if pnl_matches:
            try:
                pnl = float(pnl_matches[-1])
            except ValueError:
                pnl = 0.0

    return {
        "timestamp": timestamp,
        "strategy": strategy,
        "market": market,
        "action": action,
        "minutes": minutes,
        "price": price,
        "pnl": pnl,
        "line": line
    }

def process_log_file(filepath, output_path, price_interval=0.05):
    if not os.path.exists(filepath):
        print(f"File not found: {filepath}")
        return

    # 数据存储
    open_trades = {}  # (strategy, market) -> {minutes, price}
    
    # 统计数据
    strategy_stats = defaultdict(lambda: {"trades": 0, "wins": 0, "pnl": 0.0, "resolutions": 0, "sells": 0})
    price_agg = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0}))  # strategy -> price_bucket -> stats
    time_agg = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0}))   # strategy -> time_bucket -> stats
    real_time_agg = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0})) # strategy -> real_time_bucket -> stats

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
            price = data['price']
            
            if action == "买入":
                # 记录入场信息
                open_trades[(strat, mkt)] = {"minutes": mins, "price": price, "timestamp": data['timestamp']}
            
            elif action in ["止盈", "时间止损", "决议", "决议赎回"]:
                pnl = data['pnl']
                
                # 更新策略统计
                strategy_stats[strat]["trades"] += 1
                strategy_stats[strat]["pnl"] += pnl
                if pnl > 0:
                    strategy_stats[strat]["wins"] += 1
                
                # 区分决议 vs 出售
                if action in ["决议", "决议赎回"]:
                    strategy_stats[strat]["resolutions"] += 1
                else:
                    strategy_stats[strat]["sells"] += 1
                
                # 获取入场信息
                entry_info = open_trades.get((strat, mkt))
                if entry_info:
                    entry_mins = entry_info["minutes"]
                    entry_price = entry_info["price"]
                    entry_time = entry_info.get("timestamp", data['timestamp']) # Fallback to current if missing
                    
                    # 时间聚合 (30秒为一个桶)
                    seconds_total = int(entry_mins * 60)
                    bucket_start = (seconds_total // 30) * 30
                    time_agg[strat][bucket_start]["count"] += 1
                    time_agg[strat][bucket_start]["pnl"] += pnl
                    if pnl > 0:
                        time_agg[strat][bucket_start]["wins"] += 1
                    
                    # 价格聚合 (0.02为一个桶)
                    if entry_price > 0:
                        price_bucket = round(entry_price / price_interval) * price_interval
                        price_agg[strat][price_bucket]["count"] += 1
                        price_agg[strat][price_bucket]["pnl"] += pnl
                        if pnl > 0:
                            price_agg[strat][price_bucket]["wins"] += 1
                    
                    # 实时时间聚合 (30分钟为一个桶) - 使用买入时间
                    # timestamp format "HH:MM:SS"
                    try:
                        t_parts = entry_time.split(':')
                        h = int(t_parts[0])
                        m = int(t_parts[1])
                        # 30 min bucket
                        bucket_m = 0 if m < 30 else 30
                        rt_key = f"{h:02d}:{bucket_m:02d}"
                        real_time_agg[strat][rt_key]["count"] += 1
                        real_time_agg[strat][rt_key]["pnl"] += pnl
                        if pnl > 0:
                            real_time_agg[strat][rt_key]["wins"] += 1
                    except:
                        pass
                    
                    del open_trades[(strat, mkt)]

    # 生成报告
    report = []
    report.append(f"# 策略分析报告")
    report.append(f"\n数据来源: `{os.path.basename(filepath)}`\n")
    
    # 1. 策略总览
    report.append("## 1. 策略总览\n")
    report.append("| 策略 | 交易次数 | 胜 | 负 | 胜率 | 总PnL | 决议 | 出售 | 决议比例 |")
    report.append("| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |")
    
    all_strats = sorted(strategy_stats.keys())
    total_trades = 0
    total_pnl = 0.0
    
    for strat in all_strats:
        s = strategy_stats[strat]
        trades = s["trades"]
        wins = s["wins"]
        losses = trades - wins
        win_rate = (wins / trades * 100) if trades > 0 else 0
        pnl = s["pnl"]
        resolutions = s["resolutions"]
        sells = s["sells"]
        res_ratio = (resolutions / trades * 100) if trades > 0 else 0
        
        total_trades += trades
        total_pnl += pnl
        
        pnl_str = f"${pnl:.2f}" if pnl >= 0 else f"-${abs(pnl):.2f}"
        report.append(f"| {strat} | {trades} | {wins} | {losses} | {win_rate:.1f}% | {pnl_str} | {resolutions} | {sells} | {res_ratio:.1f}% |")
    
    total_pnl_str = f"${total_pnl:.2f}" if total_pnl >= 0 else f"-${abs(total_pnl):.2f}"
    report.append(f"| **总计** | **{total_trades}** | - | - | - | **{total_pnl_str}** | - | - | - |")
    
    # 2. 按价格聚合
    report.append("\n## 2. 按下单价格聚合\n")
    
    for strat in all_strats:
        report.append(f"### {strat}\n")
        report.append("| 价格区间 | 交易次数 | 胜率 | 累计PnL |")
        report.append("| :---: | :---: | :---: | :---: |")
        
        buckets = sorted(price_agg[strat].keys())
        strat_total_count = 0
        strat_total_pnl = 0.0
        
        for b in buckets:
            count = price_agg[strat][b]["count"]
            pnl = price_agg[strat][b]["pnl"]
            wins = price_agg[strat][b]["wins"]
            win_rate = (wins / count * 100) if count > 0 else 0
            
            strat_total_count += count
            strat_total_pnl += pnl
            pnl_str = f"${pnl:.2f}" if pnl >= 0 else f"-${abs(pnl):.2f}"
            report.append(f"| {b:.2f} - {b+price_interval:.2f} | {count} | {win_rate:.1f}% | {pnl_str} |")
        
        strat_total_pnl_str = f"${strat_total_pnl:.2f}" if strat_total_pnl >= 0 else f"-${abs(strat_total_pnl):.2f}"
        report.append(f"| **合计** | **{strat_total_count}** | - | **{strat_total_pnl_str}** |")
        report.append("")
    
    # 3. 按开盘时间聚合 (30秒粒度)
    report.append("\n## 3. 按距离开盘时间聚合 (30s)\n")
    
    for strat in all_strats:
        report.append(f"### {strat}\n")
        report.append("| 开盘后时间 | 交易次数 | 胜率 | 累计PnL |")
        report.append("| :---: | :---: | :---: | :---: |")
        
        buckets = sorted(time_agg[strat].keys())
        strat_total_count = 0
        strat_total_pnl = 0.0
        
        for b in buckets:
            count = time_agg[strat][b]["count"]
            pnl = time_agg[strat][b]["pnl"]
            wins = time_agg[strat][b]["wins"]
            win_rate = (wins / count * 100) if count > 0 else 0
            
            strat_total_count += count
            strat_total_pnl += pnl
            pnl_str = f"${pnl:.2f}" if pnl >= 0 else f"-${abs(pnl):.2f}"
            
            start_m = b // 60
            start_s = b % 60
            end_total = b + 30
            end_m = end_total // 60
            end_s = end_total % 60
            
            time_str = f"{start_m}m{start_s:02d}s - {end_m}m{end_s:02d}s"
            report.append(f"| {time_str} | {count} | {win_rate:.1f}% | {pnl_str} |")
        
        strat_total_pnl_str = f"${strat_total_pnl:.2f}" if strat_total_pnl >= 0 else f"-${abs(strat_total_pnl):.2f}"
        report.append(f"| **合计** | **{strat_total_count}** | - | **{strat_total_pnl_str}** |")
        report.append("")

    # 4. 按实际时间聚合 (30分钟粒度)
    report.append("\n## 4. 按实际下单时间聚合 (30min)\n")
    
    for strat in all_strats:
        report.append(f"### {strat}\n")
        report.append("| 下单时段 | 交易次数 | 胜率 | 累计PnL |")
        report.append("| :---: | :---: | :---: | :---: |")
        
        buckets = sorted(real_time_agg[strat].keys())
        strat_total_count = 0
        strat_total_pnl = 0.0
        
        for b in buckets:
            count = real_time_agg[strat][b]["count"]
            pnl = real_time_agg[strat][b]["pnl"]
            wins = real_time_agg[strat][b]["wins"]
            win_rate = (wins / count * 100) if count > 0 else 0
            
            strat_total_count += count
            strat_total_pnl += pnl
            pnl_str = f"${pnl:.2f}" if pnl >= 0 else f"-${abs(pnl):.2f}"
            
            # b is "HH:MM" start time
            # calculate end time
            h, m = map(int, b.split(':'))
            end_m = m + 30
            end_h = h
            if end_m >= 60:
                end_m -= 60
                end_h += 1
            if end_h >= 24:
                end_h -= 24
            
            time_str = f"{b} - {end_h:02d}:{end_m:02d}"
            
            report.append(f"| {time_str} | {count} | {win_rate:.1f}% | {pnl_str} |")
        
        strat_total_pnl_str = f"${strat_total_pnl:.2f}" if strat_total_pnl >= 0 else f"-${abs(strat_total_pnl):.2f}"
        report.append(f"| **合计** | **{strat_total_count}** | - | **{strat_total_pnl_str}** |")
        report.append("")
    
    # 写入文件
    with open(output_path, 'w', encoding='utf-8') as f:
        f.write('\n'.join(report))
    
    print(f"报告已生成: {output_path}")

import sys

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python analyze_trade_history.py <log_file> <output_file>")
        # Fallback for testing/default
        # log_file = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260205\trade_history_1844.log"
        # output_file = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260205\strategy_analysis_1844.md"
        sys.exit(1)
    else:
        log_file = sys.argv[1]
        output_file = sys.argv[2]
        price_interval = float(sys.argv[3]) if len(sys.argv) > 3 else 0.05
        
    process_log_file(log_file, output_file, price_interval)
