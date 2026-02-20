import re
import os
import pandas as pd
import matplotlib.pyplot as plt
from datetime import datetime, timedelta
import glob

# Set matplotlib style for better aesthetics
plt.style.use('ggplot')
plt.rcParams['font.sans-serif'] = ['SimHei']  # For Chinese characters if needed
plt.rcParams['axes.unicode_minus'] = False

def parse_log_line(line):
    """
    Parses a single log line to extract timestamp and PnL.
    Returns a dict with 'time_str' and 'pnl' if it's a resolution line, else None.
    """
    # Regex to find timestamp at the start: 【HH:MM:SS】
    time_match = re.search(r"^【(\d{2}:\d{2}:\d{2})】", line)
    if not time_match:
        return None
    
    time_str = time_match.group(1)
    
    # Check if this is a PnL line (contains "决议" or "决议赎回" or "止盈" or "时间止损")
    # And specifically looking for the PnL value at the end of the first quoted segment
    # e.g. "【...】【...】【$xx.xx】"
    
    # Simplified check: does it contain PnL in dollar format?
    # The existing logic looks for the last $ value in brackets.
    if "决议" in line or "决议赎回" in line or "止盈" in line or "时间止损" in line:
        pnl_matches = re.findall(r"【\$([-0-9.]+)】", line)
        if pnl_matches:
            try:
                # The last one is usually the Net PnL
                pnl = float(pnl_matches[-1])
                return {
                    "time_str": time_str,
                    "pnl": pnl
                }
            except ValueError:
                pass
                
    return None

def main():
    base_dir = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool"
    log_dates = ["20260207", "20260209", "20260210", "20260211"]
    
    all_trades = []
    
    # Track the last datetime to handle day rollovers
    last_dt = None
    
    for date_str in log_dates:
        folder_path = os.path.join(base_dir, "logs", date_str)
        log_file = os.path.join(folder_path, "trade_history_real.log")
        
        if not os.path.exists(log_file):
            print(f"Warning: File not found: {log_file}")
            continue
            
        print(f"Processing {log_file}...")
        
        # Base date from folder
        current_date_obj = datetime.strptime(date_str, "%Y%m%d")
        
        # We need to handle time within the file. 
        # Since we process files in order, and the user said they are continuous,
        # we just need to be careful about day transitions within a file.
        
        previous_time_obj = None
        day_offset = 0
        
        with open(log_file, 'r', encoding='utf-8') as f:
            for line in f:
                data = parse_log_line(line)
                if not data:
                    continue
                
                time_str = data['time_str']
                pnl = data['pnl']
                
                # Parse time
                time_obj = datetime.strptime(time_str, "%H:%M:%S")
                
                # Check for day rollover within file (e.g. 23:59 -> 00:01)
                # If we see a large backward jump in time (e.g. > 12 hours backwards), it's likely a new day
                if previous_time_obj:
                    diff = (time_obj - previous_time_obj).total_seconds()
                    if diff < -12 * 3600: # Time went backwards significantly
                        day_offset += 1
                
                previous_time_obj = time_obj
                
                full_dt = current_date_obj + timedelta(days=day_offset) + timedelta(hours=time_obj.hour, minutes=time_obj.minute, seconds=time_obj.second)
                
                all_trades.append({
                    "datetime": full_dt,
                    "pnl": pnl
                })
                
                # Update global last_dt just in case we need it for cross-file logic, 
                # although folder dates should handle gross jumps.
                last_dt = full_dt

    if not all_trades:
        print("No trades found.")
        return

    df = pd.DataFrame(all_trades)
    df.sort_values("datetime", inplace=True)
    
    print(f"Total trades parsed: {len(df)}")
    
    # Set index to datetime
    df.set_index("datetime", inplace=True)
    
    # Resample to 15T (15 minutes) and sum PnL for each interval
    df_resampled = df.resample("15T").sum()
    
    # Calculate Cumulative PnL
    df_resampled["cumulative_pnl"] = df_resampled["pnl"].cumsum()
    
    # Handle NaNs (intervals with no trades) - fill with previous cumulative value
    df_resampled["cumulative_pnl"] = df_resampled["cumulative_pnl"].ffill().fillna(0)
    
    # Plotting
    plt.figure(figsize=(12, 6))
    plt.plot(df_resampled.index, df_resampled["cumulative_pnl"], label="Total PnL", linewidth=2)
    
    plt.title("User Total PnL Over Time (15-min Granularity)", fontsize=16)
    plt.xlabel("Time", fontsize=12)
    plt.ylabel("Cumulative PnL ($)", fontsize=12)
    plt.grid(True, linestyle='--', alpha=0.7)
    plt.legend()
    plt.tight_layout()
    
    output_image = os.path.join(base_dir, "scripts", "user_pnl_chart_15m.png")
    plt.savefig(output_image)
    print(f"Chart saved to {output_image}")
    
    # Also save the data to csv for manual inspection if needed
    output_csv = os.path.join(base_dir, "scripts", "user_pnl_15m.csv")
    df_resampled.to_csv(output_csv)
    print(f"Data saved to {output_csv}")

if __name__ == "__main__":
    main()
