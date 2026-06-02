package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"NovaQuant/internal/config"
	"NovaQuant/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type report struct {
	CheckedAt time.Time        `json:"checked_at"`
	Symbols   []symbolSnapshot `json:"symbols"`
	ETF       []etfSnapshot    `json:"etf"`
}

type symbolSnapshot struct {
	Symbol               string    `json:"symbol"`
	LatestPrice          float64   `json:"latest_price"`
	LatestPriceSource    string    `json:"latest_price_source"`
	LatestPriceTS        time.Time `json:"latest_price_ts"`
	Kline1MRows          int       `json:"kline_1m_rows"`
	Kline1HRows          int       `json:"kline_1h_rows"`
	LatestAnalysis1H     string    `json:"latest_analysis_1h_trend"`
	LatestAnalysis4H     string    `json:"latest_analysis_4h_trend"`
	LatestAnalysis1D     string    `json:"latest_analysis_1d_trend"`
	LatestAnalysis1HTS   time.Time `json:"latest_analysis_1h_ts"`
	LatestAnalysisSource string    `json:"latest_analysis_market_regime"`
}

type etfSnapshot struct {
	Asset         string    `json:"asset"`
	Rows          int       `json:"rows"`
	LatestFlowUSD float64   `json:"latest_flow_usd"`
	LatestFlowDay time.Time `json:"latest_flow_day"`
}

func main() {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.PGDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	out := report{
		CheckedAt: time.Now().UTC(),
		Symbols:   make([]symbolSnapshot, 0, 2),
		ETF:       make([]etfSnapshot, 0, 2),
	}

	for _, symbol := range []string{"BTCUSDT", "ETHUSDT"} {
		item, err := inspectSymbol(ctx, pool, symbol)
		if err != nil {
			log.Fatal(err)
		}
		out.Symbols = append(out.Symbols, item)
	}

	for _, asset := range []string{"BTC", "ETH"} {
		item, err := inspectETF(ctx, pool, asset)
		if err != nil {
			log.Fatal(err)
		}
		out.ETF = append(out.ETF, item)
	}

	raw, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(raw))
}

func inspectSymbol(ctx context.Context, pool *pgxpool.Pool, symbol string) (symbolSnapshot, error) {
	item := symbolSnapshot{Symbol: symbol}
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(price::double precision, 0), COALESCE(source, ''), COALESCE(ts, now())
		FROM market_price
		WHERE symbol = $1
		ORDER BY ts DESC
		LIMIT 1
	`, symbol).Scan(&item.LatestPrice, &item.LatestPriceSource, &item.LatestPriceTS); err != nil {
		return item, err
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM market_kline WHERE symbol = $1 AND interval = '1m'`, symbol).Scan(&item.Kline1MRows); err != nil {
		return item, err
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM market_kline WHERE symbol = $1 AND interval = '1h'`, symbol).Scan(&item.Kline1HRows); err != nil {
		return item, err
	}
	if err := pool.QueryRow(ctx, `
		SELECT trend, ts, market_regime
		FROM market_analysis
		WHERE symbol = $1 AND interval = '1h'
		ORDER BY ts DESC
		LIMIT 1
	`, symbol).Scan(&item.LatestAnalysis1H, &item.LatestAnalysis1HTS, &item.LatestAnalysisSource); err != nil {
		return item, err
	}
	if err := pool.QueryRow(ctx, `
		SELECT trend
		FROM market_analysis
		WHERE symbol = $1 AND interval = '4h'
		ORDER BY ts DESC
		LIMIT 1
	`, symbol).Scan(&item.LatestAnalysis4H); err != nil {
		return item, err
	}
	if err := pool.QueryRow(ctx, `
		SELECT trend
		FROM market_analysis
		WHERE symbol = $1 AND interval = '1d'
		ORDER BY ts DESC
		LIMIT 1
	`, symbol).Scan(&item.LatestAnalysis1D); err != nil {
		return item, err
	}
	return item, nil
}

func inspectETF(ctx context.Context, pool *pgxpool.Pool, asset string) (etfSnapshot, error) {
	item := etfSnapshot{Asset: asset}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM etf_flows WHERE asset = $1`, asset).Scan(&item.Rows); err != nil {
		return item, err
	}
	if item.Rows == 0 {
		return item, nil
	}
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(net_flow_usd::double precision, 0), flow_date::timestamp
		FROM etf_flows
		WHERE asset = $1
		ORDER BY flow_date DESC
		LIMIT 1
	`, asset).Scan(&item.LatestFlowUSD, &item.LatestFlowDay); err != nil {
		return item, err
	}
	return item, nil
}
