package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"NovaQuant/internal/domain"
	"NovaQuant/internal/metrics"
	"NovaQuant/internal/repository"
)

type Analyzer struct {
	repo    *repository.Repository
	metrics *metrics.Registry
}

func NewAnalyzer(repo *repository.Repository, registry *metrics.Registry) *Analyzer {
	return &Analyzer{repo: repo, metrics: registry}
}

func (a *Analyzer) RunOnce(ctx context.Context, symbols []string, interval string) error {
	for _, symbol := range symbols {
		klines, err := a.repo.ListKlines(ctx, symbol, interval, 260)
		if err != nil {
			return err
		}
		if len(klines) < 30 {
			continue
		}
		funding, err := a.repo.ListRecentFunding(ctx, symbol, 8)
		if err != nil {
			return err
		}
		oi, err := a.repo.ListRecentOpenInterest(ctx, symbol, 8)
		if err != nil {
			return err
		}
		news, err := a.repo.ListNews(ctx, symbol, 5)
		if err != nil {
			return err
		}
		analysis, risk := ComputeAnalysis(symbol, interval, klines, funding, oi, news)
		if err := a.repo.UpsertAnalysis(ctx, analysis); err != nil {
			return err
		}
		if err := a.repo.UpsertRisk(ctx, risk); err != nil {
			return err
		}
		if a.metrics != nil {
			a.metrics.Inc("indicator_calculation_total")
			a.metrics.Inc("analysis_rows_written_total")
		}
	}
	return nil
}

func ComputeAnalysis(symbol, interval string, klines []domain.Kline, funding []domain.FundingRate, oi []domain.OpenInterest, news []domain.NewsItem) (domain.Analysis, domain.Risk) {
	closeSeries := closes(klines)
	volumeSeries := volumes(klines)
	rsiSeries := RSI(closeSeries, 14)
	ema20Series := EMA(closeSeries, 20)
	ema60Series := EMA(closeSeries, 60)
	ema200Series := EMA(closeSeries, 200)
	macdLine, macdSignal, macdHist := MACD(closeSeries)
	atrSeries := ATR(klines, 14)
	bbUpper, bbMiddle, bbLower := Bollinger(closeSeries, 20)
	supports, resistances := SupportResistance(klines, 3)

	lastIdx := len(klines) - 1
	lastClose := closeSeries[lastIdx]
	lastRSI := rsiSeries[lastIdx]
	lastATR := atrSeries[lastIdx]
	lastVolume := volumeSeries[lastIdx]
	volumeMA20 := SMA(volumeSeries, 20)[lastIdx]

	fundingScore := 0
	lastFunding := 0.0
	if len(funding) > 0 {
		lastFunding = funding[0].FundingRate
		switch {
		case math.Abs(lastFunding) >= 0.0005:
			fundingScore = 20
		case math.Abs(lastFunding) >= 0.0002:
			fundingScore = 10
		}
	}

	oiScore := 0
	oiChange := 0.0
	if len(oi) >= 2 && oi[1].OpenInterest != 0 {
		oiChange = ((oi[0].OpenInterest / oi[1].OpenInterest) - 1) * 100
		switch {
		case math.Abs(oiChange) >= 8:
			oiScore = 20
		case math.Abs(oiChange) >= 4:
			oiScore = 10
		}
	}

	rsiScore := 0
	switch {
	case lastRSI >= 80:
		rsiScore = 25
	case lastRSI <= 20 && lastRSI > 0:
		rsiScore = 20
	default:
		rsiScore = int(math.Min(math.Abs(lastRSI-50)*0.6, 20))
	}

	volatilityScore := 0
	if lastClose != 0 {
		atrRatio := (lastATR / lastClose) * 100
		switch {
		case atrRatio >= 5:
			volatilityScore = 20
		case atrRatio >= 2.5:
			volatilityScore = 10
		}
	}

	volumeScore := 0
	if volumeMA20 > 0 {
		volumeRatio := lastVolume / volumeMA20
		switch {
		case volumeRatio >= 3:
			volumeScore = 15
		case volumeRatio >= 2:
			volumeScore = 8
		}
	}

	newsScore := 0
	for _, item := range news {
		switch item.Sentiment {
		case "negative":
			newsScore += 5
		case "positive":
			newsScore -= 2
		}
	}
	if newsScore < 0 {
		newsScore = 0
	}
	if newsScore > 10 {
		newsScore = 10
	}

	totalScore := rsiScore + fundingScore + oiScore + volatilityScore + volumeScore + newsScore
	if totalScore > 100 {
		totalScore = 100
	}

	riskLevel := riskLevelFromScore(totalScore)
	trend := "range"
	switch {
	case ema20Series[lastIdx] > ema60Series[lastIdx] && closeSeries[lastIdx] > ema20Series[lastIdx]:
		trend = "bullish"
	case ema20Series[lastIdx] < ema60Series[lastIdx] && closeSeries[lastIdx] < ema20Series[lastIdx]:
		trend = "bearish"
	}

	regime := "balanced"
	switch {
	case volatilityScore >= 20 && volumeScore >= 8:
		regime = "volatile_expansion"
	case volatilityScore >= 10:
		regime = "trend_transition"
	case math.Abs(macdHist[lastIdx]) < lastClose*0.001:
		regime = "compression"
	}

	summary := fmt.Sprintf(
		"RSI %.1f, EMA20 %.2f, EMA60 %.2f, MACD Hist %.4f, funding %.4f%%, OI change %.2f%%.",
		lastRSI, ema20Series[lastIdx], ema60Series[lastIdx], macdHist[lastIdx], lastFunding*100, oiChange,
	)

	analysis := domain.Analysis{
		Symbol:           symbol,
		Interval:         interval,
		TS:               klines[lastIdx].CloseTime,
		Trend:            trend,
		MarketRegime:     regime,
		RiskLevel:        riskLevel,
		RiskScore:        totalScore,
		RSI14:            ptr(lastRSI, len(klines) > 14),
		MACD:             ptr(macdLine[lastIdx], len(klines) > 26),
		MACDSignal:       ptr(macdSignal[lastIdx], len(klines) > 35),
		MACDHist:         ptr(macdHist[lastIdx], len(klines) > 35),
		EMA20:            ptr(ema20Series[lastIdx], len(klines) >= 20),
		EMA60:            ptr(ema60Series[lastIdx], len(klines) >= 60),
		EMA200:           ptr(ema200Series[lastIdx], len(klines) >= 200),
		ATR14:            ptr(lastATR, len(klines) >= 14),
		BBUpper:          ptr(bbUpper[lastIdx], len(klines) >= 20),
		BBMiddle:         ptr(bbMiddle[lastIdx], len(klines) >= 20),
		BBLower:          ptr(bbLower[lastIdx], len(klines) >= 20),
		SupportLevels:    supports,
		ResistanceLevels: resistances,
		Summary:          summary,
	}

	risk := domain.Risk{
		Symbol:          symbol,
		TS:              klines[lastIdx].CloseTime,
		TotalScore:      totalScore,
		RiskLevel:       riskLevel,
		RSIScore:        rsiScore,
		FundingScore:    fundingScore,
		OIScore:         oiScore,
		VolatilityScore: volatilityScore,
		VolumeScore:     volumeScore,
		NewsScore:       newsScore,
		Explanation: map[string]any{
			"rsi":        lastRSI,
			"funding":    lastFunding,
			"oi_change":  oiChange,
			"atr_ratio":  safePercent(lastATR, lastClose),
			"volume_ma":  volumeMA20,
			"updated_at": klines[lastIdx].CloseTime.Format(time.RFC3339),
		},
	}

	return analysis, risk
}

func safePercent(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return (numerator / denominator) * 100
}

func riskLevelFromScore(score int) string {
	switch {
	case score >= 70:
		return "high"
	case score >= 40:
		return "medium"
	default:
		return "low"
	}
}
