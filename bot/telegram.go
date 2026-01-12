package bot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramBot Telegram 机器人 (简化版)
type TelegramBot struct {
	bot      *tgbotapi.BotAPI
	stopChan chan struct{}
	mu       sync.RWMutex
}

// New 创建新的 Telegram Bot
func New(token string, dbPath string) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("创建 Telegram Bot 失败: %w", err)
	}

	log.Printf("✅ Telegram Bot 已授权: @%s", bot.Self.UserName)

	// 初始化存储
	if err := InitBotStorage(dbPath); err != nil {
		return nil, fmt.Errorf("初始化 Bot 存储失败: %w", err)
	}

	return &TelegramBot{
		bot:      bot,
		stopChan: make(chan struct{}),
	}, nil
}

// Start 启动机器人
func (t *TelegramBot) Start() {
	// 注册命令菜单
	t.registerCommands()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := t.bot.GetUpdatesChan(u)

	log.Println("🤖 Telegram Bot 开始监听消息...")

	for {
		select {
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			if update.Message.IsCommand() {
				t.handleCommand(update.Message)
			}
		case <-t.stopChan:
			log.Println("🛑 Telegram Bot 已停止")
			return
		}
	}
}

// registerCommands 注册命令菜单
func (t *TelegramBot) registerCommands() {
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "欢迎信息 / Welcome"},
		{Command: "sim_stats", Description: "模拟交易统计 / Simulation Stats"},
	}

	cfg := tgbotapi.NewSetMyCommands(commands...)
	_, err := t.bot.Request(cfg)
	if err != nil {
		log.Printf("⚠️ 注册命令菜单失败: %v", err)
	} else {
		log.Println("✅ 命令菜单已注册")
	}
}

// Stop 停止机器人
func (t *TelegramBot) Stop() {
	close(t.stopChan)
	CloseBotStorage()
}

// handleCommand 处理命令
func (t *TelegramBot) handleCommand(message *tgbotapi.Message) {
	chatID := message.Chat.ID
	command := message.Command()

	var response string

	switch command {
	case "start":
		response = t.handleStart(message)
	case "sim_stats":
		response = t.handleSimStats()
	default:
		response = "❓ 未知命令。使用 /start 查看帮助。"
	}

	msg := tgbotapi.NewMessage(chatID, response)
	msg.ParseMode = "Markdown"
	t.bot.Send(msg)
}

// handleStart 处理 /start 命令
func (t *TelegramBot) handleStart(message *tgbotapi.Message) string {
	userName := message.From.FirstName
	if message.From.LastName != "" {
		userName += " " + message.From.LastName
	}

	return fmt.Sprintf(`👋 *欢迎, %s!*

🤖 *Polymarket 15-Min Crypto Arbitrage Tool*

这是一个简化版的 Polymarket 套利模拟工具，仅支持 Leg1 入场策略。

*可用命令:*
• /start - 显示此帮助
• /sim_stats - 查看模拟交易统计

📊 使用 /sim_stats 查看当前交易表现。`, userName)
}

// handleSimStats 处理 /sim_stats 命令
func (t *TelegramBot) handleSimStats() string {
	// 调用本地 API
	apiURL := fmt.Sprintf("http://localhost:%s/api/v1/sim-stats", getEnv("PORT", "8090"))
	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Sprintf("❌ 获取统计失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "❌ 读取响应失败"
	}

	// 解析响应
	var result struct {
		Success bool `json:"success"`
		Summary struct {
			TradeCount int     `json:"trade_count"`
			TotalPnL   float64 `json:"total_pnl"`
			WinCount   int     `json:"win_count"`
			WinRate    float64 `json:"win_rate"`
		} `json:"summary"`
		ConfigBreakdown map[string]struct {
			ConfigGroup string  `json:"config_group"`
			TradeCount  int     `json:"trade_count"`
			TotalPnL    float64 `json:"total_pnl"`
			WinCount    int     `json:"win_count"`
			WinRate     float64 `json:"win_rate"`
		} `json:"config_breakdown"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "❌ 解析响应失败"
	}

	if !result.Success {
		return "❌ API 返回错误"
	}

	// 格式化输出
	var output strings.Builder
	output.WriteString("📊 *模拟交易统计*\n\n")

	// 总体统计
	pnlEmoji := "📈"
	if result.Summary.TotalPnL < 0 {
		pnlEmoji = "📉"
	}

	output.WriteString(fmt.Sprintf("*总体表现:*\n"))
	output.WriteString(fmt.Sprintf("   交易次数: %d\n", result.Summary.TradeCount))
	output.WriteString(fmt.Sprintf("   %s 总 PnL: $%.2f\n", pnlEmoji, result.Summary.TotalPnL))
	output.WriteString(fmt.Sprintf("   胜率: %.1f%%\n\n", result.Summary.WinRate*100))

	// 配置组明细
	if len(result.ConfigBreakdown) > 0 {
		output.WriteString("*配置组明细:*\n")
		for name, stats := range result.ConfigBreakdown {
			emoji := "📈"
			if stats.TotalPnL < 0 {
				emoji = "📉"
			}
			output.WriteString(fmt.Sprintf("   [%s] %d笔 | %s $%.2f | %.1f%%\n",
				name, stats.TradeCount, emoji, stats.TotalPnL, stats.WinRate*100))
		}
	}

	if result.Summary.TradeCount == 0 {
		output.WriteString("📭 暂无已结算的交易数据。")
	}

	return output.String()
}

// getEnv 获取环境变量
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
