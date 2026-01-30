package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// 操作码 - 硬编码在后端，不暴露到前端
const operationCode = "qwer1234"

// ValidateOperationCode 验证操作码 (使用常量时间比较，防止时序攻击)
func ValidateOperationCode(code string) bool {
	return subtle.ConstantTimeCompare([]byte(code), []byte(operationCode)) == 1
}

// RequireOperationCode 中间件：要求操作码验证
func RequireOperationCode(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 从 Header 获取操作码
		code := r.Header.Get("X-Operation-Code")
		if !ValidateOperationCode(code) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "无效的操作码",
				"message": "请输入正确的操作码以执行此操作",
			})
			return
		}
		next(w, r)
	}
}
