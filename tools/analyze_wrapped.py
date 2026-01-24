import re
import sys
from collections import defaultdict

def analyze_wrapped(file_path):
    print(f"Analyzing {file_path}...")
    
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    # Remove newlines to handle wrapping
    # But be careful not to merge independent log lines inappropriately.
    # The log lines start with date "2026/..."
    # We can split by "2026/01/24" to separate entries?
    # Or just use regex on the massive string?
    
    # Simple strategy: remove all newlines, then regex.
    # The only risk is if [Group] is far from PnL.
    # But usually they are within the same log entry.
    
    clean_content = content.replace('\n', '').replace('\r', '')
    
    # Regex: [Group] ... PnL: $1.23
    # We need to be careful about not matching across different log entries.
    # A log entry ends when a new date starts? Or just greedy match?
    # Let's try to match specifically:
    # "✅ [Group]... PnL: $value"
    # NOTE: The "✅" might be missing or mangled.
    # Let's count "PnL: $" occurrences first.
    
    pnl_pattern = re.compile(r'PnL: \$([-0-9\.]+)')
    all_pnls = pnl_pattern.findall(clean_content)
    print(f"Found {len(all_pnls)} PnL entries.")
    
    # Now try to associate with groups.
    # Pattern: \[(.*?)\](?:\[.*?\])?.*?决议:.*?, PnL: \$([-0-9\.]+)
    # This might match too much if we are not careful.
    # But given "决议" is unique to resolution lines...
    
    full_pattern = re.compile(r'\[(.*?)\](?:\[.*?\])?.*?决议:.*?, PnL: \$([-0-9\.]+)')
    
    # We'll split the content by "2026/01/24" (approx timestamp start) to isolate lines roughly.
    # Since we removed newlines, we can re-insert them before every Date?
    # Better: split by "✅" if it exists, or just use findall on the big string?
    # findall is non-overlapping.
    
    matches = full_pattern.findall(clean_content)
    
    stats = defaultdict(list)
    for group, pnl_str in matches:
        # cleanup group
        if "模拟" in group:
            continue # Should be handled by grouping
        # Wait, the regex `\[(.*?)\]` captures the FIRST bracket content.
        # If line is `✅ [模拟][对照14]...` -> group=`模拟`.
        # If line is `✅ [对照14]...` -> group=`对照14`.
        # We need to handle this.
        
        # Enhanced parsing on the matches
        # Actually complex regex is better: `(?:\[模拟\])?\[(.*?)\]`
        pass

    # Let's use a simpler approach: finding "PnL:" and looking backwards for the Group.
    # Split content into chunks ending with "PnL: $XX"
    
    chunks = clean_content.split("PnL: $")
    # chunks[0] is garbage before first PnL.
    # chunks[1] starts with value, then text until next PnL...
    
    for i in range(1, len(chunks)):
        # The value is at the start of this chunk
        val_str = chunks[i].split()[0]
        try:
            val = float(val_str)
        except:
            continue
            
        # The Group ID is in the END of the PREVIOUS chunk chunks[i-1]
        prev_chunk = chunks[i-1]
        
        # Look for [Group] near the end of prev_chunk
        # It should appear after "✅" or just enclosed in brackets.
        # Search for all `[...]` in prev_chunk
        brackets = re.findall(r'\[(.*?)\]', prev_chunk)
        if not brackets:
            continue
            
        # Candidates for group are usually the last ones.
        # Format: [Group][Market] 决议...
        # So it's likely the second to last or last...
        # If we filter out "Market" like string (with dash) and "模拟"
        
        candidates = []
        for b in reversed(brackets):
            if "updown" in b: continue
            if "模拟" in b: continue
            if "心跳" in b: continue
            if ":" in b: continue # timestamp?
            candidates.append(b)
            if len(candidates) >= 1: break # We just need the group
            
        if candidates:
            group = candidates[0]
            stats[group].append(val)
            
    # Print stats
    print(f"{'Group':<30} | {'Trades':<6} | {'Total PnL':<10} | {'Win Rate':<8}")
    print("-" * 70)
    for group, vals in sorted(stats.items(), key=lambda x: sum(x[1]), reverse=True):
        total = sum(vals)
        wins = len([v for v in vals if v > 0])
        count = len(vals)
        wr = (wins/count*100) if count else 0
        print(f"{group:<30} | {count:<6} | ${total:<10.2f} | {wr:<8.1f}%")

if __name__ == "__main__":
    analyze_wrapped(sys.argv[1])
