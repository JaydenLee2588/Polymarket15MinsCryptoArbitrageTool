
import re

filepath = r"logs/20260206/output-2.log"

print(f"Reading {filepath}...")
try:
    with open(filepath, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    print(f"Total lines: {len(lines)}")
except Exception as e:
    print(f"Error reading utf-8: {e}")
    try:
        with open(filepath, 'r', encoding='gbk') as f:
            lines = f.readlines()
        print(f"Total lines (gbk): {len(lines)}")
    except Exception as e2:
        print(f"Error reading gbk: {e2}")
        exit(1)

# Check for Buy
buy_patterns = ["触发买入", "买入成功", "Buy"]
found_buy = 0
for i, line in enumerate(lines):
    if any(p in line for p in buy_patterns):
        print(f"Line {i+1}: {line.strip()}")
        found_buy += 1
        if found_buy > 5: break

# Check for Resolution/Sell/PnL
sell_patterns = ["决议", "Sell", "卖出", "赎回", "赢利", "亏损", "Profit", "Loss"]
found_sell = 0
for i, line in enumerate(lines):
    if any(p in line for p in sell_patterns):
        print(f"Line {i+1}: {line.strip()}")
        found_sell += 1
        if found_sell > 20: break
