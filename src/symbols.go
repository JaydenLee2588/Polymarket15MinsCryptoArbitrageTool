package src

// 用于价格订阅的加密货币代码
const (
	// 主要加密货币
	CryptoSymbolBTCUSDT   = "btcusdt"   // 比特币
	CryptoSymbolETHUSDT   = "ethusdt"   // 以太坊
	CryptoSymbolSOLUSDT   = "solusdt"   // Solana
	CryptoSymbolBNBUSDT   = "bnbusdt"   // 币安币
	CryptoSymbolXRPUSDT   = "xrpusdt"   // 瑞波币
	CryptoSymbolADAUSDT   = "adausdt"   // 卡尔达诺
	CryptoSymbolDOGEUSDT  = "dogeusdt"  // 狗狗币
	CryptoSymbolMATICUSDT = "maticusdt" // Polygon
	CryptoSymbolDOTUSDT   = "dotusdt"   // 波卡
	CryptoSymbolAVAXUSDT  = "avaxusdt"  // 雪崩协议
	CryptoSymbolSHIBUSDT  = "shibusdt"  // 柴犬币
	CryptoSymbolLTCUSDT   = "ltcusdt"   // 莱特币
	CryptoSymbolLINKUSDT  = "linkusdt"  // Chainlink
	CryptoSymbolUNIUSDT   = "uniusdt"   // Uniswap
	CryptoSymbolATOMUSDT  = "atomusdt"  // Cosmos
	CryptoSymbolETCUSDT   = "etcusdt"   // 以太坊经典
	CryptoSymbolXLMUSDT   = "xlmusdt"   // 恒星币
	CryptoSymbolNEARUSDT  = "nearusdt"  // NEAR 协议
	CryptoSymbolAPTUSDT   = "aptusdt"   // Aptos
	CryptoSymbolARBUSDT   = "arbusdt"   // Arbitrum
	CryptoSymbolOPUSDT    = "opusdt"    // Optimism
)

// 用于价格订阅的股票代码
const (
	// 科技股
	EquitySymbolAAPL  = "AAPL"  // 苹果公司
	EquitySymbolMSFT  = "MSFT"  // 微软公司
	EquitySymbolGOOGL = "GOOGL" // Alphabet 公司 (谷歌)
	EquitySymbolAMZN  = "AMZN"  // 亚马逊公司
	EquitySymbolNVDA  = "NVDA"  // 英伟达公司
	EquitySymbolTSLA  = "TSLA"  // 特斯拉公司
	EquitySymbolMETA  = "META"  // Meta Platforms 公司 (Facebook)
	EquitySymbolNFLX  = "NFLX"  // Netflix 公司
	EquitySymbolAMD   = "AMD"   // 超威半导体 (AMD)
	EquitySymbolINTC  = "INTC"  // 英特尔公司
	EquitySymbolCSCO  = "CSCO"  // 思科系统公司
	EquitySymbolORCL  = "ORCL"  // 甲骨文公司
	EquitySymbolADBE  = "ADBE"  // Adobe 公司
	EquitySymbolCRM   = "CRM"   // Salesforce 公司
	EquitySymbolPYPL  = "PYPL"  // PayPal 控股公司

	// 金融股
	EquitySymbolJPM = "JPM" // 摩根大通公司
	EquitySymbolBAC = "BAC" // 美国银行公司
	EquitySymbolWFC = "WFC" // 富国银行公司
	EquitySymbolGS  = "GS"  // 高盛集团公司
	EquitySymbolMS  = "MS"  // 摩根士丹利
	EquitySymbolV   = "V"   // Visa 公司
	EquitySymbolMA  = "MA"  // 万事达卡公司

	// 医疗健康股
	EquitySymbolJNJ  = "JNJ"  // 强生公司
	EquitySymbolUNH  = "UNH"  // 联合健康集团公司
	EquitySymbolPFE  = "PFE"  // 辉瑞公司
	EquitySymbolABBV = "ABBV" // 艾伯维公司
	EquitySymbolMRK  = "MRK"  // 默克公司

	// 消费股
	EquitySymbolWMT = "WMT" // 沃尔玛公司
	EquitySymbolHD  = "HD"  // 家得宝公司
	EquitySymbolDIS = "DIS" // 华特迪士尼公司
	EquitySymbolNKE = "NKE" // 耐克公司
	EquitySymbolKO  = "KO"  // 可口可乐公司
	EquitySymbolPEP = "PEP" // 百事公司
	EquitySymbolMCD = "MCD" // 麦当劳公司

	// 能源股
	EquitySymbolXOM = "XOM" // 埃克森美孚公司
	EquitySymbolCVX = "CVX" // 雪佛龙公司

	// 工业股
	EquitySymbolBA  = "BA"  // 波音公司
	EquitySymbolCAT = "CAT" // 卡特彼勒公司
	EquitySymbolGE  = "GE"  // 通用电气公司
)

// 用于使用预定义代码创建过滤器的便捷函数

// NewBTCPriceFilter 创建比特币价格过滤器
func NewBTCPriceFilter() *CryptoPriceFilter {
	return NewCryptoPriceFilter(CryptoSymbolBTCUSDT)
}

// NewETHPriceFilter 创建以太坊价格过滤器
func NewETHPriceFilter() *CryptoPriceFilter {
	return NewCryptoPriceFilter(CryptoSymbolETHUSDT)
}

// NewSOLPriceFilter 创建 Solana 价格过滤器
func NewSOLPriceFilter() *CryptoPriceFilter {
	return NewCryptoPriceFilter(CryptoSymbolSOLUSDT)
}

// NewAppleStockFilter 创建苹果股票价格过滤器
func NewAppleStockFilter() *EquityPriceFilter {
	return NewEquityPriceFilter(EquitySymbolAAPL)
}

// NewTeslaStockFilter 创建特斯拉股票价格过滤器
func NewTeslaStockFilter() *EquityPriceFilter {
	return NewEquityPriceFilter(EquitySymbolTSLA)
}

// NewNvidiaStockFilter 创建英伟达股票价格过滤器
func NewNvidiaStockFilter() *EquityPriceFilter {
	return NewEquityPriceFilter(EquitySymbolNVDA)
}

