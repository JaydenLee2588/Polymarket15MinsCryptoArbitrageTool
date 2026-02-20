import re
import os
import sys
import yaml
import math
from collections import defaultdict
from datetime import datetime

def parse_log_line(line):
    """
    解析日志行
    格式: 【HH:MM:SS】【Session】【Strategy】【Market】"【Score】【Action】【Side】【Amount】【Price】【Cost/PnL】" ...
    """
    # 提取前缀: 【14:27:00】【美股盘】【Pattern策略5】...
    prefix_match = re.match(r"【(.*?)】【(.*?)】【(.*?)】【(.*?)】", line)
    if not prefix_match:
        return None
    
    timestamp_str = prefix_match.group(1) # 14:27:00
    session = prefix_match.group(2)       # 美股盘
    strategy = prefix_match.group(3)      # Pattern策略5
    market = prefix_match.group(4)        # sol-updown-15m...
    
    # 提取引号内的内容
    # 示例: "【12.0分】【买入】【DOWN】【5】【0.5400】【$2.70】"
    # 示例: "【19.0分】【决议赎回】【3】【1.0000】【$3.00】【$0.21】"
    content_match = re.search(r"\"(.*?)\"", line)
    if not content_match:
        return None
        
    content = content_match.group(1)
    brackets = re.findall(r"【(.*?)】", content)
    
    # 解析括号内容
    # 0: Score/Minutes (e.g. 12.0分)
    # 1: Action (买入, 决议赎回, 止盈, 模拟失败...)
    # 2: Side/Reason (DOWN, UP, 深度不足...) - 注意：失败时格式不同
    # 
    # 买入: 【12.0分】【买入】【DOWN】【5】【0.5400】【$2.70】
    # 卖出/决议: 【19.0分】【决议赎回】【3】【1.0000】【$3.00】【$0.21】
    
    if len(brackets) < 2:
        return None

    score_str = brackets[0].replace("分", "")
    try:
        minutes_in = float(score_str)
    except:
        minutes_in = 0.0
        
    action = brackets[1]
    
    price = 0.0
    pnl = 0.0
    side = ""
    
    if action == "买入":
        if len(brackets) >= 5:
            side = brackets[2]
            try:
                price = float(brackets[4])
            except:
                price = 0.0
    
    elif action in ["止盈", "时间止损", "决议", "决议赎回"]:
        # 查找 PnL (通常是最后一个带$的)
        # brackets: ['19.0分', '决议赎回', '3', '1.0000', '$3.00', '$0.21']
        for b in reversed(brackets):
            if b.startswith("$"):
                try:
                    pnl = float(b.replace("$", ""))
                    break
                except:
                    continue
    
    return {
        "timestamp": timestamp_str,
        "session": session,
        "strategy": strategy,
        "market": market,
        "action": action,
        "minutes_in": minutes_in,
        "price": price,
        "pnl": pnl,
        "side": side
    }

def analyze_log(log_path, config_files, output_path):
    print(f"开始分析: {log_path}")
    
    # 数据结构
    stats = defaultdict(lambda: {
        "count": 0, "wins": 0, "pnl": 0.0, 
        "resolutions": 0, "sells": 0
    })
    
    # 聚合数据
    agg_price = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0}))
    agg_time_open = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0}))
    agg_time_clock = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0}))
    agg_session = defaultdict(lambda: defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0}))
    agg_session_global = defaultdict(lambda: {"count": 0, "pnl": 0.0, "wins": 0}) # 全局盘口
    
    # 记录开仓信息: (Strategy, Market) -> Entry Data
    entries = {}
    
    # 读取日志
    with open(log_path, 'r', encoding='utf-8') as f:
        for line in f:
            if not line.strip(): continue
            try:
                data = parse_log_line(line)
            except:
                continue
                
            if not data: continue
            
            s = data['strategy']
            mkt = data['market']
            
            if data['action'] == "买入":
                # 记录开仓
                entries[(s, mkt)] = data
                
            elif data['action'] in ["止盈", "时间止损", "决议", "决议赎回"]:
                # 结算
                pnl = data['pnl']
                
                stats[s]["count"] += 1
                stats[s]["pnl"] += pnl
                if pnl > 0: stats[s]["wins"] += 1
                
                if data['action'] in ["决议", "决议赎回"]:
                    stats[s]["resolutions"] += 1
                else:
                    stats[s]["sells"] += 1
                
                # 获取对应的开仓信息
                entry = entries.get((s, mkt))
                
                if entry:
                    # 1. 价格聚合 (粒度 $0.02)
                    price = entry['price']
                    p_bucket = math.floor(price / 0.02) * 0.02
                    agg_price[s][p_bucket]["count"] += 1
                    agg_price[s][p_bucket]["pnl"] += pnl
                    if pnl > 0: agg_price[s][p_bucket]["wins"] += 1
                    
                    # 2. 开盘后时间聚合 (粒度 30s = 0.5m)
                    t_val = entry['minutes_in']
                    t_bucket = math.floor(t_val / 0.5) * 0.5
                    agg_time_open[s][t_bucket]["count"] += 1
                    agg_time_open[s][t_bucket]["pnl"] += pnl
                    if pnl > 0: agg_time_open[s][t_bucket]["wins"] += 1
                    
                    # 3. UTC-5 下单时间聚合 (粒度 1小时)
                    # 格式 14:27:00 -> 14
                    try:
                        hh = int(entry['timestamp'].split(':')[0])
                        agg_time_clock[s][hh]["count"] += 1
                        agg_time_clock[s][hh]["pnl"] += pnl
                        if pnl > 0: agg_time_clock[s][hh]["wins"] += 1
                    except:
                        pass
                        
                    # 4. 盘口聚合
                    sess = entry['session']
                    # 分策略
                    agg_session[s][sess]["count"] += 1
                    agg_session[s][sess]["pnl"] += pnl
                    if pnl > 0: agg_session[s][sess]["wins"] += 1
                    # 全局
                    agg_session_global[sess]["count"] += 1
                    agg_session_global[sess]["pnl"] += pnl
                    if pnl > 0: agg_session_global[sess]["wins"] += 1
                    
                    # 清理
                    del entries[(s, mkt)]
    
    # 加载配置
    configs = []
    if config_files:
        for cf in config_files:
            try:
                with open(cf, 'r', encoding='utf-8') as f:
                    c_data = yaml.safe_load(f)
                    if c_data and 'groups' in c_data:
                        configs.extend(c_data['groups'])
            except Exception as e:
                print(f"Warning: Failed to load config {cf}: {e}")

    # 生成报告
    lines = []
    lines.append(f"# 📊 策略详细分析报告")
    lines.append(f"**生成时间**: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    lines.append(f"**数据源**: `{os.path.basename(log_path)}`")
    lines.append(f"> **注意**: 所有时间均为 UTC-5 (美东时间)。")
    lines.append("")
    
    # 1. 策略核心指标
    lines.append("## 1. 策略核心表现")
    lines.append("| 策略 | 交易数 | 胜率 | 总PnL | 决议/总量 | 出售/总量 |")
    lines.append("|---|---|---|---|---|---|")
    
    sorted_strats = sorted(stats.keys())
    for s in sorted_strats:
        d = stats[s]
        win_rate = (d['wins'] / d['count'] * 100) if d['count'] else 0
        res_rate = (d['resolutions'] / d['count'] * 100) if d['count'] else 0
        sell_rate = (d['sells'] / d['count'] * 100) if d['count'] else 0
        
        lines.append(f"| {s} | {d['count']} | {win_rate:.1f}% | ${d['pnl']:.2f} | {d['resolutions']} ({res_rate:.0f}%) | {d['sells']} ({sell_rate:.0f}%) |")
    lines.append("")
    
    # 2. 下单价格聚合
    lines.append("## 2. 下单价格分布 (粒度 $0.02)")
    for s in sorted_strats:
        if not agg_price[s]: continue
        lines.append(f"### {s}")
        lines.append("| 价格区间 | 次数 | PnL | 胜率 |")
        lines.append("|---|---|---|---|")
        for p in sorted(agg_price[s].keys()):
            d = agg_price[s][p]
            wr = (d['wins']/d['count']*100) if d['count'] else 0
            lines.append(f"| {p:.2f}-{p+0.02:.2f} | {d['count']} | ${d['pnl']:.2f} | {wr:.1f}% |")
        lines.append("")

    # 3. 开盘后时间聚合
    lines.append("## 3. 开盘后下单时间 (粒度 30s)")
    for s in sorted_strats:
        if not agg_time_open[s]: continue
        lines.append(f"### {s}")
        lines.append("| 开盘后(分) | 次数 | PnL | 胜率 |")
        lines.append("|---|---|---|---|")
        for t in sorted(agg_time_open[s].keys()):
            d = agg_time_open[s][t]
            wr = (d['wins']/d['count']*100) if d['count'] else 0
            lines.append(f"| {t:.1f}-{t+0.5:.1f}m | {d['count']} | ${d['pnl']:.2f} | {wr:.1f}% |")
        lines.append("")

    # 4. UTC-5 时间聚合
    lines.append("## 4. 下单时段分布 (UTC-5, 粒度 1h)")
    for s in sorted_strats:
        if not agg_time_clock[s]: continue
        lines.append(f"### {s}")
        lines.append("| 时段 (Hour) | 次数 | PnL | 胜率 |")
        lines.append("|---|---|---|---|")
        for h in sorted(agg_time_clock[s].keys()):
            d = agg_time_clock[s][h]
            wr = (d['wins']/d['count']*100) if d['count'] else 0
            lines.append(f"| {h:02d}:00 | {d['count']} | ${d['pnl']:.2f} | {wr:.1f}% |")
        lines.append("")

    # 5. 盘口聚合 (分策略)
    lines.append("## 5. 盘口分布 (分策略)")
    for s in sorted_strats:
        if not agg_session[s]: continue
        lines.append(f"### {s}")
        lines.append("| 盘口 | 次数 | PnL | 胜率 |")
        lines.append("|---|---|---|---|")
        for sess in sorted(agg_session[s].keys()):
            d = agg_session[s][sess]
            wr = (d['wins']/d['count']*100) if d['count'] else 0
            lines.append(f"| {sess} | {d['count']} | ${d['pnl']:.2f} | {wr:.1f}% |")
        lines.append("")

    # 6. 盘口聚合 (全局)
    lines.append("## 6. 盘口分布 (全局)")
    lines.append("| 盘口 | 次数 | PnL | 胜率 |")
    lines.append("|---|---|---|---|")
    for sess in sorted(agg_session_global.keys()):
        d = agg_session_global[sess]
        wr = (d['wins']/d['count']*100) if d['count'] else 0
        lines.append(f"| {sess} | {d['count']} | ${d['pnl']:.2f} | {wr:.1f}% |")
    lines.append("")
    
    # 7. 参数优化建议
    lines.append("## 7. 参数调整建议")
    if not configs:
        lines.append("> 未提供或无法加载配置文件，跳过参数建议。")
    else:
        for s in sorted_strats:
            # 查找配置
            s_cfg = next((c for c in configs if c['id'] == s), None)
            if not s_cfg: continue
            
            lines.append(f"### {s}")
            d = stats[s]
            
            # 建议逻辑
            suggestions = []
            
            # A. 检查 Trend Window
            # 查看 agg_time_open 前1分钟的表现
            early_pnl = 0
            if s in agg_time_open:
                for t in [0.0, 0.5]:
                    if t in agg_time_open[s]:
                        early_pnl += agg_time_open[s][t]['pnl']
            
            curr_start = s_cfg.get('trend_window_sec_start', 0)
            if early_pnl < -5.0: # 阈值:如果前1分钟亏损超过5刀
                suggestions.append(f"- **开盘波动风险**: 前1分钟 PnL=${early_pnl:.2f}。建议推迟 `trend_window_sec_start` (当前: {curr_start}s) 至 {curr_start+30}s 或更晚。")
            
            # B. 检查 Sell Threshold vs Resolution
            res_rate = (d['resolutions'] / d['count']) if d['count'] else 0
            sell_thresh = s_cfg.get('sell_threshold', 'N/A')
            
            if res_rate > 0.8:
                suggestions.append(f"- **高持有率 ({res_rate:.0%})**: 绝大多数订单持有至决议。如果 PnL 不佳，可能是 `sell_threshold` ({sell_thresh}) 设置过高，导致无法止盈。建议降低阈值以增加流动性。")
            elif res_rate < 0.2:
                 suggestions.append(f"- **高周转率**: 大部分订单在盘中出售。请检查滑点成本是否过高。")
            
            # C. 检查价格区间
            # 如果某个低价区胜率很低
            if s in agg_price:
                low_price_wins = 0
                low_price_count = 0
                for p in agg_price[s]:
                    if p < 0.50:
                        low_price_wins += agg_price[s][p]['wins']
                        low_price_count += agg_price[s][p]['count']
                
                if low_price_count > 5:
                    lp_wr = low_price_wins / low_price_count
                    if lp_wr < 0.4:
                        suggestions.append(f"- **低价区表现不佳**: <$0.50 的订单胜率仅 {lp_wr:.0%}。建议调整 `leg0_share_price` 或 `tier` 阈值，避免在低价位激进接盘。")

            if not suggestions:
                suggestions.append("- 暂无明显调整建议。")
                
            for sug in suggestions:
                lines.append(sug)
            lines.append("")

    # 写入文件
    with open(output_path, 'w', encoding='utf-8') as f:
        f.write("\n".join(lines))
        
    print(f"报告已生成: {output_path}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python analyze_strategy_full.py <log_path> [config_yaml1 config_yaml2 ...]")
        sys.exit(1)
        
    log_file = sys.argv[1]
    
    # 自动生成输出文件名
    # logs/20260210/trade_history_constract.log -> logs/20260210/strategy_analysis_constract.md
    dir_name = os.path.dirname(log_file)
    base_name = os.path.basename(log_file)
    name_part = base_name.replace("trade_history_", "").replace(".log", "")
    
    # 获取 HHMM (如果有的话) - 用户请求 "strategy_analysis_HHMM.md"
    # 如果文件名是 trade_history_constract.log，那 HHMM 就是 constract?
    # 用户说 "HHMM是分析的LOG文件名的时间 或 后缀名称" -> OK.
    
    out_name = f"strategy_analysis_{name_part}.md"
    out_path = os.path.join(dir_name, out_name)
    
    config_files = sys.argv[2:] if len(sys.argv) > 2 else []
    
    analyze_log(log_file, config_files, out_path)
