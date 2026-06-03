package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"NovaQuant/internal/domain"
	"NovaQuant/internal/repository"
)

func SeedDemoData(ctx context.Context, repo *repository.Repository, symbols []string) error {
	for i, symbol := range symbols {
		base := strings.TrimSuffix(symbol, "USDT")
		if _, err := repo.Pool().Exec(ctx, `
			INSERT INTO symbols(symbol, base_asset, quote_asset, market_type, sort_order)
			VALUES ($1, $2, 'USDT', 'spot', $3)
			ON CONFLICT(symbol) DO UPDATE SET enabled = true, updated_at = now()
		`, symbol, base, i+1); err != nil {
			return err
		}
	}

	if _, err := repo.Pool().Exec(ctx, `
		INSERT INTO alert_rules(name, symbol, rule_type, operator, threshold, cooldown_seconds)
		VALUES
		    ('BTC RSI Overheated', 'BTCUSDT', 'rsi', '>', 80, 1800),
		    ('ETH RSI Oversold', 'ETHUSDT', 'rsi', '<', 20, 1800),
		    ('BTC Risk High', 'BTCUSDT', 'risk_score', '>', 80, 3600)
		ON CONFLICT DO NOTHING
	`); err != nil {
		return err
	}

	analyzer := NewAnalyzer(repo, nil)
	for idx, symbol := range symbols {
		series := syntheticSeries(symbol, idx)
		if err := repo.UpsertKlines(ctx, series.Klines, "seed"); err != nil {
			return err
		}
		if err := repo.UpsertKlines(ctx, aggregateKlines(series.Klines, 4, "4h"), "seed"); err != nil {
			return err
		}
		if err := repo.UpsertKlines(ctx, aggregateKlines(series.Klines, 24, "1d"), "seed"); err != nil {
			return err
		}
		for _, kline := range series.Klines {
			if err := repo.UpsertPrice(ctx, symbol, kline.Close, kline.CloseTime, "seed"); err != nil {
				return err
			}
		}
		if err := repo.UpsertFunding(ctx, series.Funding, "seed"); err != nil {
			return err
		}
		for _, row := range series.OpenInterest {
			if err := repo.UpsertOpenInterest(ctx, row, "seed"); err != nil {
				return err
			}
		}
		if err := seedNews(ctx, repo, symbol, idx); err != nil {
			return err
		}
		if err := seedWhales(ctx, repo, symbol, idx); err != nil {
			return err
		}
		if err := seedETF(ctx, repo, symbol, idx); err != nil {
			return err
		}
		for _, interval := range []string{"1h", "4h", "1d"} {
			if err := analyzer.RunOnce(ctx, []string{symbol}, interval); err != nil {
				return err
			}
		}
	}

	alerts := NewAlerts(repo, nil)
	return alerts.RunOnce(ctx)
}

type seededSeries struct {
	Klines       []domain.Kline
	Funding      []domain.FundingRate
	OpenInterest []domain.OpenInterest
}

func syntheticSeries(symbol string, offset int) seededSeries {
	basePrices := map[string]float64{
		"BTCUSDT":  104000,
		"ETHUSDT":  5100,
		"SOLUSDT":  185,
		"BNBUSDT":  760,
		"DOGEUSDT": 0.21,
	}
	base := basePrices[symbol]
	if base == 0 {
		base = 100
	}
	// Keep the latest seeded candle fully closed at the current hour boundary.
	now := time.Now().UTC().Truncate(time.Hour)
	start := now.Add(-24 * 90 * time.Hour)
	klines := make([]domain.Kline, 0, 24*90)
	funding := make([]domain.FundingRate, 0, 24*90/8)
	oi := make([]domain.OpenInterest, 0, 24*90/6)
	prevClose := base * (0.95 + float64(offset)*0.03)

	for i := 0; i < 24*90; i++ {
		openTime := start.Add(time.Duration(i) * time.Hour)
		closeTime := openTime.Add(time.Hour)
		wave := math.Sin(float64(i+offset*7) * 0.18)
		cycle := math.Sin(float64(i) * 0.015)
		drift := 1 + 0.0015*wave + 0.0004*float64(offset+1) + 0.00035*cycle
		close := prevClose * drift
		open := prevClose
		high := math.Max(open, close) * (1 + 0.004 + math.Abs(wave)*0.002)
		low := math.Min(open, close) * (1 - 0.004 - math.Abs(wave)*0.002)
		volume := 1200 + 120*float64(offset+1) + math.Abs(wave)*800 + math.Abs(cycle)*400
		quoteVolume := volume * close
		klines = append(klines, domain.Kline{
			Symbol:      symbol,
			Interval:    "1h",
			OpenTime:    openTime,
			CloseTime:   closeTime,
			Open:        open,
			High:        high,
			Low:         low,
			Close:       close,
			Volume:      volume,
			QuoteVolume: quoteVolume,
			Trades:      int64(120 + i%40),
		})
		prevClose = close
		if i%8 == 0 {
			funding = append(funding, domain.FundingRate{
				Symbol:      symbol,
				FundingRate: 0.0001 + 0.00005*math.Sin(float64(i)*0.5),
				FundingTime: closeTime,
			})
		}
		if i%6 == 0 {
			oi = append(oi, domain.OpenInterest{
				Symbol:       symbol,
				OpenInterest: base*200 + float64(i*30) + math.Abs(wave)*base*5,
				TS:           closeTime,
			})
		}
	}

	return seededSeries{Klines: klines, Funding: funding, OpenInterest: oi}
}

func seedNews(ctx context.Context, repo *repository.Repository, symbol string, idx int) error {
	base := strings.TrimSuffix(symbol, "USDT")
	for i := 0; i < 2; i++ {
		url := fmt.Sprintf("https://novaquant.local/news/%s/%d", strings.ToLower(base), i)
		_, err := repo.Pool().Exec(ctx, `
			INSERT INTO news_items(source, title, url, published_at, related_symbols, sentiment, sentiment_score, summary)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT(url) DO UPDATE
			SET title = EXCLUDED.title, published_at = EXCLUDED.published_at, summary = EXCLUDED.summary
		`, "seed", fmt.Sprintf("%s chain activity update %d", base, i+1), url, time.Now().UTC().Add(-time.Duration(idx+i)*time.Hour), []string{symbol},
			[]string{"positive", "neutral"}[i%2], []float64{0.72, 0.12}[i%2], fmt.Sprintf("%s sentiment sample row for dashboard and agent evidence.", base))
		if err != nil {
			return err
		}
	}
	return nil
}

func seedWhales(ctx context.Context, repo *repository.Repository, symbol string, idx int) error {
	asset := strings.TrimSuffix(symbol, "USDT")
	_, err := repo.Pool().Exec(ctx, `
		INSERT INTO whale_transactions(chain, asset, amount, amount_usd, from_label, to_label, tx_hash, direction, ts, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'mock')
		ON CONFLICT(tx_hash) DO NOTHING
	`, "ethereum", asset, 1200+float64(idx*150), 9000000+float64(idx)*1500000, "large_exchange", "cold_wallet",
		fmt.Sprintf("seed-%s-whale", strings.ToLower(asset)), "outflow", time.Now().UTC().Add(-time.Duration(idx+2)*time.Hour))
	return err
}

func seedETF(ctx context.Context, repo *repository.Repository, symbol string, idx int) error {
	asset := strings.TrimSuffix(symbol, "USDT")
	if asset != "BTC" && asset != "ETH" {
		return nil
	}
	for i := 0; i < 30; i++ {
		_, err := repo.Pool().Exec(ctx, `
			INSERT INTO etf_flows(asset, provider, flow_date, net_flow_usd, total_volume_usd, note)
			VALUES ($1, 'seed', $2, $3, $4, $5)
			ON CONFLICT(asset, provider, flow_date) DO UPDATE
			SET net_flow_usd = EXCLUDED.net_flow_usd, total_volume_usd = EXCLUDED.total_volume_usd
		`, asset, time.Now().UTC().AddDate(0, 0, -i), 50000000-float64(i*1800000)+float64(idx)*1000000, 600000000+float64(i*10000000), "Seed ETF flow")
		if err != nil {
			return err
		}
	}
	return nil
}

func aggregateKlines(source []domain.Kline, step int, interval string) []domain.Kline {
	if step <= 1 || len(source) < step {
		return nil
	}
	result := make([]domain.Kline, 0, len(source)/step)
	for i := 0; i+step <= len(source); i += step {
		window := source[i : i+step]
		open := window[0]
		close := window[len(window)-1]
		high := window[0].High
		low := window[0].Low
		volume := 0.0
		quoteVolume := 0.0
		trades := int64(0)
		for _, item := range window {
			if item.High > high {
				high = item.High
			}
			if item.Low < low {
				low = item.Low
			}
			volume += item.Volume
			quoteVolume += item.QuoteVolume
			trades += item.Trades
		}
		result = append(result, domain.Kline{
			Symbol:      open.Symbol,
			Interval:    interval,
			OpenTime:    open.OpenTime,
			CloseTime:   close.CloseTime,
			Open:        open.Open,
			High:        high,
			Low:         low,
			Close:       close.Close,
			Volume:      volume,
			QuoteVolume: quoteVolume,
			Trades:      trades,
		})
	}
	return result
}
