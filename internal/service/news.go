package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"NovaQuant/internal/config"
	"NovaQuant/internal/domain"
)

type NewsCollector interface {
	GetLatest(ctx context.Context, pageSize int) ([]domain.NewsItem, string, error)
}

type SoSoNewsCollector struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

func NewNewsCollector(cfg config.Config, client *http.Client) NewsCollector {
	if strings.TrimSpace(cfg.SoSoAPIKey) == "" {
		return nil
	}
	return &SoSoNewsCollector{
		client:  client,
		baseURL: strings.TrimRight(cfg.SoSoBaseURL, "/"),
		apiKey:  strings.TrimSpace(cfg.SoSoAPIKey),
	}
}

func (c *SoSoNewsCollector) GetLatest(ctx context.Context, pageSize int) ([]domain.NewsItem, string, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	endpoint := fmt.Sprintf("%s/openapi/v1/news?page=1&page_size=%d", c.baseURL, pageSize)
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			List []struct {
				SourceLink        string  `json:"source_link"`
				OriginalLink      string  `json:"original_link"`
				ReleaseTime       string  `json:"release_time"`
				Title             *string `json:"title"`
				Content           string  `json:"content"`
				Author            string  `json:"author"`
				NickName          *string `json:"nick_name"`
				MatchedCurrencies []struct {
					Symbol string `json:"symbol"`
					Name   string `json:"name"`
				} `json:"matched_currencies"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := getJSONWithHeaderRetry(ctx, c.client, endpoint, map[string]string{"x-soso-api-key": c.apiKey}, &resp); err != nil {
		return nil, "", err
	}
	if resp.Code != 0 {
		return nil, "", fmt.Errorf("soso api code %d: %s", resp.Code, resp.Message)
	}
	rows := make([]domain.NewsItem, 0, len(resp.Data.List))
	seenURLs := map[string]bool{}
	for _, item := range resp.Data.List {
		link := strings.TrimSpace(item.OriginalLink)
		if link == "" {
			link = strings.TrimSpace(item.SourceLink)
		}
		if link == "" || seenURLs[link] {
			continue
		}
		seenURLs[link] = true
		publishedAt := parseUnixMillisString(item.ReleaseTime)
		title := ""
		if item.Title != nil {
			title = strings.TrimSpace(*item.Title)
		}
		if title == "" {
			title = summarizeTitle(item.Content)
		}
		source := strings.TrimSpace(item.Author)
		if source == "" && item.NickName != nil {
			source = strings.TrimSpace(*item.NickName)
		}
		if source == "" {
			source = "soso"
		}
		rows = append(rows, domain.NewsItem{
			Source:         source,
			Title:          title,
			URL:            link,
			PublishedAt:    publishedAt,
			RelatedSymbols: mapMatchedSymbols(item.MatchedCurrencies),
			Sentiment:      "neutral",
			SentimentScore: 0,
			Summary:        summarizeContent(item.Content),
		})
	}
	if len(rows) == 0 {
		return nil, "", fmt.Errorf("soso news returned no usable rows")
	}
	return rows, "soso_news", nil
}

func getJSONWithHeaderRetry(ctx context.Context, client *http.Client, endpoint string, headers map[string]string, out any) error {
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
		if err := getJSONWithHeadersOnce(ctx, client, endpoint, headers, out); err != nil {
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

func getJSONWithHeadersOnce(ctx context.Context, client *http.Client, endpoint string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		if isRetryableTransportError(err) {
			return &retryableUpstreamError{err: err}
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return decodeUpstreamHTTPError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		if isRetryableDecodeError(err) {
			return &retryableUpstreamError{err: err}
		}
		return err
	}
	return nil
}

func decodeUpstreamHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	upstreamErr := fmt.Errorf("upstream %s: %s", resp.Status, strings.TrimSpace(string(body)))
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		return &retryableUpstreamError{err: upstreamErr}
	}
	return upstreamErr
}

func parseUnixMillisString(value string) time.Time {
	millis, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return time.Now().UTC()
	}
	return time.UnixMilli(millis).UTC()
}

func summarizeTitle(content string) string {
	content = strings.TrimSpace(stripHTMLBreaks(content))
	if content == "" {
		return "SoSo news item"
	}
	runes := []rune(content)
	if len(runes) > 80 {
		return strings.TrimSpace(string(runes[:80])) + "..."
	}
	return content
}

func summarizeContent(content string) string {
	content = strings.TrimSpace(stripHTMLBreaks(content))
	runes := []rune(content)
	if len(runes) > 600 {
		return strings.TrimSpace(string(runes[:600])) + "..."
	}
	return content
}

func stripHTMLBreaks(value string) string {
	replacer := strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n")
	return replacer.Replace(value)
}

func mapMatchedSymbols(items []struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		symbol := normalizeMatchedSymbol(item.Symbol)
		if symbol == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		result = append(result, symbol)
	}
	return result
}

func normalizeMatchedSymbol(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" || strings.Contains(symbol, ".") {
		return ""
	}
	switch symbol {
	case "BTC", "ETH", "SOL", "BNB", "DOGE":
		return symbol + "USDT"
	default:
		return ""
	}
}
