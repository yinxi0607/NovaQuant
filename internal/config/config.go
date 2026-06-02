package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	PGDSN               string
	RedisURL            string
	APIPort             string
	AgentPort           string
	CollectorPort       string
	AnalyzerPort        string
	AlertsPort          string
	WebPort             string
	OpenAIAPIKey        string
	CollectorEnabled    bool
	MarketDataProvider  string
	MarketDataProviders []string
	BinanceSpotBaseURL  string
	BinanceFuturesURL   string
	OKXBaseURL          string
	BitgetBaseURL       string
	DefaultSymbols      []string
	CollectionIntervals []string
	NewsRSSURLs         []string
	HTTPTimeout         time.Duration
	ServiceTick         time.Duration
}

func Load() Config {
	return Config{
		PGDSN:               os.Getenv("PG_DSN"),
		RedisURL:            getenv("REDIS_URL", ""),
		APIPort:             getenv("API_PORT", "8080"),
		AgentPort:           getenv("AGENT_PORT", "8090"),
		CollectorPort:       getenv("COLLECTOR_PORT", "8091"),
		AnalyzerPort:        getenv("ANALYZER_PORT", "8092"),
		AlertsPort:          getenv("ALERTS_PORT", "8093"),
		WebPort:             getenv("WEB_PORT", "5173"),
		OpenAIAPIKey:        os.Getenv("OPENAI_API_KEY"),
		CollectorEnabled:    parseBool(getenv("COLLECTOR_ENABLED", "false"), false),
		MarketDataProvider:  strings.ToLower(getenv("MARKET_DATA_PROVIDER", "okx")),
		MarketDataProviders: marketDataProviders(),
		BinanceSpotBaseURL:  getenv("BINANCE_SPOT_BASE_URL", "https://api.binance.com"),
		BinanceFuturesURL:   getenv("BINANCE_FUTURES_BASE_URL", "https://fapi.binance.com"),
		OKXBaseURL:          getenv("OKX_BASE_URL", "https://www.okx.com"),
		BitgetBaseURL:       getenv("BITGET_BASE_URL", "https://api.bitget.com"),
		DefaultSymbols:      splitCSV(getenv("DEFAULT_SYMBOLS", "BTCUSDT,ETHUSDT,SOLUSDT,BNBUSDT,DOGEUSDT")),
		CollectionIntervals: splitCSV(getenv("COLLECTION_INTERVALS", "1m,5m,15m,1h,4h,1d")),
		NewsRSSURLs:         splitCSV(getenv("NEWS_RSS_URLS", "")),
		HTTPTimeout:         parseDuration(getenv("HTTP_TIMEOUT", "10s"), 10*time.Second),
		ServiceTick:         parseDuration(getenv("SERVICE_TICK", "1m"), time.Minute),
	}
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func marketDataProviders() []string {
	providers := splitCSV(strings.ToLower(getenv("MARKET_DATA_PROVIDERS", "")))
	if len(providers) > 0 {
		return providers
	}
	legacy := strings.ToLower(strings.TrimSpace(getenv("MARKET_DATA_PROVIDER", "okx")))
	if legacy == "" {
		return []string{"okx"}
	}
	return []string{legacy}
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}

func ParseInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func parseBool(value string, fallback bool) bool {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
