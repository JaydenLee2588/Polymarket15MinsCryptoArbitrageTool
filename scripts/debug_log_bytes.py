import sys

def debug_bytes(log_path):
    with open(log_path, 'rb') as f:
        # Read enough to find one heartbeat
        chunk = f.read(100000)
        
        # Split by newline bytes
        lines = chunk.split(b'\n')
        
        found = 0
        for line in lines:
            if b'\xe5\xbb\xba\xe8\xae\xae=' in line: # "建议="
                print(f"BYTES: {line}")
                found += 1
                if found > 0:
                    break

if __name__ == "__main__":
    debug_bytes(sys.argv[1])
