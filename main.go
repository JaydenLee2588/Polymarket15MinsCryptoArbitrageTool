package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"Polymarket15MinsCryptoArbitrageTool/api"
	"Polymarket15MinsCryptoArbitrageTool/bot"
	"Polymarket15MinsCryptoArbitrageTool/internal/database"

	"github.com/joho/godotenv"
)

func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Println("未找到 .env 文件，使用系统环境变量")
	}

	// 初始化内部 SQLite 数据库
	dbPath := getEnv("DB_PATH", "data/trades.db")
	if err := database.InitDB(dbPath); err != nil {
		log.Fatalf("❌ 数据库初始化失败: %v", err)
	}
	defer database.CloseDB()
	log.Printf("✅ 数据库已初始化: %s", dbPath)

	// 设置 API 数据库引用
	api.SetDatabase(database.GetDB())

	// 启动 Telegram Bot (如果配置了 Token)
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if telegramToken != "" {
		tgBot, err := bot.New(telegramToken, getEnv("BOT_DB_PATH", "data/bot.db"))
		if err != nil {
			log.Printf("⚠️ Telegram Bot 启动失败: %v", err)
		} else {
			go tgBot.Start()
			log.Println("🤖 Telegram Bot 已启动")
		}
	} else {
		log.Println("ℹ️ 未配置 TELEGRAM_BOT_TOKEN，跳过 Telegram Bot")
	}

	// 创建 HTTP 路由
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "Polymarket15MinsCryptoArbitrageTool",
		})
	})

	// 注册业务 API 路由
	api.RegisterRoutes(mux)

	// 静态文件服务 (API 测试页面)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// API 路由直接返回
		if strings.HasPrefix(path, "/api/") {
			if path == "/api/" {
				http.ServeFile(w, r, "front/api-test.html")
				return
			}
			http.NotFound(w, r)
			return
		}

		// 静态文件
		switch path {
		case "/", "/index.html":
			http.ServeFile(w, r, "front/index.html")
		case "/api-test.html":
			http.ServeFile(w, r, "front/api-test.html")
		default:
			http.ServeFile(w, r, "front/index.html")
		}
	})

	// 启动 HTTP 服务器
	host := getEnv("HOST", "0.0.0.0")
	port := getEnv("PORT", "8090")
	addr := fmt.Sprintf("%s:%s", host, port)

	log.Printf("🚀 服务器启动在 http://%s", addr)
	log.Printf("📝 API 测试页面: http://%s/api/", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ 服务器启动失败: %v", err)
	}
}

// getEnv 获取环境变量，如果未设置则返回默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
