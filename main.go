package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"Polymarket15MinsCryptoArbitrageTool/api"
	"Polymarket15MinsCryptoArbitrageTool/bot"
	"Polymarket15MinsCryptoArbitrageTool/internal/database"
	"Polymarket15MinsCryptoArbitrageTool/src"

	"github.com/joho/godotenv"
)

// 命令行参数
var (
	enableTUI = flag.Bool("tui", false, "启用 TUI 套利监控面板")
)

func main() {
	// 解析命令行参数
	flag.Parse()

	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️ 加载 .env 文件失败: %v (使用系统环境变量)", err)
	}

	// 如果启用 TUI 模式，启动 TUI 监控
	var tuiMonitor *src.ArbitrageTUI
	if *enableTUI {
		log.Println("🖥️ TUI 套利监控模式已启用")
		tuiMonitor = src.NewArbitrageTUI(src.TUIConfig{
			RefreshRate: 1 * time.Second,
			GetMarketInfo: func() src.TUIMarketInfo {
				info := api.GetMarketInfo()
				return src.TUIMarketInfo{
					UpPrice:     info.UpPrice,
					DownPrice:   info.DownPrice,
					Slug:        info.Slug,
					Symbol:      info.Symbol,
					EndTime:     info.EndTime,
					StrikePrice: info.StrikePrice,
					HasData:     info.HasData,
				}
			},
			GetStrategyConfigs: func() []src.TUIStrategyConfig {
				// 从 api.GetCurrentConfigs 转换为 TUIStrategyConfig
				configs := api.GetCurrentConfigs()
				result := make([]src.TUIStrategyConfig, len(configs))
				for i, cfg := range configs {
					result[i] = src.TUIStrategyConfig{
						ID:               cfg.ID,
						MinPriceEdge:     cfg.MinPriceEdge,
						SharePriceLevel3: cfg.SharePriceLevel3,
						TradingMode:      cfg.TradingMode,
					}
				}
				return result
			},
			IsStrategyRunning: api.IsStrategyRunning,
		})
		// 禁用标准日志输出，避免干扰 TUI 显示
		log.SetOutput(io.Discard)
	}

	// 初始化内部 SQLite 数据库
	dbPath := getEnv("DB_PATH", "data/trades.db")
	if err := database.InitDB(dbPath); err != nil {
		log.Fatalf("❌ 数据库初始化失败: %v", err)
	}
	defer database.CloseDB()
	log.Printf("✅ 数据库已初始化: %s", dbPath)

	// 初始化数据记录器 (用于回测数据收集)
	// if err := datarecorder.GetInstance().Init(); err != nil {
	// 	log.Printf("⚠️ 数据记录器初始化失败: %v", err)
	// }
	// defer datarecorder.GetInstance().Close()

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

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 启动服务器 goroutine
	go func() {
		log.Printf("🚀 服务器启动在 http://%s", addr)
		log.Printf("📝 API 测试页面: http://%s/api/", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ 服务器启动失败: %v", err)
		}
	}()

	// 启动 TUI 监控 (如果启用)
	if tuiMonitor != nil {
		go tuiMonitor.Start()
		log.Println("📊 TUI 套利监控已启动")
	}

	// ========== 加载并启动 Pattern 策略 ==========
	enablePatternStrategy := getEnv("ENABLE_PATTERN_STRATEGY", "true")
	patternConfigFile := getEnv("PATTERN_CONFIG_FILE", "strategy_config_3.yaml")

	if enablePatternStrategy != "true" {
		log.Println("ℹ️ Pattern 策略已禁用 (ENABLE_PATTERN_STRATEGY=false)")
	} else if _, err := os.Stat(patternConfigFile); err == nil {
		patternConfigs, err := api.LoadPatternConfigs(patternConfigFile)
		if err != nil {
			log.Printf("⚠️ 加载 Pattern 配置失败: %v", err)
		} else if len(patternConfigs) > 0 {
			log.Printf("📋 已加载 Pattern 配置: %s (%d 个策略组)", patternConfigFile, len(patternConfigs))
			// Pattern 策略需要依赖主策略的市场数据，延迟启动
			go func() {
				// 等待主策略初始化市场数据
				time.Sleep(5 * time.Second)
				api.StartPatternStrategy(patternConfigs)
			}()
		}
	} else {
		log.Printf("ℹ️ Pattern 配置文件不存在: %s，跳过 Pattern 策略", patternConfigFile)
	}

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("🛑 正在关闭服务器...")

	// 停止 TUI 监控
	if tuiMonitor != nil {
		tuiMonitor.Stop()
	}

	// 停止 Pattern 策略
	if api.IsPatternStrategyRunning() {
		api.StopPatternStrategy()
	}

	// 创建 5 秒超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("❌ 服务器强制关闭: %v", err)
	}

	log.Println("✅ 服务器已优雅退出")
}

// getEnv 获取环境变量，如果未设置则返回默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
