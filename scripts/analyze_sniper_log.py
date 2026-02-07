import re
import os
import sys
from collections import defaultdict

def parse_log_line(line):
    """解析单行日志，提取关键字段"""
    # 格式: 【时间】【策略】【市场】"【分钟】【操作】【方向/数量】【价格】【成本/收入】【PnL?】" "【决议方向?】"
    prefix_pattern = r"【(.*?)】【(.*?)】【(.*?)】\"【(.*?)分?】【(.*?)】"
    match = re.search(prefix_pattern, line)
    if not match:
        return None
    
    timestamp = match.group(1)
    strategy = match.group(2)
    market = match.group(3)
    minutes_str = match.group(4).replace('分', '')
    action = match.group(5)
    
    try:
        minutes = float(minutes_str)
    except ValueError:
        minutes = 0.0

    # 提取价格 (第4个【】中的值)
    price = 0.0
    brackets = re.findall(r"【(.*?)】", line)
    
    if action == "买入":
        if len(brackets) >= 8:
            try:
                price = float(brackets[7])
            except:
                pass
    elif action in ["止盈", "时间止损", "决议", "决议赎回"] or "Sniper" in action:
         if len(brackets) >= 8:
            try:
                price = float(brackets[7])
            except:
                pass

    # 提取 PnL (卖出/决议行)
    pnl = 0.0
    pnl_matches = re.findall(r"【\$([-0-9.]+)】", line)
    
    is_sell = False
    if "Sniper" in action or action in ["止盈", "时间止损", "卖出"]:
        is_sell = True
        if pnl_matches:
            try:
                pnl = float(pnl_matches[-1])
            except ValueError:
                pnl = 0.0
    
    elif action in ["决议", "决议赎回"]:
        is_sell = True
        if pnl_matches:
            try:
                pnl = float(pnl_matches[-1])
            except:
                pnl = 0.0

    return {
        "timestamp": timestamp,
        "strategy": strategy,
        "market": market,
        "action": action,
        "minutes": minutes,
        "price": price,
        "pnl": pnl,
        "is_sell": is_sell,
        "line": line
    }

def parse_yaml_config(config_path):
    """简单的正则解析 YAML 配置，提取 Sniper 参数"""
    if not os.path.exists(config_path):
        return {}
    
    config_data = {}
    current_id = ""
    
    with open(config_path, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            if line.startswith("- id:"):
                # 提取 ID: - id: "Sniper-C-死鱼"
                match = re.search(r'- id:\s*["\'](.*?)["\']', line)
                if match:
                    current_id = match.group(1)
                    config_data[current_id] = {}
            
            elif current_id and ":" in line and not line.startswith("#"):
                # 提取参数: zscore_entry_threshold: 3.0
                parts = line.split(":", 1)
                key = parts[0].strip()
                val_str = parts[1].strip().split("#")[0].strip() # 去掉注释
                
                try:
                    val = float(val_str)
                    config_data[current_id][key] = val
                except:
                    pass
    return config_data

def generate_recommendations(stats, config):
    """根据统计数据和配置生成建议"""
    recs = []
    
    strat_name = stats['name']
    count = stats['count']
    win_rate = stats['win_rate']
    pnl = stats['pnl']
    
    cfg = config.get(strat_name, {})
    
    z_entry = cfg.get('zscore_entry_threshold', -1)
    edge_thresh = cfg.get('edge_threshold', -1)
    dead_fish = cfg.get('dead_fish_min_edge', -1)
    
    # 1. 交易频率建议
    if count == 0:
        recs.append(f"🔴 **零交易**: 建议放宽准入门槛。")
        if z_entry > 2.0:
            recs.append(f"  - `zscore_entry_threshold` ({z_entry}) 可能太高，建议降至 {z_entry-0.5:.1f}")
        if edge_thresh > 0.05:
            recs.append(f"  - `edge_threshold` ({edge_thresh}) 可能太高，建议降至 {edge_thresh-0.02:.2f}")
    elif count < 5:
        recs.append(f"🟠 **低频交易**: 样本太少。")
        if z_entry > 2.5:
             recs.append(f"  - 考虑微调 `zscore_entry_threshold` 从 {z_entry} -> {z_entry-0.2:.1f}")
    
    # 2. 胜率建议
    if count > 5:
        if win_rate < 40:
            recs.append(f"🔴 **胜率过低 ({win_rate:.1f}%)**: 策略在亏损。")
            recs.append(f"  - 建议 **提高** `edge_threshold` (当前: {edge_thresh}) 以过滤低质量机会。")
            recs.append(f"  - 建议 **收紧** `zscore_entry_threshold` (当前: {z_entry})。")
        elif win_rate > 80:
            recs.append(f"🟢 **胜率极高 ({win_rate:.1f}%)**: 表现优异。")
            recs.append(f"  - 可以尝试 **增加** `leg1_share_amount_1` (加大注码)。")
            recs.append(f"  - 或者微调降低门槛以获取更多交易机会。")
    
    # 3. PnL 建议
    if pnl < 0:
        recs.append(f"📉 **亏损状态**: 总 PnL {pnl:.2f}。请检查是否滑点过大或止损太频繁。")
        
    if not recs:
        recs.append("⚪ **表现平稳**: 暂无特定建议。")
        
    return recs

def process_log_file(filepath, output_path, config_path=None):
    if not os.path.exists(filepath):
        print(f"File not found: {filepath}")
        return

    # 解析配置
    configs = {}
    if config_path:
        configs = parse_yaml_config(config_path)

    # 数据存储
    open_positions = {}
    strategy_stats = defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0, "resolutions": 0, "sells": 0})
    price_agg = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0}))
    time_agg = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0}))

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
                open_positions[(strat, mkt)] = {
                    "buy_price": data['price'],
                    "buy_minutes": mins
                }
                
            elif data['is_sell']:
                pnl = data['pnl']
                strategy_stats[strat]["count"] += 1
                strategy_stats[strat]["pnl"] += pnl
                if pnl > 0:
                    strategy_stats[strat]["wins"] += 1
                
                if action in ["决议", "决议赎回"]:
                    strategy_stats[strat]["resolutions"] += 1
                else:
                    strategy_stats[strat]["sells"] += 1
                
                buy_info = open_positions.get((strat, mkt))
                if buy_info:
                    buy_price = buy_info['buy_price']
                    buy_mins = buy_info['buy_minutes']
                    
                    if buy_price > 0:
                        bucket_price = int(buy_price / 0.05) * 0.05
                        price_agg[strat][bucket_price]["count"] += 1
                        price_agg[strat][bucket_price]["pnl"] += pnl
                        if pnl > 0:
                            price_agg[strat][bucket_price]["wins"] += 1
                    
                    total_seconds = int(buy_mins * 60)
                    bucket_time = (total_seconds // 10) * 10
                    time_agg[strat][bucket_time]["count"] += 1
                    time_agg[strat][bucket_time]["pnl"] += pnl
                    if pnl > 0:
                        time_agg[strat][bucket_time]["wins"] += 1

    # 生成报告
    lines = []
    lines.append(f"# 策略分析报告")
    lines.append(f"数据源: `{os.path.basename(filepath)}`")
    lines.append("")
    
    # 1. 策略总览 & 建议
    lines.append("## 1. 策略总览与调优建议")
    lines.append("| 策略 | 次数 | PnL | 胜率 | 决议/出售 |")
    lines.append("|---|---|---|---|---|")
    
    sorted_strats = sorted(strategy_stats.keys())
    for s in sorted_strats:
        stat = strategy_stats[s]
        cnt = stat['count']
        pnl = stat['pnl']
        wins = stat['wins']
        win_rate = (wins/cnt*100) if cnt > 0 else 0.0
        
        pnl_str = f"${pnl:.2f}"
        if pnl > 0: pnl_str = f"+${pnl:.2f}"
        
        lines.append(f"| {s} | {cnt} | {pnl_str} | {win_rate:.1f}% | {stat['resolutions']}/{stat['sells']} |")
    
    lines.append("")
    lines.append("### 💡 调优建议")
    for s in sorted_strats:
        stat = strategy_stats[s]
        stat['name'] = s
        stat['win_rate'] = (stat['wins']/stat['count']*100) if stat['count'] > 0 else 0.0
        
        recs = generate_recommendations(stat, configs)
        if recs:
            lines.append(f"#### {s}")
            for r in recs:
                lines.append(f"- {r}")
            lines.append("")

    # 2. Price Aggregation
    lines.append("## 2. 按下单价格聚合")
    for s in sorted_strats:
        lines.append(f"### {s}")
        lines.append("| 价格区间 | 次数 | PnL | 胜率 |")
        lines.append("|---|---|---|---|")
        
        buckets = sorted(price_agg[s].keys())
        for b in buckets:
            stat = price_agg[s][b]
            cnt = stat['count']
            pnl = stat['pnl']
            wins = stat['wins']
            win_rate = (wins/cnt*100) if cnt > 0 else 0.0
            pnl_str = f"${pnl:.2f}" if pnl < 0 else f"+${pnl:.2f}"
            range_str = f"{b:.2f}-{b+0.05:.2f}"
            lines.append(f"| {range_str} | {cnt} | {pnl_str} | {win_rate:.1f}% |")
        lines.append("")

    # 3. Time Aggregation
    lines.append("## 3. 按下单时间聚合")
    for s in sorted_strats:
        lines.append(f"### {s}")
        lines.append("| 时间区间 (与开始偏移) | 次数 | PnL | 胜率 |")
        lines.append("|---|---|---|---|")
        
        buckets = sorted(time_agg[s].keys())
        for b in buckets:
            stat = time_agg[s][b]
            cnt = stat['count']
            pnl = stat['pnl']
            wins = stat['wins']
            win_rate = (wins/cnt*100) if cnt > 0 else 0.0
            pnl_str = f"${pnl:.2f}" if pnl < 0 else f"+${pnl:.2f}"
            
            start_min, start_sec = b // 60, b % 60
            end_total = b + 10
            end_min, end_sec = end_total // 60, end_total % 60
            time_str = f"{start_min}m{start_sec:02d}s - {end_min}m{end_sec:02d}s"
            
            lines.append(f"| {time_str} | {cnt} | {pnl_str} | {win_rate:.1f}% |")
        lines.append("")

    with open(output_path, 'w', encoding='utf-8') as f:
        f.write('\n'.join(lines))
    print(f"Analysis saved to {output_path}")

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python analyze_sniper_log.py <input_log> <output_md> [config_yaml]")
        sys.exit(1)
    
    input_log = sys.argv[1]
    output_md = sys.argv[2]
    config_yaml = sys.argv[3] if len(sys.argv) > 3 else None
    
    process_log_file(input_log, output_md, config_yaml)
