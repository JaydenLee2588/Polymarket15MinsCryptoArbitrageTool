package realtime

import (
	"testing"
	"time"
)

func TestNewStream(t *testing.T) {
	stream := NewStream(StreamConfig{})

	if stream.url != DefaultWSURL {
		t.Errorf("stream.url = %v, want %v", stream.url, DefaultWSURL)
	}
	if stream.pingInterval != DefaultPingInterval {
		t.Errorf("stream.pingInterval = %v, want %v", stream.pingInterval, DefaultPingInterval)
	}
	if !stream.reconnect {
		t.Error("stream.reconnect should be true by default")
	}
}

func TestNewStreamWithCustomConfig(t *testing.T) {
	customURL := "wss://custom.example.com"
	customPing := 30 * time.Second

	stream := NewStream(StreamConfig{
		URL:          customURL,
		PingInterval: customPing,
	})

	if stream.url != customURL {
		t.Errorf("stream.url = %v, want %v", stream.url, customURL)
	}
	if stream.pingInterval != customPing {
		t.Errorf("stream.pingInterval = %v, want %v", stream.pingInterval, customPing)
	}
}

func TestParseBookUpdate(t *testing.T) {
	msg := StreamMessage{
		EventType: "book",
		Market:    "0x123",
		AssetID:   "asset123",
		Bids: []BookLevel{
			{Price: "0.55", Size: "100"},
			{Price: "0.54", Size: "200"},
		},
		Asks: []BookLevel{
			{Price: "0.56", Size: "150"},
			{Price: "0.57", Size: "250"},
		},
		Timestamp: "2024-01-01T00:00:00Z",
		Hash:      "hash123",
	}

	book := ParseBookUpdate(msg)

	if book.Market != msg.Market {
		t.Errorf("book.Market = %v, want %v", book.Market, msg.Market)
	}
	if book.AssetID != msg.AssetID {
		t.Errorf("book.AssetID = %v, want %v", book.AssetID, msg.AssetID)
	}
	if len(book.Bids) != 2 {
		t.Errorf("len(book.Bids) = %v, want 2", len(book.Bids))
	}
	if len(book.Asks) != 2 {
		t.Errorf("len(book.Asks) = %v, want 2", len(book.Asks))
	}
}

func TestParseTradeEvent(t *testing.T) {
	msg := StreamMessage{
		EventType: "last_trade_price",
		Market:    "0x123",
		AssetID:   "asset123",
		Price:     "0.55",
		Size:      "100",
		Side:      "BUY",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	trade, err := ParseTradeEvent(msg)
	if err != nil {
		t.Fatalf("ParseTradeEvent() error = %v", err)
	}

	if trade.Market != msg.Market {
		t.Errorf("trade.Market = %v, want %v", trade.Market, msg.Market)
	}
	if trade.Price.String() != "0.55" {
		t.Errorf("trade.Price = %v, want 0.55", trade.Price)
	}
	if trade.Size.String() != "100" {
		t.Errorf("trade.Size = %v, want 100", trade.Size)
	}
}

func TestGetBestBidAsk(t *testing.T) {
	book := &BookUpdate{
		Bids: []BookLevel{
			{Price: "0.55", Size: "100"},
			{Price: "0.54", Size: "200"},
		},
		Asks: []BookLevel{
			{Price: "0.56", Size: "150"},
			{Price: "0.57", Size: "250"},
		},
	}

	bidPrice, bidSize, bidOk := GetBestBid(book)
	if !bidOk {
		t.Error("GetBestBid() should return true")
	}
	if bidPrice.String() != "0.55" {
		t.Errorf("bidPrice = %v, want 0.55", bidPrice)
	}
	if bidSize.String() != "100" {
		t.Errorf("bidSize = %v, want 100", bidSize)
	}

	askPrice, askSize, askOk := GetBestAsk(book)
	if !askOk {
		t.Error("GetBestAsk() should return true")
	}
	if askPrice.String() != "0.56" {
		t.Errorf("askPrice = %v, want 0.56", askPrice)
	}
	if askSize.String() != "150" {
		t.Errorf("askSize = %v, want 150", askSize)
	}
}

func TestGetMidPrice(t *testing.T) {
	book := &BookUpdate{
		Bids: []BookLevel{{Price: "0.54", Size: "100"}},
		Asks: []BookLevel{{Price: "0.56", Size: "100"}},
	}

	mid, ok := GetMidPrice(book)
	if !ok {
		t.Error("GetMidPrice() should return true")
	}
	if mid.String() != "0.55" {
		t.Errorf("mid = %v, want 0.55", mid)
	}
}

func TestGetSpread(t *testing.T) {
	book := &BookUpdate{
		Bids: []BookLevel{{Price: "0.54", Size: "100"}},
		Asks: []BookLevel{{Price: "0.56", Size: "100"}},
	}

	spread, ok := GetSpread(book)
	if !ok {
		t.Error("GetSpread() should return true")
	}
	if spread.String() != "0.02" {
		t.Errorf("spread = %v, want 0.02", spread)
	}
}

func TestSessionManager(t *testing.T) {
	manager := NewSessionManager(SessionManagerConfig{})

	if manager.GetCurrentSession() != nil {
		t.Error("GetCurrentSession() should return nil initially")
	}

	// 不实际连接，只测试接口
	manager.Stop()
}

func TestMarketSession(t *testing.T) {
	session := NewMarketSession(MarketSessionConfig{
		AssetIDs:      []string{"token1", "token2"},
		ChannelBuffer: 50,
	})

	if len(session.GetAssetIDs()) != 2 {
		t.Errorf("len(GetAssetIDs()) = %v, want 2", len(session.GetAssetIDs()))
	}

	if session.GetDuration() < 0 {
		t.Error("GetDuration() should be non-negative")
	}
}
