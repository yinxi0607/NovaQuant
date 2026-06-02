package service

import (
	"testing"
	"time"

	"NovaQuant/internal/domain"
)

func TestComputeAnalysisProducesRiskAndSummary(t *testing.T) {
	klines := make([]domain.Kline, 0, 220)
	baseTime := time.Now().UTC().Add(-220 * time.Hour)
	price := 1000.0
	for i := 0; i < 220; i++ {
		next := price * (1 + 0.002)
		klines = append(klines, domain.Kline{
			Symbol:      "BTCUSDT",
			Interval:    "1h",
			OpenTime:    baseTime.Add(time.Duration(i) * time.Hour),
			CloseTime:   baseTime.Add(time.Duration(i+1) * time.Hour),
			Open:        price,
			High:        next * 1.01,
			Low:         price * 0.99,
			Close:       next,
			Volume:      1000 + float64(i),
			QuoteVolume: (1000 + float64(i)) * next,
		})
		price = next
	}

	funding := []domain.FundingRate{{Symbol: "BTCUSDT", FundingRate: 0.0003, FundingTime: time.Now().UTC()}}
	oi := []domain.OpenInterest{
		{Symbol: "BTCUSDT", OpenInterest: 1100, TS: time.Now().UTC()},
		{Symbol: "BTCUSDT", OpenInterest: 1000, TS: time.Now().UTC().Add(-time.Hour)},
	}
	news := []domain.NewsItem{{Sentiment: "negative", PublishedAt: time.Now().UTC()}}

	analysis, risk := ComputeAnalysis("BTCUSDT", "1h", klines, funding, oi, news)
	if analysis.Summary == "" {
		t.Fatalf("expected summary")
	}
	if risk.TotalScore <= 0 {
		t.Fatalf("expected positive risk score")
	}
	if analysis.Trend == "" || analysis.MarketRegime == "" {
		t.Fatalf("expected trend and market regime")
	}
}
