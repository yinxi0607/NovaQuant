package service

import (
	"context"
	"errors"
	"math"

	"NovaQuant/internal/domain"
	"NovaQuant/internal/repository"
)

type Backtester struct {
	repo *repository.Repository
}

func NewBacktester(repo *repository.Repository) *Backtester {
	return &Backtester{repo: repo}
}

func (b *Backtester) Run(ctx context.Context, req domain.BacktestRequest) (domain.BacktestResponse, error) {
	klines, err := b.repo.ListKlinesBetween(ctx, req.Symbol, req.Interval, req.StartTime, req.EndTime)
	if err != nil {
		return domain.BacktestResponse{}, err
	}
	if len(klines) < 30 {
		return domain.BacktestResponse{}, errors.New("not enough kline data for backtest")
	}

	result := RunBacktest(req.Strategy, klines)
	jobID, created, err := b.repo.CreateBacktestJob(ctx, req)
	if err != nil {
		return domain.BacktestResponse{}, err
	}
	if err := b.repo.StoreBacktestResult(ctx, jobID, result); err != nil {
		return domain.BacktestResponse{}, err
	}
	return domain.BacktestResponse{
		JobID:   jobID,
		Result:  result,
		Status:  "completed",
		Created: created,
	}, nil
}

func RunBacktest(strategy string, klines []domain.Kline) domain.BacktestResult {
	switch strategy {
	case "rsi":
		return backtestRSI(klines)
	case "macd":
		return backtestMACD(klines)
	default:
		return backtestEMACrossover(klines)
	}
}

func backtestRSI(klines []domain.Kline) domain.BacktestResult {
	rsi := RSI(closes(klines), 14)
	var signals []int
	for i := range klines {
		switch {
		case rsi[i] > 0 && rsi[i] < 30:
			signals = append(signals, 1)
		case rsi[i] > 70:
			signals = append(signals, -1)
		default:
			signals = append(signals, 0)
		}
	}
	return simulateLongOnly(klines, signals)
}

func backtestEMACrossover(klines []domain.Kline) domain.BacktestResult {
	closeSeries := closes(klines)
	emaFast := EMA(closeSeries, 20)
	emaSlow := EMA(closeSeries, 60)
	signals := make([]int, len(klines))
	for i := 1; i < len(klines); i++ {
		if emaFast[i] > emaSlow[i] && emaFast[i-1] <= emaSlow[i-1] {
			signals[i] = 1
		}
		if emaFast[i] < emaSlow[i] && emaFast[i-1] >= emaSlow[i-1] {
			signals[i] = -1
		}
	}
	return simulateLongOnly(klines, signals)
}

func backtestMACD(klines []domain.Kline) domain.BacktestResult {
	_, _, hist := MACD(closes(klines))
	signals := make([]int, len(klines))
	for i := 1; i < len(klines); i++ {
		if hist[i] > 0 && hist[i-1] <= 0 {
			signals[i] = 1
		}
		if hist[i] < 0 && hist[i-1] >= 0 {
			signals[i] = -1
		}
	}
	return simulateLongOnly(klines, signals)
}

func simulateLongOnly(klines []domain.Kline, signals []int) domain.BacktestResult {
	equity := 1.0
	peak := equity
	maxDrawdown := 0.0
	positionOpen := false
	entryPrice := 0.0
	entryTime := klines[0].OpenTime
	trades := make([]domain.BacktestTrade, 0)
	curve := make([]domain.EquityPoint, 0, len(klines))
	wins := 0
	grossProfit := 0.0
	grossLoss := 0.0

	for i, item := range klines {
		if !positionOpen && signals[i] == 1 {
			positionOpen = true
			entryPrice = item.Close
			entryTime = item.CloseTime
		}

		if positionOpen && (signals[i] == -1 || i == len(klines)-1) {
			ret := (item.Close / entryPrice) - 1
			equity *= 1 + ret
			trade := domain.BacktestTrade{
				Side:       "long",
				EntryTime:  entryTime,
				ExitTime:   item.CloseTime,
				EntryPrice: entryPrice,
				ExitPrice:  item.Close,
				Return:     ret * 100,
			}
			trades = append(trades, trade)
			if ret > 0 {
				wins++
				grossProfit += ret
			} else {
				grossLoss += -ret
			}
			positionOpen = false
		}

		curve = append(curve, domain.EquityPoint{Time: item.CloseTime, Equity: equity})
		if equity > peak {
			peak = equity
		}
		drawdown := (peak - equity) / peak
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	winRate := 0.0
	if len(trades) > 0 {
		winRate = float64(wins) / float64(len(trades)) * 100
	}
	profitFactor := grossProfit
	if grossLoss > 0 {
		profitFactor = grossProfit / grossLoss
	}

	return domain.BacktestResult{
		TotalReturn:  (equity - 1) * 100,
		WinRate:      winRate,
		ProfitFactor: profitFactor,
		SharpeRatio:  estimateSharpe(curve),
		MaxDrawdown:  maxDrawdown * 100,
		TradeCount:   len(trades),
		Trades:       trades,
		EquityCurve:  curve,
	}
}

func estimateSharpe(curve []domain.EquityPoint) float64 {
	if len(curve) < 3 {
		return 0
	}
	returns := make([]float64, 0, len(curve)-1)
	for i := 1; i < len(curve); i++ {
		prev := curve[i-1].Equity
		if prev == 0 {
			continue
		}
		returns = append(returns, (curve[i].Equity/prev)-1)
	}
	if len(returns) == 0 {
		return 0
	}
	var mean float64
	for _, ret := range returns {
		mean += ret
	}
	mean /= float64(len(returns))
	var variance float64
	for _, ret := range returns {
		diff := ret - mean
		variance += diff * diff
	}
	std := math.Sqrt(variance / float64(len(returns)))
	if std == 0 {
		return 0
	}
	return mean / std * math.Sqrt(252)
}
