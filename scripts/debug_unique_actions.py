import re
import sys

def check_actions(log_path):
    print(f"Checking {log_path} for unique actions...")
    
    unique_actions = set()
    
    prefix_pattern = r"^【(.*?)】【(.*?)】【(.*?)】【(.*?)】"
    
    with open(log_path, 'r', encoding='utf-8') as f:
        for line in f:
            if not line.strip(): continue
            
            # extract the quoted part
            # ... "【6.2分】【买入】【UP】【30】【0.8700】【$26.40】" ...
            # Action is usually the 2nd bracket in the quotes?
            # Let's extract all brackets after the prefix
            
            match = re.match(prefix_pattern, line)
            if match:
                rest = line[match.end():]
                # find all brackets in rest
                brackets = re.findall(r"【(.*?)】", rest)
                # expected: Score, Action, Side, ...
                if len(brackets) >= 2:
                    action = brackets[1]
                    unique_actions.add(action)
    
    print("Unique Actions Found:")
    for a in sorted(unique_actions):
        print(f"- {a}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python debug_unique_actions.py <log_file>")
        sys.exit(1)
    
    check_actions(sys.argv[1])
