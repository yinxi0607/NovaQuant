package service

import (
	"math"
	"testing"
	"time"

	"NovaQuant/internal/domain"
)

func TestRSIProducesExpectedRange(t *testing.T) {
	values := []float64{44, 44.15, 43.9, 44.35, 44.8, 45.05, 44.7, 44.95, 45.4, 45.2, 45.85, 46.1, 45.7, 46.4, 46.8, 46.55, 46.95, 47.2, 47.55, 47.9}
	rsi := RSI(values, 14)
	last := rsi[len(rsi)-1]
	if last <= 0 || last >= 100 {
		t.Fatalf("expected RSI between 0 and 100, got %.4f", last)
	}
	if math.Abs(last-82.92) > 8 {
		t.Fatalf("unexpected RSI value %.4f", last)
	}
}

func TestSupportResistanceReturnsDistinctLevels(t *testing.T) {
	klines := make([]domain.Kline, 0, 20)
	baseTime := time.Now().UTC().Add(-20 * time.Hour)
	prices := []float64{100, 97, 102, 96, 103, 99, 105, 98, 107, 101, 108, 100, 110, 102, 111, 103, 113, 104, 112, 106}
	for i, price := range prices {
		klines = append(klines, domain.Kline{
			OpenTime:  baseTime.Add(time.Duration(i) * time.Hour),
			CloseTime: baseTime.Add(time.Duration(i+1) * time.Hour),
			Open:      price - 1,
			High:      price + 2,
			Low:       price - 2,
			Close:     price,
			Volume:    100 + float64(i),
		})
	}

	supports, resistances := SupportResistance(klines, 2)
	if len(supports) == 0 || len(resistances) == 0 {
		t.Fatalf("expected support and resistance levels, got %v %v", supports, resistances)
	}
}
