
import re
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.dates as mdates
from datetime import datetime, timedelta, time
import os

# --- Configuration ---
LOG_FILE = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260219\trade_history_constract.log"
BASE_DATE_STR = "2026-02-19" 
OUTPUT_IMAGE = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260219\pnl_chart.png"

# Session definitions (EST)
SESSIONS = [
    {"name": "Asian", "start": "19:00", "end": "02:00", "color": "#87CEEB", "label": "亚洲盘"},      # SkyBlue
    {"name": "Asian-Euro", "start": "02:00", "end": "03:30", "color": "#98FB98", "label": "亚欧重叠盘"}, # PaleGreen
    {"name": "European", "start": "03:30", "end": "08:00", "color": "#FFE4B5", "label": "欧洲盘"},      # Moccasin
    {"name": "Euro-US", "start": "08:00", "end": "11:30", "color": "#F08080", "label": "欧美重叠盘"},  # LightCoral
    {"name": "US", "start": "11:30", "end": "16:00", "color": "#DDA0DD", "label": "美股盘"},        # Plum
    {"name": "Non-Trading", "start": "16:00", "end": "19:00", "color": "#D3D3D3", "label": "非交易时段"}, # LightGray
]

STRATEGIES_OF_INTEREST = [
    "基准1号",
    "简化版",
    "低阈值11",
    "低阈值13",
    "高阈值17",
    "高阈值19",
    "高阈值21",
    "时段版210-300",
    "时段版300-390",
    "时段版480-570",
    "时段版570-660"
]

STRATEGY_MAPPING = {}

def parse_pnl(line):
    # Identifying PnL events: "决议赎回", "Sniper:profit_target_reached", "Sniper:zscore_reverted"
    # Format usually: ... "【Reason】【Quantity】【Price】【Revenue】【PnL】" "【Resolution】"
    # Example: "【22.5分】【决议赎回】【20】【1.0000】【$20.00】【$4.60】" "【DOWN】"
    
    # We find all items in brackets 【...】
    # The PnL is typically the one before the last one, or the last one in the first group of quotes.
    # Actually, simplistic approach: find all occurrences of $... and pick the last one that looks like PnL.
    # But Revenue also has $.
    # Let's count from the end of the line.
    
    # Clean line first?
    pass

def parse_log(file_path):
    data = []
    
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
             lines = f.readlines()
    except UnicodeDecodeError:
        print("Warning: UTF-8 decode failed, trying gbk")
        with open(file_path, 'r', encoding='gbk') as f:
             lines = f.readlines()
    except Exception as e:
         print(f"Error reading file: {e}")
         return pd.DataFrame()

    current_date = datetime.strptime(BASE_DATE_STR, "%Y-%m-%d")
    last_time = None

    for line in lines:
        line = line.strip()
        if not line.startswith("【"):
            continue
        
        # Extract timestamp
        ts_match = re.match(r"【(\d{2}:\d{2}:\d{2})】", line)
        if not ts_match:
            continue
        
        time_str = ts_match.group(1)
        time_obj = datetime.strptime(time_str, "%H:%M:%S").time()
        
        # Handle day rollover
        if last_time is not None:
            # If we go from 23:xx to 00:xx
            if last_time.hour >= 20 and time_obj.hour < 4:
                current_date += timedelta(days=1)
            # Or just significantly backwards?
            elif time_obj < last_time and (last_time.hour - time_obj.hour > 12):
                 current_date += timedelta(days=1)
        
        dt = datetime.combine(current_date, time_obj)
        last_time = time_obj

        # Check for PnL indicators
        if "决议赎回" in line or "Sniper:" in line or "Sniper:profit_target_reached" in line or "Sniper:zscore_reverted" in line:
            # Extract PnL
            # Typical ending: ...【$4.60】" "【DOWN】" or ...【$-0.35】" "【】"
            # Regex for currency
            dollar_matches = re.findall(r"【\$([-]?\d+\.\d+)】", line)
            if not dollar_matches:
                 continue
            
            # The last dollar match is usually PnL (Revenue is before it)
            # E.g. 【$20.00】【$4.60】 -> PnL is 4.60
            # E.g. 【$2.85】【$-0.35】 -> PnL is -0.35
            pnl = float(dollar_matches[-1])
            
            # Extract Strategy
            # 【Time】【Session】【Strategy】...
            brackets = re.findall(r"【([^】]*)】", line)
            if len(brackets) < 3:
                continue
            raw_strategy = brackets[2]
            strategy = STRATEGY_MAPPING.get(raw_strategy, raw_strategy)
            
            data.append({
                "timestamp": dt,
                "strategy": strategy,
                "pnl": pnl
            })
            
    return pd.DataFrame(data)

def plot_pnl(df):
    if df.empty:
        print("No PnL data found!")
        return

    # Set timestamp as index
    df = df.set_index('timestamp')
    
    # Resample to 1H, sum PnL
    # We aggregate by pivoting logic: grouped by strategy, then resample
    strategies = df['strategy'].unique()
    
    # Create a unified hourly index covering the whole range
    start_time = df.index.min().floor('h')
    end_time = df.index.max().ceil('h')
    full_idx = pd.date_range(start_time, end_time, freq='1H')
    
    result_df = pd.DataFrame(index=full_idx)
    
    for strat in STRATEGIES_OF_INTEREST:
        if strat in strategies:
            # Filter, resample sum, fillna 0
            s_data = df[df['strategy'] == strat]['pnl'].resample('1H').sum()
            # Reindex to full range, fill with 0 (no trades = 0 PnL change)
            s_data = s_data.reindex(full_idx, fill_value=0)
            result_df[strat] = s_data.cumsum()
        else:
            result_df[strat] = 0.0

    # Total Calculation: Sum of hourly PnLs, then cumsum
    # Or sum of cumsums? No, sum of hourly PnLs then cumsum.
    # Re-calculate hourly totals first
    hourly_pnl_all = pd.DataFrame(index=full_idx)
    for strat in strategies:
        # Use simple mapping or raw names if not in interest list?
        # Better to sum ALL strategies found in log for "Total" or just the interested ones?
        # User said "总计" (Total). Usually implies all activities.
        s_data = df[df['strategy'] == strat]['pnl'].resample('1H').sum()
        hourly_pnl_all[strat] = s_data
        
    hourly_pnl_all = hourly_pnl_all.fillna(0)
    result_df['Total'] = hourly_pnl_all.sum(axis=1).cumsum()

    # Plotting
    plt.figure(figsize=(18, 10))
    
    # Configure Chinese Font
    plt.rcParams['font.sans-serif'] = ['SimHei', 'Microsoft YaHei', 'Arial Unicode MS']
    plt.rcParams['axes.unicode_minus'] = False
    
    # Plot background sessions
    curr_date = start_time.date()
    end_date = end_time.date()
    
    # Iterate days
    d = curr_date
    while d <= end_date:
        for session in SESSIONS:
            sh, sm = map(int, session['start'].split(':'))
            eh, em = map(int, session['end'].split(':'))
            
            s_dt = datetime.combine(d, time(sh, sm))
            if eh < sh:
                e_dt = datetime.combine(d + timedelta(days=1), time(eh, em))
            else:
                e_dt = datetime.combine(d, time(eh, em))
                
            # Plot span if overlaps
            plt.axvspan(s_dt, e_dt, color=session['color'], alpha=0.3, label=session['label'] if d == curr_date else "")
            
            # Add text label in the middle of session
            mid_point = s_dt + (e_dt - s_dt)/2
            # Only add label if it's within plot range roughly
            if start_time <= mid_point <= end_time:
                 plt.text(mid_point, plt.gca().get_ylim()[1], session['label'], 
                          ha='center', va='bottom', fontsize=9, color='black', weight='bold', rotation=0)

        d += timedelta(days=1)

    # Plot lines with markers to see data points better
    for i, strat in enumerate(STRATEGIES_OF_INTEREST):
        if strat in result_df.columns:
            plt.plot(result_df.index, result_df[strat], label=strat, linewidth=2, alpha=0.9)
            
    # Plot Total
    plt.plot(result_df.index, result_df['Total'], label='总计', linewidth=3, color='black')

    plt.title(f"Cumulative PnL by Strategy (Hourly) - {BASE_DATE_STR}", fontsize=16)
    plt.xlabel("Time (EST)", fontsize=12)
    plt.ylabel("Cumulative PnL ($)", fontsize=12)
    plt.legend(loc='upper left')
    plt.grid(True, which='both', linestyle='--', linewidth=0.5)
    
    # Trim x-axis to data range
    plt.xlim(df.index.min(), df.index.max())
    
    # Hourly ticks
    plt.gca().xaxis.set_major_formatter(mdates.DateFormatter('%m-%d %H:%M'))
    plt.gca().xaxis.set_major_locator(mdates.HourLocator(interval=1))
    plt.gcf().autofmt_xdate()
    
    plt.savefig(OUTPUT_IMAGE, dpi=150)
    print(f"Chart generated: {OUTPUT_IMAGE}")

if __name__ == "__main__":
    df = parse_log(LOG_FILE)
    if not df.empty:
        print(f"Parsed {len(df)} PnL events.")
        # print(df.groupby('strategy')['pnl'].sum())
        plot_pnl(df)
    else:
        print("No valid PnL lines found.")
