package api

import (
	"log"
	"net/http"

	"Polymarket15MinsCryptoArbitrageTool/internal/database"
)

// DeleteTradesHandler 处理删除所有交易记录的请求
// DELETE /api/v1/trades
func DeleteTradesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := database.DeleteAllTrades(); err != nil {
		log.Printf("❌ [DeleteTradesHandler] 删除失败: %v", err)
		http.Error(w, "Failed to delete trades", http.StatusInternalServerError)
		return
	}

	log.Println("🗑️ [DeleteTradesHandler] 所有模拟交易记录已清空")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true, "message": "All trades deleted"}`))
}
