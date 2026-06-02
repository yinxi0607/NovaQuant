package service

import (
	"testing"
	"time"

	"NovaQuant/internal/domain"
)

func TestBacktestProducesTrades(t *testing.T) {
	klines := make([]domain.Kline, 0, 120)
	base := time.Now().UTC().Add(-120 * time.Hour)
	price := 100.0
	for i := 0; i < 120; i++ {
		drift := 1.0
		if i%15 < 8 {
			drift = 1.01
		} else {
			drift = 0.995
		}
		next := price * drift
		klines = append(klines, domain.Kline{
			OpenTime:  base.Add(time.Duration(i) * time.Hour),
			CloseTime: base.Add(time.Duration(i+1) * time.Hour),
			Open:      price,
			High:      next * 1.01,
			Low:       next * 0.99,
			Close:     next,
			Volume:    1200,
		})
		price = next
	}

	result := RunBacktest("ema_crossover", klines)
	if result.TradeCount == 0 {
		t.Fatalf("expected at least one trade")
	}
	if len(result.EquityCurve) == 0 {
		t.Fatalf("expected equity curve")
	}
}
