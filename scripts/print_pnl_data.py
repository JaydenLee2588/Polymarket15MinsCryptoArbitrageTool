
import pandas as pd
import re
from datetime import datetime, timedelta

LOG_FILE = r"d:\Github\OtherProjects\BackupPolymarket15MinsCryptoArbitrageTool\logs\20260214\trade_history_constract.log"
BASE_DATE_STR = "2026-02-11"

def parse_log_and_print():
    data = []
    current_date = datetime.strptime(BASE_DATE_STR, "%Y-%m-%d")
    last_time = None
    
    try:
        with open(LOG_FILE, 'r', encoding='utf-8') as f:
            lines = f.readlines()
    except:
        with open(LOG_FILE, 'r', encoding='gbk') as f:
            lines = f.readlines()

    for line in lines:
        line = line.strip()
        if not line.startswith("【"): continue
        
        ts_match = re.match(r"【(\d{2}:\d{2}:\d{2})】", line)
        if not ts_match: continue
        
        time_obj = datetime.strptime(ts_match.group(1), "%H:%M:%S").time()
        
        # Simple rollover check
        if last_time and last_time.hour > 20 and time_obj.hour < 4:
            current_date += timedelta(days=1)
        elif last_time and (last_time.hour - time_obj.hour > 12):
             current_date += timedelta(days=1)
             
        last_time = time_obj
        dt = datetime.combine(current_date, time_obj)
        
        # Extract PnL
        if "决议赎回" in line or "Sniper:" in line:
            dollar_matches = re.findall(r"【\$([-]?\d+\.\d+)】", line)
            if dollar_matches:
                pnl = float(dollar_matches[-1])
                data.append({"timestamp": dt, "pnl": pnl})

    if not data:
        print("No PnL data found.")
        return

    df = pd.DataFrame(data)
    df = df.set_index('timestamp')
    
    # Resample to 1H
    hourly = df['pnl'].resample('1H').sum().fillna(0)
    cumulative = hourly.cumsum()
    
    with open("pnl_data.txt", "w") as f:
        f.write("--- Hourly Cumulative PnL ---\n")
        for t, val in cumulative.items():
            f.write(f"{t}: {val:.2f}\n")
    print("Data written to pnl_data.txt")

if __name__ == "__main__":
    parse_log_and_print()
