package bot

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var botDB *sql.DB

// InitBotStorage 初始化 Bot 存储
func InitBotStorage(dbPath string) error {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("无法创建数据库目录: %w", err)
	}

	var err error
	botDB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("无法打开 Bot 数据库: %w", err)
	}

	// 创建表
	_, err = botDB.Exec(`
		CREATE TABLE IF NOT EXISTS user_settings (
			chat_id INTEGER PRIMARY KEY,
			language TEXT DEFAULT 'zh',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	return nil
}

// CloseBotStorage 关闭 Bot 存储
func CloseBotStorage() {
	if botDB != nil {
		botDB.Close()
	}
}
