package realtime

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// ParseBookUpdate 从流消息解析订单簿更新
func ParseBookUpdate(msg StreamMessage) *BookUpdate {
	return &BookUpdate{
		Market:    msg.Market,
		AssetID:   msg.AssetID,
		Bids:      msg.Bids,
		Asks:      msg.Asks,
		Timestamp: msg.Timestamp,
		Hash:      msg.Hash,
	}
}

// ParseTradeEvent 从流消息解析交易事件
func ParseTradeEvent(msg StreamMessage) (*TradeEvent, error) {
	price, err := decimal.NewFromString(msg.Price)
	if err != nil {
		return nil, fmt.Errorf("invalid price: %w", err)
	}
	size, err := decimal.NewFromString(msg.Size)
	if err != nil {
		return nil, fmt.Errorf("invalid size: %w", err)
	}

	return &TradeEvent{
		Market:     msg.Market,
		AssetID:    msg.AssetID,
		Side:       msg.Side,
		Price:      price,
		Size:       size,
		FeeRateBps: msg.FeeRateBps,
		Timestamp:  msg.Timestamp,
	}, nil
}

// ParsePriceChange 从流消息解析价格变动
func ParsePriceChange(msg StreamMessage, item PriceChangeItem) (*PriceChange, error) {
	price, err := decimal.NewFromString(item.Price)
	if err != nil {
		return nil, fmt.Errorf("invalid price: %w", err)
	}
	size, err := decimal.NewFromString(item.Size)
	if err != nil {
		return nil, fmt.Errorf("invalid size: %w", err)
	}
	bestBid, _ := decimal.NewFromString(item.BestBid)
	bestAsk, _ := decimal.NewFromString(item.BestAsk)

	return &PriceChange{
		Market:    msg.Market,
		AssetID:   item.AssetID,
		Price:     price,
		Size:      size,
		Side:      item.Side,
		BestBid:   bestBid,
		BestAsk:   bestAsk,
		Timestamp: msg.Timestamp,
	}, nil
}

// GetBestBid 从订单簿获取最佳买价 (最高买价)
func GetBestBid(book *BookUpdate) (price decimal.Decimal, size decimal.Decimal, ok bool) {
	if len(book.Bids) == 0 {
		return decimal.Zero, decimal.Zero, false
	}

	// Polymarket 订单簿可能未排序，需要遍历找最高买价
	bestPrice := decimal.Zero
	bestSize := decimal.Zero
	for _, bid := range book.Bids {
		p, err := decimal.NewFromString(bid.Price)
		if err != nil {
			continue
		}
		if p.GreaterThan(bestPrice) {
			bestPrice = p
			bestSize, _ = decimal.NewFromString(bid.Size)
		}
	}

	if bestPrice.IsZero() {
		return decimal.Zero, decimal.Zero, false
	}
	return bestPrice, bestSize, true
}

// GetBestAsk 从订单簿获取最佳卖价 (最低卖价)
func GetBestAsk(book *BookUpdate) (price decimal.Decimal, size decimal.Decimal, ok bool) {
	if len(book.Asks) == 0 {
		return decimal.Zero, decimal.Zero, false
	}

	// Polymarket 订单簿可能未排序，需要遍历找最低卖价
	bestPrice := decimal.NewFromInt(999) // 初始化为一个很大的值
	bestSize := decimal.Zero
	for _, ask := range book.Asks {
		p, err := decimal.NewFromString(ask.Price)
		if err != nil {
			continue
		}
		if p.LessThan(bestPrice) && p.GreaterThan(decimal.Zero) {
			bestPrice = p
			bestSize, _ = decimal.NewFromString(ask.Size)
		}
	}

	if bestPrice.Equal(decimal.NewFromInt(999)) {
		return decimal.Zero, decimal.Zero, false
	}
	return bestPrice, bestSize, true
}

// GetMidPrice 计算中间价
func GetMidPrice(book *BookUpdate) (decimal.Decimal, bool) {
	bidPrice, _, bidOk := GetBestBid(book)
	askPrice, _, askOk := GetBestAsk(book)

	if !bidOk || !askOk {
		return decimal.Zero, false
	}

	return bidPrice.Add(askPrice).Div(decimal.NewFromInt(2)), true
}

// GetSpread 计算买卖价差
func GetSpread(book *BookUpdate) (decimal.Decimal, bool) {
	bidPrice, _, bidOk := GetBestBid(book)
	askPrice, _, askOk := GetBestAsk(book)

	if !bidOk || !askOk {
		return decimal.Zero, false
	}

	return askPrice.Sub(bidPrice), true
}
