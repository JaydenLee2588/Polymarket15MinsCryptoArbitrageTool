import re
import sys
import os
from collections import defaultdict

def strip_ansi(text):
    ansi_escape = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')
    return ansi_escape.sub('', text)

def parse_log(log_path):
    data = defaultdict(list)
    current_time = None
    
    # Regex for timestamp: "2026/02/04 20:29:22"
    time_pat = re.compile(r'^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})')
    
    print(f"Reading log file: {log_path}")
    
    with open(log_path, 'r', encoding='utf-8') as f:
        for line in f:
            # Strip ANSI and whitespace
            line_clean = strip_ansi(line).strip()
            
            # Check for timestamp at start of line
            time_match = time_pat.match(line_clean)
            if time_match:
                current_time = time_match.group(1)
                
            # Parse ticker line
            # Format: [BTC] RSI=... 建议=【观望】 PM=0.99/0.02
            # We look for lines starting with [SYMBOL]
            if line_clean.startswith("["):
                # Regex to extract Symbol, Suggestion, and Prices
                # Example: [BTC] ... 建议=【买跌】 ... PM=0.97/0.04
                # Note: prices are YES/NO.
                # PM=YES_PRICE/NO_PRICE
                
                # Check if it has "建议=" to confirm it's a ticker line we care about
                if "建议=" in line_clean and "PM=" in line_clean:
                    try:
                        # Extract Symbol
                        symbol_match = re.match(r'^\[([A-Z]+)\]', line_clean)
                        if not symbol_match:
                            continue
                        symbol = symbol_match.group(1)
                        
                        # Extract Suggestion
                        sug_match = re.search(r'建议=\s*[【\[](.*?)[】\]]', line_clean)
                        suggestion = sug_match.group(1) if sug_match else "UNKNOWN"
                        
                        # Extract Prices
                        price_match = re.search(r'PM=([0-9.]+)/([0-9.]+)', line_clean)
                        if price_match:
                            price_yes = float(price_match.group(1))
                            price_no = float(price_match.group(2))
                            
                            if current_time:
                                data[symbol].append({
                                    'time': current_time,
                                    'symbol': symbol,
                                    'suggestion': suggestion,
                                    'yes': price_yes,
                                    'no': price_no,
                                    'line': line_clean # for debug
                                })
                    except Exception as e:
                        print(f"Error parsing line: {line_clean}\nError: {e}")

    return data

def simulate_strategy(data):
    results = {}
    
    for symbol, records in data.items():
        analyzed_records = []
        # Sort by time
        records.sort(key=lambda x: x['time'])
        
        trades = []
        
        # We assume the log sampling interval is roughly consistent (30s)
        # Strategy:
        # If Suggestion == "买涨" (Buy UP) -> Buy YES at current Price -> Sell at NEXT heartbeat Price YES
        # If Suggestion == "买跌" (Buy DOWN) -> Buy NO at current Price -> Sell at NEXT heartbeat Price NO
        # If Suggestion == "观望" (Wait) -> No Trade
        
        total_pnl = 0.0
        wins = 0
        total_trades = 0
        
        for i in range(len(records) - 1):
            curr = records[i]
            nxt = records[i+1]
            
            action = curr['suggestion']
            
            entry_price = 0.0
            exit_price = 0.0
            pnl = 0.0
            trade_type = None # "UP" or "DOWN"
            
            if action == "买涨":
                trade_type = "UP"
                entry_price = curr['yes']
                exit_price = nxt['yes']
                # PnL = Exit - Entry
                pnl = exit_price - entry_price
                
            elif action == "买跌":
                trade_type = "DOWN"
                entry_price = curr['no']
                exit_price = nxt['no']
                # PnL = Exit - Entry
                pnl = exit_price - entry_price
            
            if trade_type:
                total_trades += 1
                if pnl > 0:
                    wins += 1
                elif pnl == 0:
                    # Treat flat as neutral, not win
                    pass
                
                total_pnl += pnl
                
                analyzed_records.append({
                    'time_entry': curr['time'],
                    'time_exit': nxt['time'],
                    'symbol': symbol,
                    'action': action,
                    'entry_price': entry_price,
                    'exit_price': exit_price,
                    'pnl': pnl
                })
        
        results[symbol] = {
            'trades': analyzed_records,
            'stats': {
                'total_trades': total_trades,
                'wins': wins,
                'total_pnl': total_pnl,
                'win_rate': (wins / total_trades * 100) if total_trades > 0 else 0
            }
        }
        
    return results

def generate_report(results, output_path):
    lines = []
    lines.append("# 🤖 '无脑'策略回测报告 (Dumb Strategy Simulation)")
    lines.append("> 策略规则：")
    lines.append("> 1. 当建议【买涨】时，买入 YES 分额，在下一个的心跳（约30秒后）无条件卖出。")
    lines.append("> 2. 当建议【买跌】时，买入 NO 分额，在下一个的心跳（约30秒后）无条件卖出。")
    lines.append("> 3. 当建议【观望】时，不进行操作（若持有仓位则在上一周期已卖出）。")
    lines.append("")
    
    grand_total_trades = 0
    grand_total_wins = 0
    grand_total_pnl = 0.0
    
    # Summary Table
    lines.append("## 📊 总体统计")
    lines.append("| 代币 (Symbol) | 总交易数 | 胜场 | 胜率 | 总盈亏 (每单1单位) | 平均盈亏/单 |")
    lines.append("| :--- | :---: | :---: | :---: | :---: | :---: |")
    
    sorted_symbols = sorted(results.keys())
    
    for symbol in sorted_symbols:
        stats = results[symbol]['stats']
        avg_pnl = (stats['total_pnl'] / stats['total_trades']) if stats['total_trades'] > 0 else 0
        
        pnl_str = f"{stats['total_pnl']:.4f}"
        if stats['total_pnl'] > 0: pnl_str = f"+{pnl_str}"
        
        avg_pnl_str = f"{avg_pnl:.4f}"
        if avg_pnl > 0: avg_pnl_str = f"+{avg_pnl_str}"
        
        lines.append(f"| **{symbol}** | {stats['total_trades']} | {stats['wins']} | {stats['win_rate']:.1f}% | {pnl_str} | {avg_pnl_str} |")
        
        grand_total_trades += stats['total_trades']
        grand_total_wins += stats['wins']
        grand_total_pnl += stats['total_pnl']
        
    grand_win_rate = (grand_total_wins / grand_total_trades * 100) if grand_total_trades > 0 else 0
    pnl_str = f"{grand_total_pnl:.4f}"
    if grand_total_pnl > 0: pnl_str = f"+{pnl_str}"
    
    lines.append(f"| **TOTAL** | **{grand_total_trades}** | **{grand_total_wins}** | **{grand_win_rate:.1f}%** | **{pnl_str}** | - |")
    lines.append("")
    
    # Detail Tables
    lines.append("## 📝 详细交易记录")
    
    for symbol in sorted_symbols:
        trades = results[symbol]['trades']
        if not trades:
            continue
            
        lines.append(f"### {symbol}")
        lines.append("| 开仓时间 | 平仓时间 | 操作 | 开仓价 | 平仓价 | 盈亏 (PnL) | 结果 |")
        lines.append("| :--- | :--- | :---: | :---: | :---: | :---: | :---: |")
        
        for t in trades:
            pnl_val = t['pnl']
            pnl_str = f"{pnl_val:.4f}"
            res_icon = "➖"
            
            if pnl_val > 0.0001: # Check slightly above 0 for floating point
                pnl_str = f"**+{pnl_str}**"
                res_icon = "✅"
            elif pnl_val < -0.0001:
                res_icon = "❌"
            
            action_icon = "🟢 UP" if t['action'] == "买涨" else "🔴 DOWN"
            
            # Format times to be shorter (HH:MM:SS)
            t_entry = t['time_entry'].split(' ')[1] if ' ' in t['time_entry'] else t['time_entry']
            t_exit = t['time_exit'].split(' ')[1] if ' ' in t['time_exit'] else t['time_exit']
            
            lines.append(f"| {t_entry} | {t_exit} | {action_icon} | {t['entry_price']:.3f} | {t['exit_price']:.3f} | {pnl_str} | {res_icon} |")
            
        lines.append("")

    with open(output_path, 'w', encoding='utf-8') as f:
        f.write("\n".join(lines))
    
    print(f"Report generated: {output_path}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python simulate_dumb_strategy.py <log_file>")
        sys.exit(1)
        
    log_file = sys.argv[1]
    if not os.path.exists(log_file):
        print(f"File not found: {log_file}")
        sys.exit(1)
        
    # Determine output path
    dir_name = os.path.dirname(log_file)
    output_filename = "dumb_strategy_analysis.md"
    output_path = os.path.join(dir_name, output_filename)
    
    data = parse_log(log_file)
    results = simulate_strategy(data)
    generate_report(results, output_path)
