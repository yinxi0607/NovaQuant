package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"NovaQuant/internal/config"
	"NovaQuant/internal/domain"
)

type ETFCollector interface {
	GetSummaryHistory(ctx context.Context, asset string) ([]domain.ETFFlow, string, error)
}

type SoSoETFCollector struct {
	client      *http.Client
	baseURL     string
	apiKey      string
	countryCode string
}

func NewETFCollector(cfg config.Config, client *http.Client) ETFCollector {
	if strings.TrimSpace(cfg.SoSoAPIKey) == "" {
		return nil
	}
	return &SoSoETFCollector{
		client:      client,
		baseURL:     strings.TrimRight(cfg.SoSoBaseURL, "/"),
		apiKey:      strings.TrimSpace(cfg.SoSoAPIKey),
		countryCode: strings.ToUpper(strings.TrimSpace(cfg.ETFCountryCode)),
	}
}

func (c *SoSoETFCollector) GetSummaryHistory(ctx context.Context, asset string) ([]domain.ETFFlow, string, error) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if asset != "BTC" && asset != "ETH" {
		return nil, "", fmt.Errorf("unsupported ETF asset %q", asset)
	}
	endpoint := fmt.Sprintf("%s/openapi/v1/etfs/summary-history?symbol=%s&country_code=%s", c.baseURL, url.QueryEscape(asset), url.QueryEscape(c.countryCode))
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    []struct {
			Date             string  `json:"date"`
			TotalNetInflow   float64 `json:"total_net_inflow"`
			TotalValueTraded float64 `json:"total_value_traded"`
			TotalNetAssets   float64 `json:"total_net_assets"`
			CumNetInflow     float64 `json:"cum_net_inflow"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, endpoint, &resp); err != nil {
		return nil, "", err
	}
	if resp.Code != 0 {
		return nil, "", fmt.Errorf("soso api code %d: %s", resp.Code, resp.Message)
	}
	provider := c.providerName()
	seenDates := map[string]bool{}
	rows := make([]domain.ETFFlow, 0, len(resp.Data))
	for _, item := range resp.Data {
		flowDate, err := time.Parse("2006-01-02", item.Date)
		if err != nil {
			continue
		}
		dateKey := flowDate.Format("2006-01-02")
		if seenDates[dateKey] {
			continue
		}
		seenDates[dateKey] = true
		rows = append(rows, domain.ETFFlow{
			Asset:          asset,
			Provider:       provider,
			FlowDate:       flowDate.UTC(),
			NetFlowUSD:     item.TotalNetInflow,
			TotalVolumeUSD: item.TotalValueTraded,
			Note:           fmt.Sprintf("country=%s total_net_assets=%.2f cum_net_inflow=%.2f", c.countryCode, item.TotalNetAssets, item.CumNetInflow),
		})
	}
	if len(rows) == 0 {
		return nil, "", errors.New("soso etf summary history returned no usable rows")
	}
	return rows, provider, nil
}

func (c *SoSoETFCollector) providerName() string {
	return "soso_" + strings.ToLower(c.countryCode)
}

func (c *SoSoETFCollector) getJSON(ctx context.Context, endpoint string, out any) error {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			if err := sleepWithContext(ctx, retryBackoff(attempt)); err != nil {
				if lastErr != nil {
					return lastErr
				}
				return err
			}
		}
		if err := c.getJSONOnce(ctx, endpoint, out); err != nil {
			lastErr = err
			if !isRetryableUpstreamError(err) || attempt == 3 {
				return err
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (c *SoSoETFCollector) getJSONOnce(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-soso-api-key", c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		if isRetryableTransportError(err) {
			return &retryableUpstreamError{err: err}
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		upstreamErr := fmt.Errorf("upstream %s: %s", resp.Status, strings.TrimSpace(string(body)))
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			return &retryableUpstreamError{err: upstreamErr}
		}
		return upstreamErr
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		if isRetryableDecodeError(err) {
			return &retryableUpstreamError{err: err}
		}
		return err
	}
	return nil
}
