package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"NovaQuant/internal/config"
	"NovaQuant/internal/domain"
	"NovaQuant/internal/metrics"
	"NovaQuant/internal/repository"
)

type MarketProvider interface {
	Name() string
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

func (p *BinanceProvider) Name() string {
	return "binance"
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

type OKXProvider struct {
	client  *http.Client
	baseURL string
}

type BitgetProvider struct {
	client  *http.Client
	baseURL string
}

func NewOKXProvider(client *http.Client, baseURL string) *OKXProvider {
	return &OKXProvider{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

func (p *OKXProvider) Name() string {
	return "okx"
}

func NewBitgetProvider(client *http.Client, baseURL string) *BitgetProvider {
	return &BitgetProvider{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

func (p *BitgetProvider) Name() string {
	return "bitget"
}

func NewMarketProviders(cfg config.Config, client *http.Client) ([]MarketProvider, error) {
	providers := make([]MarketProvider, 0, len(cfg.MarketDataProviders))
	for _, name := range cfg.MarketDataProviders {
		provider, err := newMarketProvider(name, cfg, client)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	if len(providers) == 0 {
		return nil, errors.New("no market data providers configured")
	}
	return providers, nil
}

func NewMarketProvider(cfg config.Config, client *http.Client) (MarketProvider, error) {
	providers, err := NewMarketProviders(cfg, client)
	if err != nil {
		return nil, err
	}
	return providers[0], nil
}

func newMarketProvider(name string, cfg config.Config, client *http.Client) (MarketProvider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "okx":
		return NewOKXProvider(client, cfg.OKXBaseURL), nil
	case "binance":
		return NewBinanceProvider(client, cfg.BinanceSpotBaseURL, cfg.BinanceFuturesURL), nil
	case "bitget":
		return NewBitgetProvider(client, cfg.BitgetBaseURL), nil
	default:
		return nil, fmt.Errorf("unsupported market data provider %q", name)
	}
}

func (p *OKXProvider) GetPrice(ctx context.Context, symbol string) (float64, time.Time, error) {
	endpoint := fmt.Sprintf("%s/api/v5/market/ticker?instId=%s", p.baseURL, url.QueryEscape(okxSpotInstID(symbol)))
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Last string `json:"last"`
			Ts   string `json:"ts"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, endpoint, &resp); err != nil {
		return 0, time.Time{}, err
	}
	if len(resp.Data) == 0 {
		return 0, time.Time{}, errors.New("okx ticker returned no data")
	}
	price, _ := strconv.ParseFloat(resp.Data[0].Last, 64)
	ts, _ := strconv.ParseInt(resp.Data[0].Ts, 10, 64)
	return price, time.UnixMilli(ts).UTC(), nil
}

func (p *OKXProvider) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]domain.Kline, error) {
	endpoint := fmt.Sprintf("%s/api/v5/market/candles?instId=%s&bar=%s&limit=%d", p.baseURL, url.QueryEscape(okxSpotInstID(symbol)), url.QueryEscape(okxBar(interval)), limit)
	var resp struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}
	if err := p.getJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}
	result := make([]domain.Kline, 0, len(resp.Data))
	for i := len(resp.Data) - 1; i >= 0; i-- {
		row := resp.Data[i]
		if len(row) < 8 {
			continue
		}
		ts, _ := strconv.ParseInt(row[0], 10, 64)
		openTime := time.UnixMilli(ts).UTC()
		barDur := intervalDuration(interval)
		result = append(result, domain.Kline{
			Symbol:      symbol,
			Interval:    interval,
			OpenTime:    openTime,
			CloseTime:   openTime.Add(barDur),
			Open:        parseAnyFloat(row[1]),
			High:        parseAnyFloat(row[2]),
			Low:         parseAnyFloat(row[3]),
			Close:       parseAnyFloat(row[4]),
			Volume:      parseAnyFloat(row[5]),
			QuoteVolume: parseAnyFloat(row[6]),
			Trades:      0,
		})
	}
	return result, nil
}

func (p *OKXProvider) GetFundingRates(ctx context.Context, symbol string, limit int) ([]domain.FundingRate, error) {
	endpoint := fmt.Sprintf("%s/api/v5/public/funding-rate-history?instId=%s&limit=%d", p.baseURL, url.QueryEscape(okxSwapInstID(symbol)), limit)
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			FundingRate string `json:"fundingRate"`
			FundingTime string `json:"fundingTime"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}
	result := make([]domain.FundingRate, 0, len(resp.Data))
	for _, row := range resp.Data {
		value, _ := strconv.ParseFloat(row.FundingRate, 64)
		ts, _ := strconv.ParseInt(row.FundingTime, 10, 64)
		result = append(result, domain.FundingRate{
			Symbol:      symbol,
			FundingRate: value,
			FundingTime: time.UnixMilli(ts).UTC(),
		})
	}
	return result, nil
}

func (p *OKXProvider) GetOpenInterest(ctx context.Context, symbol string) (domain.OpenInterest, error) {
	endpoint := fmt.Sprintf("%s/api/v5/public/open-interest?instType=SWAP&instId=%s", p.baseURL, url.QueryEscape(okxSwapInstID(symbol)))
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Oi string `json:"oi"`
			Ts string `json:"ts"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, endpoint, &resp); err != nil {
		return domain.OpenInterest{}, err
	}
	if len(resp.Data) == 0 {
		return domain.OpenInterest{}, errors.New("okx open interest returned no data")
	}
	value, _ := strconv.ParseFloat(resp.Data[0].Oi, 64)
	ts, _ := strconv.ParseInt(resp.Data[0].Ts, 10, 64)
	return domain.OpenInterest{Symbol: symbol, OpenInterest: value, TS: time.UnixMilli(ts).UTC()}, nil
}

func (p *OKXProvider) getJSON(ctx context.Context, endpoint string, out any) error {
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

func (p *BitgetProvider) GetPrice(ctx context.Context, symbol string) (float64, time.Time, error) {
	endpoint := fmt.Sprintf("%s/api/v3/market/tickers?category=SPOT&symbol=%s", p.baseURL, url.QueryEscape(symbol))
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			LastPrice string `json:"lastPrice"`
			TS        string `json:"ts"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, endpoint, &resp); err != nil {
		return 0, time.Time{}, err
	}
	if len(resp.Data) == 0 {
		return 0, time.Time{}, errors.New("bitget ticker returned no data")
	}
	price, _ := strconv.ParseFloat(resp.Data[0].LastPrice, 64)
	ts, _ := strconv.ParseInt(resp.Data[0].TS, 10, 64)
	return price, time.UnixMilli(ts).UTC(), nil
}

func (p *BitgetProvider) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]domain.Kline, error) {
	if limit > 100 {
		limit = 100
	}
	endpoint := fmt.Sprintf("%s/api/v3/market/candles?category=SPOT&symbol=%s&interval=%s&type=market&limit=%d", p.baseURL, url.QueryEscape(symbol), url.QueryEscape(bitgetInterval(interval)), limit)
	var resp struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}
	if err := p.getJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}
	result := make([]domain.Kline, 0, len(resp.Data))
	for i := len(resp.Data) - 1; i >= 0; i-- {
		row := resp.Data[i]
		if len(row) < 7 {
			continue
		}
		ts, _ := strconv.ParseInt(row[0], 10, 64)
		openTime := time.UnixMilli(ts).UTC()
		barDur := intervalDuration(interval)
		result = append(result, domain.Kline{
			Symbol:      symbol,
			Interval:    interval,
			OpenTime:    openTime,
			CloseTime:   openTime.Add(barDur),
			Open:        parseAnyFloat(row[1]),
			High:        parseAnyFloat(row[2]),
			Low:         parseAnyFloat(row[3]),
			Close:       parseAnyFloat(row[4]),
			Volume:      parseAnyFloat(row[5]),
			QuoteVolume: parseAnyFloat(row[6]),
		})
	}
	return result, nil
}

func (p *BitgetProvider) GetFundingRates(ctx context.Context, symbol string, limit int) ([]domain.FundingRate, error) {
	endpoint := fmt.Sprintf("%s/api/v3/market/current-fund-rate?symbol=%s", p.baseURL, url.QueryEscape(symbol))
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			FundingRate string `json:"fundingRate"`
			NextUpdate  string `json:"nextUpdate"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}
	result := make([]domain.FundingRate, 0, minInt(limit, len(resp.Data)))
	for i, row := range resp.Data {
		if i >= limit {
			break
		}
		value, _ := strconv.ParseFloat(row.FundingRate, 64)
		ts, _ := strconv.ParseInt(row.NextUpdate, 10, 64)
		result = append(result, domain.FundingRate{
			Symbol:      symbol,
			FundingRate: value,
			FundingTime: time.UnixMilli(ts).UTC(),
		})
	}
	if len(result) == 0 {
		return nil, errors.New("bitget funding rate returned no data")
	}
	return result, nil
}

func (p *BitgetProvider) GetOpenInterest(ctx context.Context, symbol string) (domain.OpenInterest, error) {
	endpoint := fmt.Sprintf("%s/api/v3/market/open-interest?category=USDT-FUTURES&symbol=%s", p.baseURL, url.QueryEscape(symbol))
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			List []struct {
				Symbol       string `json:"symbol"`
				OpenInterest string `json:"openInterest"`
			} `json:"list"`
			TS string `json:"ts"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, endpoint, &resp); err != nil {
		return domain.OpenInterest{}, err
	}
	if len(resp.Data.List) == 0 {
		return domain.OpenInterest{}, errors.New("bitget open interest returned no data")
	}
	value, _ := strconv.ParseFloat(resp.Data.List[0].OpenInterest, 64)
	ts, _ := strconv.ParseInt(resp.Data.TS, 10, 64)
	return domain.OpenInterest{Symbol: symbol, OpenInterest: value, TS: time.UnixMilli(ts).UTC()}, nil
}

func (p *BitgetProvider) getJSON(ctx context.Context, endpoint string, out any) error {
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
	repo      *repository.Repository
	providers []MarketProvider
	metrics   *metrics.Registry
}

func NewCollector(repo *repository.Repository, providers []MarketProvider, registry *metrics.Registry) *Collector {
	return &Collector{repo: repo, providers: providers, metrics: registry}
}

func (c *Collector) RunOnce(ctx context.Context, symbols, intervals []string) error {
	for _, symbol := range symbols {
		price, ts, spotSource, err := c.fetchPrice(ctx, symbol)
		if err != nil {
			return err
		}
		if err := c.repo.UpsertPrice(ctx, symbol, price, ts, spotSource); err != nil {
			return err
		}
		if c.metrics != nil {
			c.metrics.Inc("collector_http_requests_total")
			c.metrics.Inc("collector_rows_written_total")
		}
		for _, interval := range intervals {
			klines, klineSource, err := c.fetchKlines(ctx, symbol, interval, 200)
			if err != nil {
				return err
			}
			if err := c.repo.UpsertKlines(ctx, klines, klineSource); err != nil {
				return err
			}
			if c.metrics != nil {
				c.metrics.Add("collector_rows_written_total", uint64(len(klines)))
			}
		}
		funding, fundingSource, err := c.fetchFundingRates(ctx, symbol, 16)
		if err != nil {
			return err
		}
		if err := c.repo.UpsertFunding(ctx, funding, fundingSource+"_swap"); err != nil {
			return err
		}
		oi, oiSource, err := c.fetchOpenInterest(ctx, symbol)
		if err != nil {
			return err
		}
		if err := c.repo.UpsertOpenInterest(ctx, oi, oiSource+"_swap"); err != nil {
			return err
		}
	}
	return nil
}

func (c *Collector) fetchPrice(ctx context.Context, symbol string) (float64, time.Time, string, error) {
	var providerErrors []string
	for _, provider := range c.providers {
		price, ts, err := provider.GetPrice(ctx, symbol)
		if err == nil {
			return price, ts, provider.Name(), nil
		}
		providerErrors = append(providerErrors, provider.Name()+": "+err.Error())
	}
	return 0, time.Time{}, "", fmt.Errorf("all providers failed for %s price: %s", symbol, strings.Join(providerErrors, "; "))
}

func (c *Collector) fetchKlines(ctx context.Context, symbol, interval string, limit int) ([]domain.Kline, string, error) {
	var providerErrors []string
	for _, provider := range c.providers {
		klines, err := provider.GetKlines(ctx, symbol, interval, limit)
		if err == nil {
			return klines, provider.Name(), nil
		}
		providerErrors = append(providerErrors, provider.Name()+": "+err.Error())
	}
	return nil, "", fmt.Errorf("all providers failed for %s %s klines: %s", symbol, interval, strings.Join(providerErrors, "; "))
}

func (c *Collector) fetchFundingRates(ctx context.Context, symbol string, limit int) ([]domain.FundingRate, string, error) {
	var providerErrors []string
	for _, provider := range c.providers {
		rows, err := provider.GetFundingRates(ctx, symbol, limit)
		if err == nil {
			return rows, provider.Name(), nil
		}
		providerErrors = append(providerErrors, provider.Name()+": "+err.Error())
	}
	return nil, "", fmt.Errorf("all providers failed for %s funding: %s", symbol, strings.Join(providerErrors, "; "))
}

func (c *Collector) fetchOpenInterest(ctx context.Context, symbol string) (domain.OpenInterest, string, error) {
	var providerErrors []string
	for _, provider := range c.providers {
		row, err := provider.GetOpenInterest(ctx, symbol)
		if err == nil {
			return row, provider.Name(), nil
		}
		providerErrors = append(providerErrors, provider.Name()+": "+err.Error())
	}
	return domain.OpenInterest{}, "", fmt.Errorf("all providers failed for %s open interest: %s", symbol, strings.Join(providerErrors, "; "))
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

func okxSpotInstID(symbol string) string {
	return strings.TrimSuffix(symbol, "USDT") + "-USDT"
}

func okxSwapInstID(symbol string) string {
	return strings.TrimSuffix(symbol, "USDT") + "-USDT-SWAP"
}

func okxBar(interval string) string {
	switch interval {
	case "1m":
		return "1m"
	case "5m":
		return "5m"
	case "15m":
		return "15m"
	case "1h":
		return "1H"
	case "4h":
		return "4H"
	case "1d":
		return "1Dutc"
	default:
		return "1H"
	}
}

func intervalDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return time.Hour
	}
}

func bitgetInterval(interval string) string {
	switch interval {
	case "1m":
		return "1m"
	case "5m":
		return "5m"
	case "15m":
		return "15m"
	case "1h":
		return "1H"
	case "4h":
		return "4H"
	case "1d":
		return "1D"
	default:
		return "1H"
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
