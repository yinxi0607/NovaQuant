package service

import (
	"math"
	"sort"

	"NovaQuant/internal/domain"
)

func EMA(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 || period <= 0 {
		return out
	}
	alpha := 2.0 / float64(period+1)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}

func SMA(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if period <= 0 {
		return out
	}
	var sum float64
	for i := range values {
		sum += values[i]
		if i >= period {
			sum -= values[i-period]
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		}
	}
	return out
}

func RSI(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) <= period || period <= 0 {
		return out
	}
	var gainSum float64
	var lossSum float64
	for i := 1; i <= period; i++ {
		diff := values[i] - values[i-1]
		if diff >= 0 {
			gainSum += diff
		} else {
			lossSum += -diff
		}
	}
	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)
	if avgLoss == 0 {
		out[period] = 100
	} else {
		rs := avgGain / avgLoss
		out[period] = 100 - (100 / (1 + rs))
	}

	for i := period + 1; i < len(values); i++ {
		diff := values[i] - values[i-1]
		gain := math.Max(diff, 0)
		loss := math.Max(-diff, 0)
		avgGain = ((avgGain * float64(period-1)) + gain) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + loss) / float64(period)
		if avgLoss == 0 {
			out[i] = 100
			continue
		}
		rs := avgGain / avgLoss
		out[i] = 100 - (100 / (1 + rs))
	}
	return out
}

func MACD(values []float64) (line, signal, hist []float64) {
	ema12 := EMA(values, 12)
	ema26 := EMA(values, 26)
	line = make([]float64, len(values))
	for i := range values {
		line[i] = ema12[i] - ema26[i]
	}
	signal = EMA(line, 9)
	hist = make([]float64, len(values))
	for i := range line {
		hist[i] = line[i] - signal[i]
	}
	return line, signal, hist
}

func ATR(klines []domain.Kline, period int) []float64 {
	out := make([]float64, len(klines))
	if len(klines) == 0 || period <= 0 {
		return out
	}
	tr := make([]float64, len(klines))
	for i, item := range klines {
		if i == 0 {
			tr[i] = item.High - item.Low
			continue
		}
		prevClose := klines[i-1].Close
		tr[i] = max(item.High-item.Low, math.Abs(item.High-prevClose), math.Abs(item.Low-prevClose))
	}
	var sum float64
	for i := 0; i < len(tr); i++ {
		sum += tr[i]
		if i == period-1 {
			out[i] = sum / float64(period)
		}
		if i >= period {
			out[i] = ((out[i-1] * float64(period-1)) + tr[i]) / float64(period)
		}
	}
	return out
}

func Bollinger(values []float64, period int) (upper, middle, lower []float64) {
	middle = SMA(values, period)
	upper = make([]float64, len(values))
	lower = make([]float64, len(values))
	for i := range values {
		if i < period-1 {
			continue
		}
		var variance float64
		for j := i - period + 1; j <= i; j++ {
			diff := values[j] - middle[i]
			variance += diff * diff
		}
		std := math.Sqrt(variance / float64(period))
		upper[i] = middle[i] + 2*std
		lower[i] = middle[i] - 2*std
	}
	return upper, middle, lower
}

func SupportResistance(klines []domain.Kline, window int) (supports, resistances []float64) {
	if len(klines) < window*2+1 {
		return []float64{}, []float64{}
	}
	var lows []float64
	var highs []float64
	for i := window; i < len(klines)-window; i++ {
		low := klines[i].Low
		high := klines[i].High
		isLow := true
		isHigh := true
		for j := i - window; j <= i+window; j++ {
			if klines[j].Low < low {
				isLow = false
			}
			if klines[j].High > high {
				isHigh = false
			}
		}
		if isLow {
			lows = append(lows, low)
		}
		if isHigh {
			highs = append(highs, high)
		}
	}
	return dedupeLevels(lows), dedupeLevels(highs)
}

func dedupeLevels(values []float64) []float64 {
	if len(values) == 0 {
		return []float64{}
	}
	sort.Float64s(values)
	out := []float64{values[0]}
	for _, value := range values[1:] {
		last := out[len(out)-1]
		if math.Abs(value-last)/math.Max(last, 1) > 0.01 {
			out = append(out, value)
		}
		if len(out) == 5 {
			break
		}
	}
	return out
}

func ptr(value float64, ok bool) *float64 {
	if !ok {
		return nil
	}
	v := value
	return &v
}

func closes(klines []domain.Kline) []float64 {
	result := make([]float64, len(klines))
	for i, item := range klines {
		result[i] = item.Close
	}
	return result
}

func volumes(klines []domain.Kline) []float64 {
	result := make([]float64, len(klines))
	for i, item := range klines {
		result[i] = item.Volume
	}
	return result
}

func max(values ...float64) float64 {
	best := values[0]
	for _, value := range values[1:] {
		if value > best {
			best = value
		}
	}
	return best
}
