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
	// 模拟交易表 (简化版)
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS simulation_trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_group TEXT DEFAULT 'default',
			market_slug TEXT NOT NULL,
			stage TEXT NOT NULL DEFAULT 'Leg1',
			side TEXT NOT NULL,
			price REAL NOT NULL,
			amount REAL NOT NULL,
			cost REAL NOT NULL,
			pnl REAL DEFAULT 0,
			final_resolution TEXT,
			trend_percent REAL,
			fair_price REAL,
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

	// 迁移: 添加 fair_price 列 (如果不存在)
	db.Exec(`ALTER TABLE simulation_trades ADD COLUMN fair_price REAL`)

	// 迁移: 删除旧列 (SQLite 不支持 DROP COLUMN，使用重建表的方式)
	cleanupOldColumns()

	return nil
}

// cleanupOldColumns 清理旧列 (strategy_type, symbol, binance_price)
func cleanupOldColumns() {
	// 检查是否存在旧列
	rows, err := db.Query(`PRAGMA table_info(simulation_trades)`)
	if err != nil {
		return
	}

	hasOldColumns := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == "strategy_type" || name == "symbol" || name == "binance_price" {
			hasOldColumns = true
			break
		}
	}
	rows.Close() // 必须在开始事务前关闭

	if !hasOldColumns {
		return
	}

	log.Println("🛠️ 检测到旧列，正在迁移数据库...")

	// SQLite 不支持 DROP COLUMN，需要重建表
	tx, err := db.Begin()
	if err != nil {
		log.Printf("⚠️ 迁移失败 (开始事务): %v", err)
		return
	}

	// 1. 创建新表
	_, err = tx.Exec(`
		CREATE TABLE simulation_trades_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_group TEXT DEFAULT 'default',
			market_slug TEXT NOT NULL,
			stage TEXT NOT NULL DEFAULT 'Leg1',
			side TEXT NOT NULL,
			price REAL NOT NULL,
			amount REAL NOT NULL,
			cost REAL NOT NULL,
			pnl REAL DEFAULT 0,
			final_resolution TEXT,
			trend_percent REAL,
			fair_price REAL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		tx.Rollback()
		log.Printf("⚠️ 迁移失败 (创建新表): %v", err)
		return
	}

	// 2. 复制数据
	_, err = tx.Exec(`
		INSERT INTO simulation_trades_new 
		(id, config_group, market_slug, stage, side, price, amount, cost, pnl, final_resolution, trend_percent, fair_price, created_at, updated_at)
		SELECT id, config_group, market_slug, stage, side, price, amount, cost, pnl, final_resolution, trend_percent, NULL, created_at, updated_at
		FROM simulation_trades
	`)
	if err != nil {
		tx.Rollback()
		log.Printf("⚠️ 迁移失败 (复制数据): %v", err)
		return
	}

	// 3. 删除旧表
	_, err = tx.Exec(`DROP TABLE simulation_trades`)
	if err != nil {
		tx.Rollback()
		log.Printf("⚠️ 迁移失败 (删除旧表): %v", err)
		return
	}

	// 4. 重命名新表
	_, err = tx.Exec(`ALTER TABLE simulation_trades_new RENAME TO simulation_trades`)
	if err != nil {
		tx.Rollback()
		log.Printf("⚠️ 迁移失败 (重命名): %v", err)
		return
	}

	// 5. 重建索引
	tx.Exec(`CREATE INDEX IF NOT EXISTS idx_trades_slug ON simulation_trades(market_slug)`)
	tx.Exec(`CREATE INDEX IF NOT EXISTS idx_trades_config ON simulation_trades(config_group)`)

	if err := tx.Commit(); err != nil {
		log.Printf("⚠️ 迁移失败 (提交): %v", err)
		return
	}

	log.Println("✅ 数据库迁移完成，已删除旧列")
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
	ConfigGroup     string
	MarketSlug      string
	Stage           string
	Side            string
	Price           float64
	Amount          float64
	Cost            float64
	PnL             float64
	FinalResolution string
	TrendPercent    float64
	FairPrice       float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// InsertTrade 插入交易记录
func InsertTrade(trade *SimulationTrade) (int64, error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	result, err := db.Exec(`
		INSERT INTO simulation_trades 
		(config_group, market_slug, stage, side, price, amount, cost, pnl, trend_percent, fair_price)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		trade.ConfigGroup,
		trade.MarketSlug,
		trade.Stage,
		trade.Side,
		trade.Price,
		trade.Amount,
		trade.Cost,
		trade.PnL,
		trade.TrendPercent,
		trade.FairPrice,
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
		SELECT id, config_group, market_slug, stage, side, price, amount, cost, pnl, 
		       COALESCE(final_resolution, '') as final_resolution, COALESCE(trend_percent, 0), created_at
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
			&t.ID, &t.ConfigGroup, &t.MarketSlug,
			&t.Stage, &t.Side, &t.Price, &t.Amount, &t.Cost, &t.PnL,
			&t.FinalResolution, &t.TrendPercent, &t.CreatedAt,
		)
		if err != nil {
			continue
		}
		trades = append(trades, t)
	}
	return trades, nil
}

// GetTradeStats 获取交易统计
func GetTradeStats() (tradeCount int, totalPnL float64, winCount int, err error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	query := `
		SELECT 
			COUNT(*) as trade_count,
			COALESCE(SUM(pnl), 0) as total_pnl,
			COALESCE(SUM(CASE WHEN pnl > 0 THEN 1 ELSE 0 END), 0) as win_count
		FROM simulation_trades
		WHERE final_resolution IS NOT NULL AND final_resolution != ''
	`
	err = db.QueryRow(query).Scan(&tradeCount, &totalPnL, &winCount)
	if err != nil {
		log.Printf("⚠️ [GetTradeStats] 查询失败: %v", err)
	} else {
		log.Printf("📊 [GetTradeStats] 查询结果: count=%d, pnl=%.2f, win=%d", tradeCount, totalPnL, winCount)
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
			COALESCE(SUM(CASE WHEN pnl > 0 THEN 1 ELSE 0 END), 0) as win_count
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

// GetEarliestTradeTime 获取最早的交易时间
func GetEarliestTradeTime() (time.Time, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	var earliest string
	err := db.QueryRow(`SELECT MIN(created_at) FROM simulation_trades`).Scan(&earliest)
	if err != nil || earliest == "" {
		return time.Time{}, err
	}

	// 解析时间
	t, err := time.Parse("2006-01-02 15:04:05", earliest)
	if err != nil {
		// 尝试带时区格式
		t, err = time.Parse(time.RFC3339, earliest)
		if err != nil {
			return time.Time{}, err
		}
	}

	return t, nil
}
