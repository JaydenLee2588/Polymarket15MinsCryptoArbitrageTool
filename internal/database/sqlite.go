package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	db   *sql.DB
	dbMu sync.RWMutex
)

// InitDB 初始化 SQLite 数据库
func InitDB(dbPath string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("无法创建数据库目录: %w", err)
	}

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("无法打开数据库: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	// 启用 WAL 模式
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("⚠️ 无法启用 WAL 模式: %v", err)
	}

	// 创建表
	if err := createTables(); err != nil {
		return fmt.Errorf("无法创建表: %w", err)
	}

	return nil
}

// createTables 创建必要的数据库表
func createTables() error {
	// 模拟交易表
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS simulation_trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			strategy_type TEXT NOT NULL DEFAULT 'type2',
			config_group TEXT DEFAULT 'default',
			market_slug TEXT NOT NULL,
			symbol TEXT,
			stage TEXT NOT NULL DEFAULT 'Leg1',
			side TEXT NOT NULL,
			price REAL NOT NULL,
			amount REAL NOT NULL,
			cost REAL NOT NULL,
			pnl REAL DEFAULT 0,
			final_resolution TEXT,
			binance_price REAL,
			trend_percent REAL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 创建索引
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_trades_slug ON simulation_trades(market_slug)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_trades_config ON simulation_trades(config_group)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_trades_strategy ON simulation_trades(strategy_type)`)

	return nil
}

// GetDB 获取数据库实例
func GetDB() *sql.DB {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return db
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	dbMu.Lock()
	defer dbMu.Unlock()
	if db != nil {
		return db.Close()
	}
	return nil
}

// SimulationTrade 模拟交易记录
type SimulationTrade struct {
	ID              int64
	StrategyType    string
	ConfigGroup     string
	MarketSlug      string
	Symbol          string
	Stage           string
	Side            string
	Price           float64
	Amount          float64
	Cost            float64
	PnL             float64
	FinalResolution string
	BinancePrice    float64
	TrendPercent    float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// InsertTrade 插入交易记录
func InsertTrade(trade *SimulationTrade) (int64, error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	result, err := db.Exec(`
		INSERT INTO simulation_trades 
		(strategy_type, config_group, market_slug, symbol, stage, side, price, amount, cost, pnl, binance_price, trend_percent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		trade.StrategyType,
		trade.ConfigGroup,
		trade.MarketSlug,
		trade.Symbol,
		trade.Stage,
		trade.Side,
		trade.Price,
		trade.Amount,
		trade.Cost,
		trade.PnL,
		trade.BinancePrice,
		trade.TrendPercent,
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// UpdateTradeResolution 更新交易的最终决议
func UpdateTradeResolution(id int64, resolution string, pnl float64) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := db.Exec(`
		UPDATE simulation_trades 
		SET final_resolution = ?, pnl = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, resolution, pnl, id)
	return err
}

// GetUnresolvedTrades 获取未决议的交易
func GetUnresolvedTrades() ([]*SimulationTrade, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	rows, err := db.Query(`
		SELECT id, strategy_type, config_group, market_slug, symbol, stage, side, price, amount, cost, pnl, 
		       COALESCE(final_resolution, '') as final_resolution, binance_price, trend_percent, created_at
		FROM simulation_trades 
		WHERE final_resolution IS NULL OR final_resolution = ''
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*SimulationTrade
	for rows.Next() {
		t := &SimulationTrade{}
		err := rows.Scan(
			&t.ID, &t.StrategyType, &t.ConfigGroup, &t.MarketSlug, &t.Symbol,
			&t.Stage, &t.Side, &t.Price, &t.Amount, &t.Cost, &t.PnL,
			&t.FinalResolution, &t.BinancePrice, &t.TrendPercent, &t.CreatedAt,
		)
		if err != nil {
			continue
		}
		trades = append(trades, t)
	}
	return trades, nil
}

// GetTradeStats 获取交易统计
func GetTradeStats(strategyType string) (tradeCount int, totalPnL float64, winCount int, err error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	query := `
		SELECT 
			COUNT(*) as trade_count,
			COALESCE(SUM(pnl), 0) as total_pnl,
			SUM(CASE WHEN pnl > 0 THEN 1 ELSE 0 END) as win_count
		FROM simulation_trades
		WHERE final_resolution IS NOT NULL AND final_resolution != ''
	`
	if strategyType != "" {
		query += " AND strategy_type = ?"
		err = db.QueryRow(query, strategyType).Scan(&tradeCount, &totalPnL, &winCount)
	} else {
		err = db.QueryRow(query).Scan(&tradeCount, &totalPnL, &winCount)
	}
	return
}

// GetTradeStatsByConfig 获取按配置组的交易统计
func GetTradeStatsByConfig() (map[string]struct {
	TradeCount int
	TotalPnL   float64
	WinCount   int
}, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	rows, err := db.Query(`
		SELECT 
			config_group,
			COUNT(*) as trade_count,
			COALESCE(SUM(pnl), 0) as total_pnl,
			SUM(CASE WHEN pnl > 0 THEN 1 ELSE 0 END) as win_count
		FROM simulation_trades
		WHERE final_resolution IS NOT NULL AND final_resolution != ''
		GROUP BY config_group
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]struct {
		TradeCount int
		TotalPnL   float64
		WinCount   int
	})

	for rows.Next() {
		var configGroup string
		var stats struct {
			TradeCount int
			TotalPnL   float64
			WinCount   int
		}
		if err := rows.Scan(&configGroup, &stats.TradeCount, &stats.TotalPnL, &stats.WinCount); err != nil {
			continue
		}
		result[configGroup] = stats
	}

	return result, nil
}
