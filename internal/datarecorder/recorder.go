package datarecorder

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Recorder handles recording market data to CSV files.
type Recorder struct {
	mu          sync.Mutex
	baseDir     string
	startTime   time.Time
	klineFile   *os.File
	klineWriter *csv.Writer
	obFile      *os.File
	obWriter    *csv.Writer
	initialized bool
}

var (
	GlobalRecorder *Recorder
	once           sync.Once
)

// GetInstance returns the singleton instance of Recorder.
func GetInstance() *Recorder {
	once.Do(func() {
		GlobalRecorder = &Recorder{
			baseDir: "data/history",
		}
	})
	return GlobalRecorder
}

// Init initializes the recorder with a start time.
// Creates files: {timestamp}_klines.csv and {timestamp}_orderbooks.csv
func (r *Recorder) Init() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return nil
	}

	r.startTime = time.Now()
	timeStr := r.startTime.Format("20060102_150405")

	// Ensure directory exists
	if err := os.MkdirAll(r.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create Kline file
	klinePath := filepath.Join(r.baseDir, fmt.Sprintf("%s_klines.csv", timeStr))
	kf, err := os.Create(klinePath)
	if err != nil {
		return fmt.Errorf("failed to create kline file: %w", err)
	}
	r.klineFile = kf
	r.klineWriter = csv.NewWriter(kf)
	// Write header
	r.klineWriter.Write([]string{"Time", "Symbol", "Open", "High", "Low", "Close", "Volume"})
	r.klineWriter.Flush()

	// Create OrderBook file
	obPath := filepath.Join(r.baseDir, fmt.Sprintf("%s_orderbooks.csv", timeStr))
	of, err := os.Create(obPath)
	if err != nil {
		return fmt.Errorf("failed to create orderbook file: %w", err)
	}
	r.obFile = of
	r.obWriter = csv.NewWriter(of)
	// Write header: Time, TokenID, BestBid, BestAsk, Spread
	r.obWriter.Write([]string{"Time", "TokenID", "BestBid", "BestAsk", "Spread"})
	r.obWriter.Flush()

	r.initialized = true
	fmt.Printf("📝 Data Recorder Initialized: %s\n", timeStr)
	return nil
}

// RecordKline records a kline update.
func (r *Recorder) RecordKline(symbol string, open, high, low, close, volume float64, timestamp int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return
	}

	record := []string{
		fmt.Sprintf("%d", timestamp),
		symbol,
		fmt.Sprintf("%.8f", open),
		fmt.Sprintf("%.8f", high),
		fmt.Sprintf("%.8f", low),
		fmt.Sprintf("%.8f", close),
		fmt.Sprintf("%.8f", volume),
	}
	r.klineWriter.Write(record)
	r.klineWriter.Flush() // Flush frequently to avoid data loss on crash
}

// RecordOrderBook records a snapshot of the top of the book.
func (r *Recorder) RecordOrderBook(tokenID string, bestBid, bestAsk float64, timestamp int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return
	}

	spread := bestAsk - bestBid

	record := []string{
		fmt.Sprintf("%d", timestamp),
		tokenID,
		fmt.Sprintf("%.4f", bestBid),
		fmt.Sprintf("%.4f", bestAsk),
		fmt.Sprintf("%.4f", spread),
	}
	r.obWriter.Write(record)
	r.obWriter.Flush()
}

// Data Structs for Reading
type KlineRecord struct {
	Timestamp int64
	Symbol    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

type OrderBookRecord struct {
	Timestamp int64
	TokenID   string
	BestBid   float64
	BestAsk   float64
	Spread    float64
}

// LoadKlines reads a CSV file and returns a slice of KlineRecord
func LoadKlines(filePath string) ([]KlineRecord, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var klines []KlineRecord
	// Skip header
	for i, record := range records {
		if i == 0 {
			continue
		}
		if len(record) < 7 {
			continue
		}

		ts, _ := strconv.ParseInt(record[0], 10, 64)
		open, _ := strconv.ParseFloat(record[2], 64)
		high, _ := strconv.ParseFloat(record[3], 64)
		low, _ := strconv.ParseFloat(record[4], 64)
		closeP, _ := strconv.ParseFloat(record[5], 64)
		vol, _ := strconv.ParseFloat(record[6], 64)

		klines = append(klines, KlineRecord{
			Timestamp: ts,
			Symbol:    record[1],
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closeP,
			Volume:    vol,
		})
	}
	return klines, nil
}

// LoadOrderBooks reads a CSV file and returns a slice of OrderBookRecord
func LoadOrderBooks(filePath string) ([]OrderBookRecord, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var books []OrderBookRecord
	// Skip header
	for i, record := range records {
		if i == 0 {
			continue
		}
		if len(record) < 4 {
			continue
		}

		ts, _ := strconv.ParseInt(record[0], 10, 64)
		bid, _ := strconv.ParseFloat(record[2], 64)
		ask, _ := strconv.ParseFloat(record[3], 64)
		spread := 0.0
		if len(record) > 4 {
			spread, _ = strconv.ParseFloat(record[4], 64)
		}

		books = append(books, OrderBookRecord{
			Timestamp: ts,
			TokenID:   record[1],
			BestBid:   bid,
			BestAsk:   ask,
			Spread:    spread,
		})
	}
	return books, nil
}

// Close closes all open files.
func (r *Recorder) Close() {
	// ...
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.klineWriter != nil {
		r.klineWriter.Flush()
	}
	if r.klineFile != nil {
		r.klineFile.Close()
	}
	if r.obWriter != nil {
		r.obWriter.Flush()
	}
	if r.obFile != nil {
		r.obFile.Close()
	}
	r.initialized = false
}
