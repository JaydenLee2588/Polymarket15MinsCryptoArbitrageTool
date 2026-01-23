
import re
from datetime import datetime
import time

# 1. Check Timestamp
ts = 1769110200
dt_utc = datetime.utcfromtimestamp(ts)
dt_local = datetime.fromtimestamp(ts)

print(f"Timestamp {ts}:")
print(f"  UTC: {dt_utc}")
print(f"  Local: {dt_local}")

# 2. Check PnL Regex
line = "2026/01/22 19:50:47 ✅ [基准1号][eth-updown-15m-1769110200] 决议: UP, 持仓: UP, PnL: $1.16"
# Original Regex
resolution_pattern_old = re.compile(r"\[([\w-]+)\].*决议: (UP|DOWN),.*PnL: \$([\d\.\-]+)")
match_old = resolution_pattern_old.search(line)
if match_old:
    print(f"\nOld Regex Result: Group 1 (Slug?): '{match_old.group(1)}'")
else:
    print("\nOld Regex: No match")

# New Regex Proposal to capture the second bracket which looks like a slug
# Or better, capture the slug pattern specifically inside brackets
resolution_pattern_new = re.compile(r"\[.*?\]\[([\w-]+)\].*决议: (UP|DOWN),.*PnL: \$([\d\.\-]+)")
match_new = resolution_pattern_new.search(line)
if match_new:
    print(f"New Regex Result: Group 1 (Slug): '{match_new.group(1)}', PnL: '{match_new.group(3)}'")
else:
    print("New Regex: No match")
