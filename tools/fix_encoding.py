
import os

FILES = [
    r"d:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\.env",
    r"d:\Github\OtherProjects\Polymarket15MinsCryptoArbitrageTool\.env.example"
]

def fix_file(path):
    if not os.path.exists(path):
        print(f"File not found: {path}")
        return

    print(f"Fixing {path}...")
    try:
        with open(path, 'rb') as f:
            content = f.read()
        
        # Scenario: Mixed UTF-8 and UTF-16LE
        # 1. Remove UTF-16LE BOM if present in the middle (Powershell append might add it)
        content = content.replace(b'\xff\xfe', b'')
        
        # 2. Remove null bytes (simplest way to converting ASCII-in-UTF16 to ASCII)
        # Be careful not to break valid UTF-8 sequences if they happened to have 0x00? 
        # UTF-8 doesn't use 0x00 except for null char itself.
        clean_content = content.replace(b'\x00', b'')
        
        # 3. Decode as utf-8 to ensure validity
        text = clean_content.decode('utf-8', errors='ignore')
        
        # 4. Write back clean
        with open(path, 'w', encoding='utf-8') as f:
            f.write(text)
            
        print("Scrubbed successfully.")
        
    except Exception as e:
        print(f"Error fixing {path}: {e}")

if __name__ == "__main__":
    for f in FILES:
        fix_file(f)
