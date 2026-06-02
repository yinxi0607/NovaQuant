package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"NovaQuant/internal/domain"
	"NovaQuant/internal/metrics"
	"NovaQuant/internal/repository"
)

type MarketProvider interface {
	GetPrice(ctx context.Context, symbol string) (float64, time.Time, error)
	GetKlines(ctx context.Context, symbol, interval string, limit int) ([]domain.Kline, error)
	GetFundingRates(ctx context.Context, symbol string, limit int) ([]domain.FundingRate, error)
	GetOpenInterest(ctx context.Context, symbol string) (domain.OpenInterest, error)
}

type BinanceProvider struct {
	client         *http.Client
	spotBaseURL    string
	futuresBaseURL string
}

func NewBinanceProvider(client *http.Client, spotBaseURL, futuresBaseURL string) *BinanceProvider {
	return &BinanceProvider{client: client, spotBaseURL: strings.TrimRight(spotBaseURL, "/"), futuresBaseURL: strings.TrimRight(futuresBaseURL, "/")}
}

func (p *BinanceProvider) GetPrice(ctx context.Context, symbol string) (float64, time.Time, error) {
	endpoint := p.spotBaseURL + "/api/v3/ticker/price?symbol=" + url.QueryEscape(symbol)
	var resp struct {
		Price string `json:"price"`
	}
	if err := p.getJSON(ctx, endpoint, &resp); err != nil {
		return 0, time.Time{}, err
	}
	price, err := strconv.ParseFloat(resp.Price, 64)
	return price, time.Now().UTC(), err
}

func (p *BinanceProvider) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]domain.Kline, error) {
	endpoint := fmt.Sprintf("%s/api/v3/klines?symbol=%s&interval=%s&limit=%d", p.spotBaseURL, url.QueryEscape(symbol), url.QueryEscape(interval), limit)
	var rows [][]any
	if err := p.getJSON(ctx, endpoint, &rows); err != nil {
		return nil, err
	}
	result := make([]domain.Kline, 0, len(rows))
	for _, row := range rows {
		if len(row) < 9 {
			continue
		}
		openTime := parseMillis(row[0])
		closeTime := parseMillis(row[6])
		result = append(result, domain.Kline{
			Symbol:      symbol,
			Interval:    interval,
			OpenTime:    openTime,
			CloseTime:   closeTime,
			Open:        parseAnyFloat(row[1]),
			High:        parseAnyFloat(row[2]),
			Low:         parseAnyFloat(row[3]),
			Close:       parseAnyFloat(row[4]),
			Volume:      parseAnyFloat(row[5]),
			QuoteVolume: parseAnyFloat(row[7]),
			Trades:      int64(parseAnyFloat(row[8])),
		})
	}
	return result, nil
}

func (p *BinanceProvider) GetFundingRates(ctx context.Context, symbol string, limit int) ([]domain.FundingRate, error) {
	endpoint := fmt.Sprintf("%s/fapi/v1/fundingRate?symbol=%s&limit=%d", p.futuresBaseURL, url.QueryEscape(symbol), limit)
	var rows []struct {
		FundingRate string `json:"fundingRate"`
		FundingTime int64  `json:"fundingTime"`
		Symbol      string `json:"symbol"`
	}
	if err := p.getJSON(ctx, endpoint, &rows); err != nil {
		return nil, err
	}
	result := make([]domain.FundingRate, 0, len(rows))
	for _, row := range rows {
		value, _ := strconv.ParseFloat(row.FundingRate, 64)
		result = append(result, domain.FundingRate{
			Symbol:      row.Symbol,
			FundingRate: value,
			FundingTime: time.UnixMilli(row.FundingTime).UTC(),
		})
	}
	return result, nil
}

func (p *BinanceProvider) GetOpenInterest(ctx context.Context, symbol string) (domain.OpenInterest, error) {
	endpoint := fmt.Sprintf("%s/fapi/v1/openInterest?symbol=%s", p.futuresBaseURL, url.QueryEscape(symbol))
	var resp struct {
		Symbol       string `json:"symbol"`
		OpenInterest string `json:"openInterest"`
		Time         int64  `json:"time"`
	}
	if err := p.getJSON(ctx, endpoint, &resp); err != nil {
		return domain.OpenInterest{}, err
	}
	value, _ := strconv.ParseFloat(resp.OpenInterest, 64)
	return domain.OpenInterest{Symbol: resp.Symbol, OpenInterest: value, TS: time.UnixMilli(resp.Time).UTC()}, nil
}

func (p *BinanceProvider) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("upstream %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type Collector struct {
	repo     *repository.Repository
	provider MarketProvider
	metrics  *metrics.Registry
}

func NewCollector(repo *repository.Repository, provider MarketProvider, registry *metrics.Registry) *Collector {
	return &Collector{repo: repo, provider: provider, metrics: registry}
}

func (c *Collector) RunOnce(ctx context.Context, symbols, intervals []string) error {
	for _, symbol := range symbols {
		price, ts, err := c.provider.GetPrice(ctx, symbol)
		if err != nil {
			return err
		}
		if err := c.repo.UpsertPrice(ctx, symbol, price, ts, "binance"); err != nil {
			return err
		}
		if c.metrics != nil {
			c.metrics.Inc("collector_http_requests_total")
			c.metrics.Inc("collector_rows_written_total")
		}
		for _, interval := range intervals {
			klines, err := c.provider.GetKlines(ctx, symbol, interval, 200)
			if err != nil {
				return err
			}
			if err := c.repo.UpsertKlines(ctx, klines, "binance"); err != nil {
				return err
			}
			if c.metrics != nil {
				c.metrics.Add("collector_rows_written_total", uint64(len(klines)))
			}
		}
		funding, err := c.provider.GetFundingRates(ctx, symbol, 16)
		if err != nil {
			return err
		}
		if err := c.repo.UpsertFunding(ctx, funding, "binance_futures"); err != nil {
			return err
		}
		oi, err := c.provider.GetOpenInterest(ctx, symbol)
		if err != nil {
			return err
		}
		if err := c.repo.UpsertOpenInterest(ctx, oi, "binance_futures"); err != nil {
			return err
		}
	}
	return nil
}

func parseMillis(value any) time.Time {
	switch v := value.(type) {
	case float64:
		return time.UnixMilli(int64(v)).UTC()
	case int64:
		return time.UnixMilli(v).UTC()
	default:
		return time.Now().UTC()
	}
}

func parseAnyFloat(value any) float64 {
	switch v := value.(type) {
	case string:
		out, _ := strconv.ParseFloat(v, 64)
		return out
	case float64:
		return v
	case int64:
		return float64(v)
	default:
		return 0
	}
}
