//go:build ignore

// M0 Day 0 public market-data probe (Refs #3).
//
// Standalone, read-only external connectivity probe. NOT business code, not part
// of `go build .` or `./...` (build tag `ignore`). Runs inside the builder image:
//
//	go run scripts/poc/wsprobe/main.go -seconds 30
//
// Uses only public endpoints; sends no credentials; places no orders.
// Uses the same endpoints/subscription shape as src/binance_ws.go:16,
// src/endpoint.go:8 and src/realtime/stream.go:254-268.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var hc = &http.Client{Timeout: 20 * time.Second}

func ts() string { return time.Now().UTC().Format(time.RFC3339) }

func getJSON(u string, v any) (int, error) {
	resp, err := hc.Get(u)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if v != nil {
		if err := json.Unmarshal(b, v); err != nil {
			return resp.StatusCode, fmt.Errorf("decode: %v body=%.200s", err, b)
		}
	}
	return resp.StatusCode, nil
}

// countWS dials, optionally sends subscription messages, and counts text
// messages received within d. Returns (handshake HTTP status, msgs, firstMsgLatency, sample).
func countWS(raw string, subs []any, d time.Duration) (int, int, time.Duration, string, error) {
	t0 := time.Now()
	c, resp, err := websocket.DefaultDialer.Dial(raw, nil)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	if err != nil {
		return status, 0, 0, "", err
	}
	defer c.Close()
	for _, s := range subs {
		if err := c.WriteJSON(s); err != nil {
			return status, 0, 0, "", fmt.Errorf("subscribe: %w", err)
		}
	}
	deadline := time.Now().Add(d)
	n, first, sample := 0, time.Duration(0), ""
	for time.Now().Before(deadline) {
		c.SetReadDeadline(deadline)
		_, msg, err := c.ReadMessage()
		if err != nil {
			if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
				break
			}
			return status, n, first, sample, err
		}
		if n == 0 {
			first = time.Since(t0)
			sample = string(msg)
			if len(sample) > 300 {
				sample = sample[:300] + "..."
			}
		}
		n++
	}
	return status, n, first, sample, nil
}

func main() {
	secs := flag.Int("seconds", 30, "seconds to collect per websocket")
	flag.Parse()
	d := time.Duration(*secs) * time.Second
	fail := false

	// ---- S07 Binance REST (src/binance_rest.go) ----
	fmt.Printf("== S07 Binance REST (%s)\n", ts())
	var kl []any
	st, err := getJSON("https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=15m&limit=2", &kl)
	fmt.Printf("GET api.binance.com/api/v3/klines BTCUSDT 15m -> http=%d rows=%d err=%v\n", st, len(kl), err)
	if st != 200 || len(kl) == 0 {
		fail = true
	}

	// ---- S08 Binance futures WS (src/binance_ws.go:16) ----
	fmt.Printf("== S08 Binance futures WS (%s) collect=%s\n", ts(), d)
	// First URL = exact shape the app uses (src/binance_ws.go:16). Others are
	// diagnostics only, to tell "endpoint changed" apart from "network blocked".
	binanceURLs := []string{
		"wss://fstream.binance.com/stream?streams=btcusdt@kline_1m/btcusdt@aggTrade",        // app shape
		"wss://fstream.binance.com/ws/btcusdt@aggTrade",                                     // diag: raw futures stream
		"wss://fstream.binance.com/market/stream?streams=btcusdt@kline_1m/btcusdt@aggTrade", // diag: futures /market path
		"wss://stream.binance.com:9443/stream?streams=btcusdt@kline_1m/btcusdt@aggTrade",    // diag: spot
	}
	var n int
	var first time.Duration
	var sample string
	for i, u := range binanceURLs {
		st, ni, fi, sa, e := countWS(u, nil, d)
		tag := "APP"
		if i > 0 {
			tag = "DIAG"
		}
		fmt.Printf("[%s] %s\n  handshake=%d msgs=%d first_msg_after=%s err=%v\n  sample=%s\n", tag, u, st, ni, fi.Round(time.Millisecond), e, sa)
		if i == 0 {
			n = ni
		}
	}
	if n == 0 {
		fmt.Println("RESULT S08: app-shape Binance WS URL delivered 0 messages")
		fail = true
	}

	// ---- S09 Polymarket market discovery (api/strategy.go:1180-1189) ----
	fmt.Printf("== S09 Polymarket discovery (%s)\n", ts())
	var sr struct {
		Events []struct {
			Slug    string `json:"slug"`
			Active  bool   `json:"active"`
			Closed  bool   `json:"closed"`
			EndDate string `json:"endDate"`
		} `json:"events"`
	}
	st, err = getJSON("https://gamma-api.polymarket.com/public-search?q="+url.QueryEscape("btc updown 15m")+"&events_status=active", &sr)
	fmt.Printf("GET gamma public-search q='btc updown 15m' active -> http=%d events=%d err=%v\n", st, len(sr.Events), err)
	var tokens []string
	chosen := ""
	for i, e := range sr.Events {
		if i < 8 {
			fmt.Printf("  slug=%s active=%v closed=%v end=%s\n", e.Slug, e.Active, e.Closed, e.EndDate)
		}
		if tokens != nil || !e.Active || e.Closed || !strings.Contains(e.Slug, "updown-15m") {
			continue
		}
		var evs []struct {
			Markets []struct {
				Question     string `json:"question"`
				ClobTokenIDs string `json:"clobTokenIds"`
				AcceptingOrd bool   `json:"acceptingOrders"`
			} `json:"markets"`
		}
		st2, err2 := getJSON("https://gamma-api.polymarket.com/events?slug="+url.QueryEscape(e.Slug), &evs)
		if err2 != nil || len(evs) == 0 || len(evs[0].Markets) == 0 {
			fmt.Printf("  events?slug=%s -> http=%d err=%v (skip)\n", e.Slug, st2, err2)
			continue
		}
		m := evs[0].Markets[0]
		var ids []string
		if json.Unmarshal([]byte(m.ClobTokenIDs), &ids) == nil && len(ids) > 0 {
			tokens, chosen = ids, e.Slug
			fmt.Printf("  chosen slug=%s question=%q acceptingOrders=%v tokens=%d\n", e.Slug, m.Question, m.AcceptingOrd, len(ids))
		}
	}
	if len(tokens) == 0 {
		fmt.Println("RESULT S09(app method public-search): no active *-updown-15m-* event returned")
		fail = true
		// External check: same slug rule the app parses (api/strategy.go:1225-1232).
		w := time.Now().Unix() / 900 * 900
		slug := fmt.Sprintf("btc-updown-15m-%d", w)
		var evs []struct {
			Title   string `json:"title"`
			Markets []struct {
				Question     string `json:"question"`
				ClobTokenIDs string `json:"clobTokenIds"`
				AcceptingOrd bool   `json:"acceptingOrders"`
			} `json:"markets"`
		}
		st2, err2 := getJSON("https://gamma-api.polymarket.com/events?slug="+slug, &evs)
		fmt.Printf("GET gamma events?slug=%s (computed current window) -> http=%d events=%d err=%v\n", slug, st2, len(evs), err2)
		if len(evs) > 0 && len(evs[0].Markets) > 0 {
			m := evs[0].Markets[0]
			var ids []string
			if json.Unmarshal([]byte(m.ClobTokenIDs), &ids) == nil && len(ids) > 0 {
				tokens, chosen = ids, slug
				fmt.Printf("  title=%q acceptingOrders=%v tokens=%d\n", evs[0].Title, m.AcceptingOrd, len(ids))
			}
		}
	}
	if len(tokens) == 0 {
		fmt.Println("RESULT S09: no active 15m market with clobTokenIds found")
		fmt.Println("RESULT S10: NOT_RUN (no token id)")
		os.Exit(1)
	}

	// ---- S10 Polymarket CLOB REST book + WS (src/realtime/rest.go, stream.go) ----
	fmt.Printf("== S10 Polymarket CLOB book REST (%s) slug=%s\n", ts(), chosen)
	var book struct {
		Bids []any `json:"bids"`
		Asks []any `json:"asks"`
	}
	st, err = getJSON("https://clob.polymarket.com/book?token_id="+tokens[0], &book)
	fmt.Printf("GET clob.polymarket.com/book?token_id=<tok0> -> http=%d bids=%d asks=%d err=%v\n", st, len(book.Bids), len(book.Asks), err)

	fmt.Printf("== S10 Polymarket CLOB market WS (%s) collect=%s\n", ts(), d)
	subs := []any{
		map[string]any{"assets_ids": tokens, "type": "market"},
		map[string]any{"assets_ids": tokens, "type": "book"},
	}
	st, n, first, sample, err = countWS("wss://ws-subscriptions-clob.polymarket.com/ws/market", subs, d)
	fmt.Printf("handshake=%d msgs=%d first_msg_after=%s err=%v\nsample=%s\n", st, n, first.Round(time.Millisecond), err, sample)
	if n == 0 {
		fail = true
	}
	fmt.Printf("== done (%s)\n", ts())
	if fail {
		os.Exit(1)
	}
}
