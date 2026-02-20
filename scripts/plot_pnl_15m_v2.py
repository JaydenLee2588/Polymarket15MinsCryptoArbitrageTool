import re
import os
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.dates as mdates
from datetime import datetime, timedelta

# Set matplotlib style
plt.style.use('ggplot')
plt.rcParams['font.sans-serif'] = ['SimHei', 'Microsoft YaHei', 'Arial']
plt.rcParams['axes.unicode_minus'] = False

def parse_log_line(line):
    """
    Parses a single log line.
    Returns dict with time_str and pnl if it's a resolution line.
    """
    # Time format: 【HH:MM:SS】
    time_match = re.search(r"^【(\d{2}:\d{2}:\d{2})】", line)
    if not time_match:
        return None
    
    time_str = time_match.group(1)
    
    # Check for PnL
    # Looking for lines with PnL values like 【$1.05】 or 【$-4.05】
    # These usually appear in resolution lines: "【决议赎回】...【$Cost】【$Revenue】【$PnL】"
    # Or "【止盈】...【$PnL】"
    
    try:
        if "决议" in line or "决议赎回" in line or "止盈" in line or "时间止损" in line:
            # Find all currency-like values
            pnl_matches = re.findall(r"【\$([-0-9.]+)】", line)
            if pnl_matches:
                # The last one is the Net PnL
                pnl = float(pnl_matches[-1])
                return {
                    "time_str": time_str,
                    "pnl": pnl
                }
    except Exception as e:
        pass
                
    return None

def main():
    base_dir = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs"
    # The user explicitly listed these dates
    target_dates = ["20260207", "20260209", "20260210", "20260211"]
    
    # Possible filenames (handling typo)
    filenames_to_check = ["trade_history_real.log", "trade_histrory_real.log"]
    
    all_trades = []
    
    print(f"Scanning for logs in {base_dir}...")
    
    # We loop through the dates in order
    for date_str in target_dates:
        folder_path = os.path.join(base_dir, date_str)
        if not os.path.exists(folder_path):
            print(f"  [Warn] Folder not found: {folder_path}")
            continue
            
        # Find the log file
        found_file = None
        for fname in filenames_to_check:
            fpath = os.path.join(folder_path, fname)
            if os.path.exists(fpath):
                found_file = fpath
                break
        
        if not found_file:
            print(f"  [Warn] No log file found in {folder_path}")
            continue
            
        print(f"  Parsing {found_file}...")
        
        # Determine the base date for this file
        current_base_date = datetime.strptime(date_str, "%Y%m%d")
        
        # Track time within file to handle day rollovers
        last_time_obj = None
        day_offset = 0
        
        trades_in_file = 0
        
        with open(found_file, 'r', encoding='utf-8') as f:
            for line in f:
                data = parse_log_line(line)
                if not data:
                    continue
                
                time_str = data['time_str']
                pnl = data['pnl']
                
                # Parse HTML/Log time
                t_obj = datetime.strptime(time_str, "%H:%M:%S")
                
                # Check for rollover
                if last_time_obj:
                    # If time jumps back significantly (e.g. 23:59 -> 00:01), add a day
                    delta = (t_obj - last_time_obj).total_seconds()
                    if delta < -12 * 3600: 
                        day_offset += 1
                
                last_time_obj = t_obj
                
                # Construct full datetime
                full_dt = current_base_date + timedelta(days=day_offset) + timedelta(hours=t_obj.hour, minutes=t_obj.minute, seconds=t_obj.second)
                
                all_trades.append({
                    "datetime": full_dt,
                    "pnl": pnl
                })
                trades_in_file += 1
        
        print(f"    -> Found {trades_in_file} PnL events.")

    if not all_trades:
        print("No trades found across all files.")
        return

    # Convert to DataFrame
    df = pd.DataFrame(all_trades)
    df.sort_values("datetime", inplace=True)
    
    # Set index
    df.set_index("datetime", inplace=True)
    
    # ---------------------------------------------------------
    # 15-Minute Aggregation
    # ---------------------------------------------------------
    # We strictly resample to 15T. 
    # This aligns timestamps to 00, 15, 30, 45.
    df_15m = df.resample("15T").sum()
    
    # Calculate Cumulative PnL
    df_15m["cumulative_pnl"] = df_15m["pnl"].cumsum()
    
    # Fill gaps?
    # If we want a continuous line over time, we ffill the cumulative PnL.
    # (Because if no trade happened, PnL didn't change, so cumulative stays same)
    df_15m["cumulative_pnl"] = df_15m["cumulative_pnl"].ffill().fillna(0)

    print("-" * 30)
    print(f"Total Time Span: {df_15m.index.min()} to {df_15m.index.max()}")
    print(f"Total 15m Intervals: {len(df_15m)}")
    print(f"Final Cumulative PnL: ${df_15m['cumulative_pnl'].iloc[-1]:.2f}")
    print("-" * 30)

    # ---------------------------------------------------------
    # Plotting
    # ---------------------------------------------------------
    fig, ax = plt.subplots(figsize=(14, 7))
    
    # Plot the line
    ax.plot(df_15m.index, df_15m["cumulative_pnl"], 
            color="#2E86C1", 
            linewidth=2, 
            label="累计收益 (Cumulative PnL)")
    
    # Add markers for each 15m point to emphasize granularity, 
    # but only if not too dense (e.g. < 500 points). 
    # Given 4 days ~ 4*24*4 = 384 points, it's fine.
    if len(df_15m) < 1000:
        ax.scatter(df_15m.index, df_15m["cumulative_pnl"], s=10, color="#1B4F72", alpha=0.6)

    # Formatting Time Axis
    # Show Day + Hour
    ax.xaxis.set_major_formatter(mdates.DateFormatter('%m-%d %H:%M'))
    ax.xaxis.set_major_locator(mdates.HourLocator(interval=6)) # Tick every 6 hours
    plt.xticks(rotation=45)
    
    # Titles and Labels
    ax.set_title("用户总PnL资金曲线 (15分钟粒度)\nUser Total PnL (15-min Granularity)", fontsize=16)
    ax.set_ylabel("累计收益 ($)", fontsize=12)
    ax.set_xlabel("时间 (Time)", fontsize=12)
    
    # Grid
    ax.grid(True, which='both', linestyle='--', alpha=0.5)
    ax.minorticks_on()
    ax.grid(True, which='minor', linestyle=':', alpha=0.2)
    
    plt.tight_layout()
    
    output_path = os.path.join(base_dir, "..", "scripts", "user_pnl_chart_15m_v2.png")
    plt.savefig(output_path, dpi=120)
    print(f"Chart saved to: {output_path}")
    
    # Save CSV
    csv_path = os.path.join(base_dir, "..", "scripts", "user_pnl_15m_v2.csv")
    df_15m.to_csv(csv_path)
    print(f"Data saved to: {csv_path}")

if __name__ == "__main__":
    main()
