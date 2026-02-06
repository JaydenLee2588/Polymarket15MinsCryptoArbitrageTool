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
    
    # Regex for ticker line: "[BTC] ... 建议=【(.*?)】 PM=([0-9.]+)/([0-9.]+)"
    # Note: Color codes might be mixed in, so we strip them first.
    # We'll look for "[XXX]" at start.
    
    with open(log_path, 'r', encoding='utf-8') as f:
        for line in f:
            # Check for timestamp
            line_clean = strip_ansi(line).strip()
            
            # Identify Heartbeat lines (containing time)
            # Example: "2026/02/04 20:29:22 💓 [心跳] ..."
            # Note: actual log lines might be:
            # "2026/02/04 20:29:22 📊 指标概览:"
            time_match = time_pat.match(line_clean)
            if time_match:
                # Update current time if this line looks like a header or heartbeat
                # The log has "💓 [心跳]" then "📊 指标概览:" with same timestamp usually.
                # We'll just update timestamp whenever we see one, it applies to following lines until next timestamp.
                current_time = time_match.group(1)
                
            # Parse ticker line
            # They look like: "[BTC] RSI=... 建议=【观望】 PM=0.99/0.02"
            if line_clean.startswith("["):
                # Use a specific regex to be safe
                # [BTC] RSI=... Pattern=... 建议=【观望】 PM=0.49/0.52
                # DEBUG: Use permissive regex first
                match = re.search(r'\[([A-Z]+)\].*?PM=([0-9.]+)/([0-9.]+)', line_clean)
                if match:
                    if current_time:
                        symbol = match.group(1)
                        # Try to extract suggestion separately
                        sug_match = re.search(r'建议=\s*【(.*?)】', line_clean)
                        suggestion = sug_match.group(1) if sug_match else "UNKNOWN"
                        
                        price_yes = float(match.group(2))
                        price_no = float(match.group(3))
                        
                        data[symbol].append({
                            'time': current_time,
                            'symbol': symbol,
                            'suggestion': suggestion,
                            'yes': price_yes,
                            'no': price_no
                        })
                    else:
                        print(f"DEBUG: Match found but no current_time! Line: {line_clean}")
                else:
                    # Print why it didn't match if it looks like a ticker line
                    print(f"DEBUG: No regex match for line: {repr(line_clean)}")

    return data

def analyze_effectiveness(data):
    results = {}
    
    for symbol, records in data.items():
        analyzed_records = []
        # Sort by time just in case
        records.sort(key=lambda x: x['time'])
        
        for i in range(len(records)):
            curr = records[i]
            prev = records[i-1] if i > 0 else None
            nxt = records[i+1] if i < len(records) - 1 else None
            
            # Determine actionable price based on suggestion
            # Buy UP -> YES Price
            # Buy DOWN -> NO Price
            # Watch -> None (but we track YES/NO changes anyway)
            
            action = curr['suggestion']
            target_price_curr = 0.0
            target_price_next = 0.0
            target_price_prev = 0.0
            
            is_signal = False
            direction = ""
            
            if action == "买涨":
                is_signal = True
                direction = "UP"
                target_price_curr = curr['yes']
                target_price_next = nxt['yes'] if nxt else 0.0
                target_price_prev = prev['yes'] if prev else 0.0
            elif action == "买跌":
                is_signal = True
                direction = "DOWN"
                target_price_curr = curr['no']
                target_price_next = nxt['no'] if nxt else 0.0
                target_price_prev = prev['no'] if prev else 0.0
            else:
                # 观望
                pass

            pnl_next = 0.0
            pnl_prev = 0.0
            
            if is_signal and nxt:
                pnl_next = target_price_next - target_price_curr
            
            if is_signal and prev:
                # Comparison with previous:
                # Did price rise or fall to get here?
                pnl_prev = target_price_curr - target_price_prev

            analyzed_records.append({
                'time': curr['time'],
                'symbol': curr['symbol'],
                'action': action,
                'direction': direction,
                'price': target_price_curr,
                'next_price': target_price_next,
                'prev_price': target_price_prev,
                'pnl_next': pnl_next,
                'pnl_prev': pnl_prev,
                'has_next': nxt is not None,
                'has_prev': prev is not None
            })
            
        results[symbol] = analyzed_records
        
    return results

def generate_markdown(results, output_path):
    lines = []
    lines.append("# 💓 心跳建议有效性分析")
    lines.append("> 比较建议发出时的价格与下一次心跳（30秒后）的价格变化。\n")
    
    total_signals = 0
    correct_signals = 0
    total_pnl = 0.0
    
    for symbol in sorted(results.keys()):
        records = results[symbol]
        signals = [r for r in records if r['action'] in ["买涨", "买跌"]]
        
        if not signals:
            continue
            
        lines.append(f"## {symbol}")
        lines.append(f"| 时间 | 建议 | 当前价格 | 下次价格 | 变化 (PnL) | 上次价格 | 趋势(前->今) | 结果 |")
        lines.append(f"| :--- | :--- | :---: | :---: | :---: | :---: | :---: | :---: |")
        
        symbol_correct = 0
        symbol_pnl = 0.0
        
        for r in signals:
            if not r['has_next']:
                res_icon = "❓(无后继)"
                change_str = "-"
            else:
                change = r['pnl_next']
                total_pnl += change
                symbol_pnl += change
                
                change_str = f"{change:+.2f}"
                if change > 0:
                    res_icon = "✅ 盈利"
                    correct_signals += 1
                    symbol_correct += 1
                elif change < 0:
                    res_icon = "❌ 亏损"
                else:
                    res_icon = "➖ 持平"
            
            prev_str = "-"
            trend_str = "-"
            if r['has_prev']:
                prev_str = f"{r['prev_price']:.2f}"
                trend_val = r['pnl_prev']
                if trend_val > 0:
                    trend_str = f"↗️ (+{trend_val:.2f})" # Price rose to current
                elif trend_val < 0:
                    trend_str = f"↘️ ({trend_val:.2f})" # Price fell to current
                else:
                    trend_str = "➡️"

            # Suggestion color/icon
            sug_icon = "🟢 买涨" if r['action'] == "买涨" else "🔴 买跌"
            
            lines.append(f"| {r['time'][-8:]} | {sug_icon} | {r['price']:.2f} | {r['next_price']:.2f} | **{change_str}** | {prev_str} | {trend_str} | {res_icon} |")
        
        count = len(signals)
        win_rate = (symbol_correct / count * 100) if count > 0 else 0
        lines.append(f"\n**{symbol} 统计**: 信号数 {count}, 准确数 {symbol_correct}, 胜率 {win_rate:.1f}%, 理论总盈亏: ${symbol_pnl:.2f}\n")
        
        total_signals += count
    
    lines.append("## 📈 总体汇总")
    lines.append(f"- **总信号数**: {total_signals}")
    lines.append(f"- **准确信号**: {correct_signals}")
    if total_signals > 0:
        lines.append(f"- **总胜率**: {correct_signals/total_signals*100:.1f}%")
    lines.append(f"- **理论总盈亏**: ${total_pnl:.2f}")

    with open(output_path, 'w', encoding='utf-8') as f:
        f.write("\n".join(lines))
    
    print(f"Analysis saved to: {output_path}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python analyze_heartbeat.py <log_file>")
        sys.exit(1)
        
    log_file = sys.argv[1]
    if not os.path.exists(log_file):
        print(f"File not found: {log_file}")
        sys.exit(1)
        
    # Determine output path
    dir_name = os.path.dirname(log_file)
    output_filename = "heartbeat_analysis.md"
    output_path = os.path.join(dir_name, output_filename)
    
    data = parse_log(log_file)
    result = analyze_effectiveness(data)
    generate_markdown(result, output_path)
