import re
import sys
import glob
from collections import defaultdict
from datetime import datetime

def parse_time_into_candle(time_str, market_id):
    # Try to parse time string HH:MM:SS
    # 15m candles align to :00, :15, :30, :45
    try:
        t = datetime.strptime(time_str, "%H:%M:%S")
        total_seconds = t.minute * 60 + t.second
        # Find offset from last 15m boundary
        # Boundaries: 0, 900, 1800, 2700 inside an hour? No, just mod 15 min.
        # Boundary seconds in hour: 0, 900, 1800, 2700 (0, 15, 30, 45 minutes)
        
        # Seconds into current hour
        sec_in_hour = t.minute * 60 + t.second
        
        # Find nearest previous boundary
        # 0, 15, 30, 45
        block_15m = 15 * 60
        start_of_candle = (sec_in_hour // block_15m) * block_15m
        
        offset = sec_in_hour - start_of_candle
        return offset
    except:
        return -1

def get_bin_key(value, bin_size=0.1, is_int=False):
    if is_int:
        start = (int(value) // int(bin_size)) * int(bin_size)
        end = start + bin_size
        return f"{start}-{end}"
    else:
        start = (float(value) // bin_size) * bin_size
        end = start + bin_size
        return f"{start:.2f}-{end:.2f}"

def analyze_file(filename, specific_group=None):
    print(f"Analyzing {filename}...")
    
    # Storage: pending_trades[(group, market)] = list of entries
    # entry = {price, z, time_val, quantity, token}
    pending_trades = defaultdict(list)
    
    # Aggregators
    pnl_by_price = defaultdict(float) # Bin 0.1
    pnl_by_time = defaultdict(float)  # Bin 60s
    pnl_by_z = defaultdict(float)     # Bin 0.5
    pnl_by_token = defaultdict(float) # Token string
    
    counts_by_price = defaultdict(int)
    counts_by_time = defaultdict(int)
    counts_by_z = defaultdict(int)
    counts_by_token = defaultdict(int)

    # Regex patterns
    # Entry: ✅ [模拟][对照7_狂欢] Leg1买入成功 | 时间=11:31:54 | 市场=eth-updown... | ... | Z=1.09 | 价格=0.6700 | 数量=5.00 ...
    # Note: The group part might vary like [基准1号] without [模拟] tag or with it.
    # We'll be flexible with the prefix.
    
    # Regex 1: Entry
    # Capture: Group, Market, Time, Z, Price, Qty
    re_entry = re.compile(r"✅ (?:\[.*?\])?\[(.*?)\] Leg\d买入成功 \| 时间=(.*?) \| 市场=(.*?) \|.*? Z=(.*?) \| 价格=(.*?) \| 数量=(.*?) \|")
    
    # Regex 2: Resolution
    # ✅ [对照6_低波敏感][eth-updown-15m-...] 决议: DOWN, 持仓: DOWN, PnL: $0.27
    re_res = re.compile(r"✅ (?:\[.*?\])?\[(.*?)\]\[(.*?)\] 决议:.*?, PnL: \$([-\d\.]+)")

    with open(filename, 'r', encoding='utf-8') as f:
        for line in f:
            # Check Entry
            m_entry = re_entry.search(line)
            if m_entry:
                group = m_entry.group(1)
                time_str = m_entry.group(2)
                market = m_entry.group(3)
                z_val = float(m_entry.group(4))
                price = float(m_entry.group(5))
                qty = float(m_entry.group(6))
                
                # Filter specific group if needed (e.g. for Real log where we might only care about one)
                if specific_group and group != specific_group:
                    continue
                
                # Token
                token = market.split('-')[0].upper()
                
                # Time into candle
                t_sec = parse_time_into_candle(time_str, market)
                
                pending_trades[(group, market)].append({
                    'price': price,
                    'z': z_val,
                    'time': t_sec,
                    'qty': qty,
                    'token': token
                })
                continue

            # Check Resolution
            m_res = re_res.search(line)
            if m_res:
                group = m_res.group(1)
                market = m_res.group(2)
                pnl_str = m_res.group(3)
                
                try:
                    pnl = float(pnl_str)
                except:
                    continue
                
                entries = pending_trades.get((group, market))
                if not entries:
                    continue
                
                # Calculate total qty to distribute PnL
                total_qty = sum(e['qty'] for e in entries)
                if total_qty == 0:
                    continue
                
                for e in entries:
                    # Attribution share
                    share = e['qty'] / total_qty
                    part_pnl = pnl * share
                    
                    # Accumulate
                    # Price Bin (0.05 granularity for finer detail)
                    p_bin = get_bin_key(e['price'], 0.05)
                    pnl_by_price[p_bin] += part_pnl
                    counts_by_price[p_bin] += 1
                    
                    # Time Bin (60s)
                    if e['time'] >= 0:
                        t_bin = get_bin_key(e['time'], 60, True)
                        pnl_by_time[t_bin] += part_pnl
                        counts_by_time[t_bin] += 1
                    
                    # Z Bin (0.2)
                    z_bin = get_bin_key(e['z'], 0.2)
                    pnl_by_z[z_bin] += part_pnl
                    counts_by_z[z_bin] += 1
                    
                    # Token
                    pnl_by_token[e['token']] += part_pnl
                    counts_by_token[e['token']] += 1
                
                # Clear treated trades
                # Note: Assuming 'Resolution' clears all positions for this market/group
                del pending_trades[(group, market)]
                continue

    return {
        'price': (pnl_by_price, counts_by_price),
        'time': (pnl_by_time, counts_by_time),
        'z': (pnl_by_z, counts_by_z),
        'token': (pnl_by_token, counts_by_token)
    }

def print_table(title, data_tuple, sort_key_lambda, f):
    pnl_dict, count_dict = data_tuple
    f.write(f"\n--- {title} ---\n")
    f.write(f"{'Range':<20} | {'Trades':<6} | {'Total PnL':<10} | {'Avg PnL':<8}\n")
    f.write("-" * 55 + "\n")
    
    # Sort keys
    try:
        keys = sorted(pnl_dict.keys(), key=sort_key_lambda)
    except Exception as e:
        print(f"Sort error: {e}")
        keys = sorted(pnl_dict.keys())

    total_trades = 0
    total_pnl = 0

    if not keys:
        f.write("(No data found)\n")

    for k in keys:
        count = count_dict[k]
        pnl = pnl_dict[k]
        avg = pnl / count if count > 0 else 0
        f.write(f"{str(k):<20} | {count:<6} | ${pnl:<9.2f} | ${avg:<7.2f}\n")

        total_trades += count
        total_pnl += pnl
    
    f.write("-" * 55 + "\n")
    f.write(f"{'TOTAL':<20} | {total_trades:<6} | ${total_pnl:<9.2f} | {'-':<7}\n")

def sort_range_str(k):
    # k is like "0.60-0.70" or "60-120"
    try:
        # Check if it looks like a range number
        if '-' in str(k) and str(k).replace('-','').replace('.','').isdigit():
             return float(k.split('-')[0])
        # Try float directly
        return float(k)
    except:
        return str(k)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python analyze_multidim.py <logfile> [optional_group_filter]")
        sys.exit(1)
        
    filename = sys.argv[1]
    group_filter = sys.argv[2] if len(sys.argv) > 2 else None
    
    results = analyze_file(filename, group_filter)
    
    output_filename = filename + "_analysis.txt"
    with open(output_filename, "w", encoding="utf-8") as f:
        print_table("PnL by Price Interval (0.05)", results['price'], sort_range_str, f)
        print_table("PnL by Time Into Candle (60s)", results['time'], sort_range_str, f)
        print_table("PnL by Z-Score (0.2)", results['z'], sort_range_str, f)
        print_table("PnL by Token", results['token'], lambda k: k, f)
    
    print(f"Analysis written to {output_filename}")
