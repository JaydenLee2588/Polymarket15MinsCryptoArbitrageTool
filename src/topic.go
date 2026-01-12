package src

type Topic string

const (
	// TopicActivity 是活动主题
	TopicActivity Topic = "activity"

	// TopicComments 是评论主题
	TopicComments Topic = "comments"

	// TopicRfq 是询价 (RFQ) 主题
	TopicRfq Topic = "rfq"

	// TopicCryptoPrices 是加密货币价格主题
	TopicCryptoPrices Topic = "crypto_prices"

	// TopicCryptoPricesChainlink 是来自 Chainlink 的加密货币价格主题
	TopicCryptoPricesChainlink Topic = "crypto_prices_chainlink"

	// TopicEquityPrices 是股票价格主题
	TopicEquityPrices Topic = "equity_prices"

	// TopicClobUser 是 CLOB 用户数据主题
	TopicClobUser Topic = "clob_user"

	// TopicClobMarket 是 CLOB 市场数据主题
	TopicClobMarket Topic = "clob_market"

	// 用于向后兼容和清晰的别名
	TopicCLOBUser   = TopicClobUser
	TopicCLOBMarket = TopicClobMarket
)

