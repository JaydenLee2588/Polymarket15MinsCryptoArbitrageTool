import re
import sys

def check_failures(log_path):
    print(f"Checking {log_path} for unparsed lines...")
    
    # Standard pattern from analysis script
    # 【14:51:10】【美股盘】【基本2号】【sol-updown-15m-1770579900】
    prefix_pattern = r"^【(.*?)】【(.*?)】【(.*?)】【(.*?)】"
    
    with open(log_path, 'r', encoding='utf-8') as f:
        for line in f:
            if not line.strip(): continue
            
            # If it matches the standard prefix, we assume it's a standard trade log
            if not re.match(prefix_pattern, line):
                print(f"Unparsed Line: {line.strip()}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python debug_log_failures.py <log_file>")
        sys.exit(1)
    
    check_failures(sys.argv[1])
