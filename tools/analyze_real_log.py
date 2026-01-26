
import re
import os
from collections import defaultdict

LOG_FILE = r"logs/20260126/output_real_1.log"
OUTPUT_FILE = r"logs/20260126/analysis_summary.txt"

def analyze_log(file_path):
    if not os.path.exists(file_path):
        return None

    stats = {
        'attempts': 0,
        'success': 0,
        'failed_fak': 0,
        'redemptions': 0,
        'redemption_timeouts': 0,
        'strategies': defaultdict(lambda: {'attempts': 0, 'success': 0, 'pnl': 0.0, 'wins': 0, 'losses': 0}),
    }

    with open(file_path, 'r', encoding='utf-8') as f:
        for line in f:
            if "准备下单" in line:
                stats['attempts'] += 1
                m = re.search(r"🚀 \[(.*?)\] 准备下单", line)
                if m:
                    strat = m.group(1)
                    stats['strategies'][strat]['attempts'] += 1

            if "Leg1买入成功" in line:
                stats['success'] += 1
                m = re.search(r"\[💰真实\]\[(.*?)\]", line)
                if m:
                    strat = m.group(1)
                    stats['strategies'][strat]['success'] += 1

            if "API 错误 (400)" in line and "no orders found" in line:
                stats['failed_fak'] += 1
                m = re.search(r"❌ \[(.*?)\] 下单被拒绝", line)
                if m:
                    # strat = m.group(1)
                    pass

            if "决议:" in line and "PnL:" in line:
                m_strat = re.search(r"✅ \[(.*?)\]\[.*?\] 决议:", line)
                m_pnl = re.search(r"PnL: \$([\d\.\-]+)", line)
                
                if m_strat and m_pnl:
                    strat = m_strat.group(1)
                    try:
                        pnl = float(m_pnl.group(1))
                        stats['strategies'][strat]['pnl'] += pnl
                        if pnl > 0:
                            stats['strategies'][strat]['wins'] += 1
                        else:
                            stats['strategies'][strat]['losses'] += 1
                    except:
                        pass

            if "💰 [赎回] 正在执行" in line:
                stats['redemptions'] += 1
            if "等待确认超时" in line:
                stats['redemption_timeouts'] += 1

    return stats

def save_report(stats):
    lines = []
    lines.append("=== Analysis Report ===")
    lines.append(f"Total Order Attempts: {stats['attempts']}")
    lines.append(f"Successful Orders:    {stats['success']}")
    lines.append(f"Failed Orders (FAK):  {stats['failed_fak']}")
    lines.append(f"Redemptions Triggered: {stats['redemptions']}")
    lines.append(f"Redemption Timeouts:   {stats['redemption_timeouts']}")
    
    lines.append("\n--- Strategy Performance ---")
    lines.append(f"{'Strategy':<15} | {'Att':<5} | {'Succ':<5} | {'Fail':<5} | {'Win%':<6} | {'PnL ($)':<10}")
    lines.append("-" * 65)
    
    total_pnl = 0.0
    for strat, data in stats['strategies'].items():
        attempts = data['attempts']
        success = data['success']
        fails = attempts - success # Approx
        
        wins = data['wins']
        losses = data['losses']
        total_res = wins + losses
        win_rate = (wins / total_res * 100) if total_res > 0 else 0
        pnl = data['pnl']
        total_pnl += pnl
        
        lines.append(f"{strat:<15} | {attempts:<5} | {success:<5} | {fails:<5} | {win_rate:5.1f}% | {pnl:10.2f}")

    lines.append("-" * 65)
    lines.append(f"{'TOTAL':<15} | {stats['attempts']:<5} | {stats['success']:<5} | {stats['failed_fak']:<5} | {'N/A':<6} | {total_pnl:10.2f}")

    lines.append("\n--- Insight ---")
    if stats['attempts'] > 0:
        fail_rate = stats['failed_fak'] / stats['attempts'] * 100
        lines.append(f"Order Failure Rate: {fail_rate:.1f}%")
        if fail_rate > 20:
            lines.append("(!) High order failure rate (>20%). Liquidity might be thinner than expected or slippage tolerance too low.")
        elif fail_rate > 5:
             lines.append(f"(!) Moderate order failure rate ({fail_rate:.1f}%). Check slippage settings.")

    with open(OUTPUT_FILE, 'w', encoding='utf-8') as f:
        f.write("\n".join(lines))
    print(f"Report saved to {OUTPUT_FILE}")

if __name__ == "__main__":
    data = analyze_log(LOG_FILE)
    if data:
        save_report(data)
