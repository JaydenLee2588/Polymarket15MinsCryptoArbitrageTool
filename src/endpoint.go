package src

const (
	GammaAPIURL = "https://gamma-api.polymarket.com"
	DataAPIURL  = "https://data-api.polymarket.com"
	// 官方 CLOB WebSocket 端点 (来自 https://docs.polymarket.com/developers/CLOB/websocket)
	// 注意: 旧端点 wss://ws-live-data.polymarket.com 已被 Cloudflare 拦截返回 504
	WebSocketURL = "wss://ws-subscriptions-clob.polymarket.com/ws/market"
)
