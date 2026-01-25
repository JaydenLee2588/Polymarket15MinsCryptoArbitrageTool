
import re

TXT_FILE = r"logs/20260125/price_granularity_report_full.txt"
TARGET_FILE = r"logs/20260125/20250125配置和结果数据.md"

def main():
    try:
        with open(TXT_FILE, 'r', encoding='utf-8') as f:
            lines = f.readlines()
            
        md_lines = []
        md_lines.append("\n\n### 💰 价格粒度绩效表 (每 0.01 一档)\n")
        md_lines.append("| 下单价格 | 胜率 (Win Rate) | 总盈亏 (Total PnL) | 平均单笔盈亏 | 交易次数 |")
        md_lines.append("|:---:|:---:|:---:|:---:|:---:|")
        
        # Skip headers and separator lines in txt (first 5 lines based on view_file output)
        # Line 6 starts data: "0.53     | 100.0% | ..."
        
        start_processing = False
        for line in lines:
            if "--------" in line:
                start_processing = True
                continue
            if not start_processing:
                continue
            if "TOTAL" in line:
                break
                
            parts = line.split('|')
            if len(parts) < 5:
                continue
                
            # Clean whitespaces
            price = parts[0].strip()
            wr = parts[1].strip()
            pnl = parts[2].strip()
            avg = parts[3].strip()
            count = parts[4].strip()
            
            # Highlight negative PnL or low Win Rate
            try:
                pnl_val = float(pnl.replace(',', ''))
                if pnl_val < -100:
                    pnl = f"🔻 **{pnl}**"
            except:
                pass
                
            md_lines.append(f"| {price} | {wr} | {pnl} | {avg} | {count} |")
            
        md_lines.append("\n**重点观察**:")
        md_lines.append("*   **0.63** 和 **0.65** 是主要的亏损来源 (合计亏损超 **$1300**)。")
        md_lines.append("*   **0.72** 也是一个显著的亏损点 (-$412)。")
        md_lines.append("*   **0.80** 以上基本稳定盈利。")
        
        with open(TARGET_FILE, 'a', encoding='utf-8') as f:
            f.write("\n".join(md_lines))
            
        print("Successfully appended markdown table.")
        
    except Exception as e:
        print(f"Error: {e}")

if __name__ == "__main__":
    main()
