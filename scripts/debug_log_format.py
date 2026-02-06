import sys
import re

def debug_log(log_path):
    print(f"Reading {log_path}...")
    with open(log_path, 'r', encoding='utf-8') as f:
        count = 0
        for line in f:
            if "建议=" in line:
                print(f"Line {count}: {repr(line)}")
                # Also try our strip_ansi
                ansi_escape = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')
                stripped = ansi_escape.sub('', line)
                print(f"Stripped: {repr(stripped)}")
                if count > 5:
                    break
                count += 1

if __name__ == "__main__":
    debug_log(sys.argv[1])
