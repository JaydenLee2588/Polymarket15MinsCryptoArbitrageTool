import re
import matplotlib.pyplot as plt
import datetime
import sys
import os

def parse_log(file_path):
    data = {} # market -> list of dicts
    
    # Regex for lines like:
    # 2026/01/24 13:31:07 🔍 [基准1号][sol-updown-15m-1769261400] 趋势满足(UP: 0.0157%) 但优势不足: Z=0.16 (Sigmoid价=0.561), 市场价=0.580, 优势=-0.019 (要求=0.99), 剩余833秒
    pattern = re.compile(r"(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) .*?\[(.*?)\]\[(.*?)\] 趋势满足\(.*?: (.*?)%\) 但优势不足: Z=(.*?) \(Sigmoid价=(.*?)\), 市场价=(.*?), 优势=(.*?) \(要求=.*?\)")

    with open(file_path, 'r', encoding='utf-8') as f:
        for line in f:
            match = pattern.search(line)
            if match:
                dt_str, group, market, trend_pct_str, z_str, fair_str, mkt_str, edge_str = match.groups()
                
                try:
                    dt = datetime.datetime.strptime(dt_str, "%Y/%m/%d %H:%M:%S")
                    trend = float(trend_pct_str)
                    z = float(z_str)
                    fair = float(fair_str)
                    mk_price = float(mkt_str)
                    edge = float(edge_str)
                    
                    if market not in data:
                        data[market] = {
                            "time": [],
                            "trend": [],
                            "z": [],
                            "fair": [],
                            "market_price": [],
                            "edge": []
                        }
                    
                    data[market]["time"].append(dt)
                    data[market]["trend"].append(trend)
                    data[market]["z"].append(z)
                    data[market]["fair"].append(fair)
                    data[market]["market_price"].append(mk_price)
                    data[market]["edge"].append(edge)
                    
                except ValueError as e:
                    print(f"Error parsing line: {line}\n{e}")
                    continue

    return data

def plot_data(data):
    if not data:
        print("No data found to plot.")
        return

    # Create 4 subplots sharing x axis (Combined Price Chart)
    fig, axes = plt.subplots(4, 1, figsize=(12, 12), sharex=True)
    
    # 1. Trend
    ax = axes[0]
    market_list = list(data.keys())
    colors = ['b', 'g', 'r', 'c', 'm', 'y', 'k'] # Basic cycle
    
    for j, market in enumerate(market_list):
        color = colors[j % len(colors)]
        m_data = data[market]
        ax.plot(m_data["time"], m_data["trend"], label=market, color=color, alpha=0.7)
    ax.set_ylabel("Trend %")
    ax.legend(loc='upper left', bbox_to_anchor=(1, 1), fontsize='small')
    ax.grid(True, linestyle='--', alpha=0.5)

    # 2. Z-Score
    ax = axes[1]
    for j, market in enumerate(market_list):
        color = colors[j % len(colors)]
        m_data = data[market]
        ax.plot(m_data["time"], m_data["z"], label=market, color=color, alpha=0.7)
    ax.set_ylabel("Z-Score")
    ax.grid(True, linestyle='--', alpha=0.5)

    # 3. Price Comparison (Sigmoid vs Market)
    ax = axes[2]
    for j, market in enumerate(market_list):
        color = colors[j % len(colors)]
        m_data = data[market]
        # Sigmoid = Solid
        ax.plot(m_data["time"], m_data["fair"], label=f"{market} Sigmoid", color=color, linestyle='-', alpha=0.8, linewidth=1.5)
        # Market = Dashed
        ax.plot(m_data["time"], m_data["market_price"], label=f"{market} Market", color=color, linestyle='--', alpha=0.6, linewidth=1.5)
    
    ax.set_ylabel("Price (Sigmoid vs Market)")
    ax.grid(True, linestyle='--', alpha=0.5)
    # Optional: Add legend for line styles if needed, or rely on color matching

    # 4. Edge
    ax = axes[3]
    for j, market in enumerate(market_list):
        color = colors[j % len(colors)]
        m_data = data[market]
        ax.plot(m_data["time"], m_data["edge"], label=market, color=color, alpha=0.7)
    
    # Add a horizontal line for the 0.99 requirement
    ax.axhline(y=0.99, color='k', linestyle=':', label='Min Req (0.99)')
    ax.set_ylabel("Edge (Fair - Market)")
    ax.grid(True, linestyle='--', alpha=0.5)

    plt.xlabel("Time")
    plt.tight_layout()
    
    output_file = "log_visualization.png"
    plt.savefig(output_file)
    print(f"Chart saved to {output_file}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python tools/plot_log_gen.py <log_file>")
        sys.exit(1)
        
    log_file = sys.argv[1]
    if not os.path.exists(log_file):
        print(f"File not found: {log_file}")
        sys.exit(1)
        
    print(f"Parsing {log_file}...")
    parsed_data = parse_log(log_file)
    print(f"Found data for {len(parsed_data)} markets.")
    plot_data(parsed_data)
