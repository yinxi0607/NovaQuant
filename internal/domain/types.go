package domain

import "time"

type Symbol struct {
	Symbol     string    `json:"symbol"`
	BaseAsset  string    `json:"base_asset"`
	QuoteAsset string    `json:"quote_asset"`
	MarketType string    `json:"market_type"`
	Enabled    bool      `json:"enabled"`
	SortOrder  int       `json:"sort_order"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type OverviewCard struct {
	Symbol      string    `json:"symbol"`
	Price       float64   `json:"price"`
	Change24H   float64   `json:"change_24h"`
	Trend       string    `json:"trend"`
	RiskLevel   string    `json:"risk_level"`
	RiskScore   int       `json:"risk_score"`
	UpdatedAt   time.Time `json:"updated_at"`
	MarketPhase string    `json:"market_phase"`
}

type MarketSnapshot struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	Change24H     float64   `json:"change_24h"`
	FundingRate   float64   `json:"funding_rate"`
	OpenInterest  float64   `json:"open_interest"`
	Trend         string    `json:"trend"`
	RiskLevel     string    `json:"risk_level"`
	RiskScore     int       `json:"risk_score"`
	UpdatedAt     time.Time `json:"updated_at"`
	AnalysisTS    time.Time `json:"analysis_timestamp"`
	Support       []float64 `json:"support"`
	Resistance    []float64 `json:"resistance"`
	Summary       string    `json:"summary"`
	MarketRegime  string    `json:"market_regime"`
	LatestKlineTS time.Time `json:"latest_kline_timestamp"`
}

type Kline struct {
	Symbol      string    `json:"symbol"`
	Interval    string    `json:"interval"`
	OpenTime    time.Time `json:"open_time"`
	CloseTime   time.Time `json:"close_time"`
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
	Volume      float64   `json:"volume"`
	QuoteVolume float64   `json:"quote_volume"`
	Trades      int64     `json:"trades"`
}

type FundingRate struct {
	Symbol      string    `json:"symbol"`
	FundingRate float64   `json:"funding_rate"`
	FundingTime time.Time `json:"funding_time"`
}

type OpenInterest struct {
	Symbol       string    `json:"symbol"`
	OpenInterest float64   `json:"open_interest"`
	TS           time.Time `json:"ts"`
}

type Analysis struct {
	Symbol           string    `json:"symbol"`
	Interval         string    `json:"interval"`
	TS               time.Time `json:"ts"`
	Trend            string    `json:"trend"`
	MarketRegime     string    `json:"market_regime"`
	RiskLevel        string    `json:"risk_level"`
	RiskScore        int       `json:"risk_score"`
	RSI14            *float64  `json:"rsi14"`
	MACD             *float64  `json:"macd"`
	MACDSignal       *float64  `json:"macd_signal"`
	MACDHist         *float64  `json:"macd_hist"`
	EMA20            *float64  `json:"ema20"`
	EMA60            *float64  `json:"ema60"`
	EMA200           *float64  `json:"ema200"`
	ATR14            *float64  `json:"atr14"`
	BBUpper          *float64  `json:"bb_upper"`
	BBMiddle         *float64  `json:"bb_middle"`
	BBLower          *float64  `json:"bb_lower"`
	SupportLevels    []float64 `json:"support_levels"`
	ResistanceLevels []float64 `json:"resistance_levels"`
	Summary          string    `json:"summary"`
}

type Risk struct {
	Symbol          string         `json:"symbol"`
	TS              time.Time      `json:"ts"`
	TotalScore      int            `json:"total_score"`
	RiskLevel       string         `json:"risk_level"`
	RSIScore        int            `json:"rsi_score"`
	FundingScore    int            `json:"funding_score"`
	OIScore         int            `json:"oi_score"`
	VolatilityScore int            `json:"volatility_score"`
	VolumeScore     int            `json:"volume_score"`
	NewsScore       int            `json:"news_score"`
	Explanation     map[string]any `json:"explanation"`
}

type NewsItem struct {
	ID             string    `json:"id"`
	Source         string    `json:"source"`
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	PublishedAt    time.Time `json:"published_at"`
	RelatedSymbols []string  `json:"related_symbols"`
	Sentiment      string    `json:"sentiment"`
	SentimentScore float64   `json:"sentiment_score"`
	Summary        string    `json:"summary"`
}

type ETFFlow struct {
	ID             string    `json:"id"`
	Asset          string    `json:"asset"`
	Provider       string    `json:"provider"`
	FlowDate       time.Time `json:"flow_date"`
	NetFlowUSD     float64   `json:"net_flow_usd"`
	TotalVolumeUSD float64   `json:"total_volume_usd"`
	Note           string    `json:"note"`
}

type WhaleTransaction struct {
	ID        string    `json:"id"`
	Chain     string    `json:"chain"`
	Asset     string    `json:"asset"`
	Amount    float64   `json:"amount"`
	AmountUSD float64   `json:"amount_usd"`
	FromLabel string    `json:"from_label"`
	ToLabel   string    `json:"to_label"`
	TxHash    string    `json:"tx_hash"`
	Direction string    `json:"direction"`
	TS        time.Time `json:"ts"`
	Source    string    `json:"source"`
}

type AlertEvent struct {
	ID          string         `json:"id"`
	RuleID      *string        `json:"rule_id,omitempty"`
	Symbol      string         `json:"symbol"`
	Severity    string         `json:"severity"`
	Message     string         `json:"message"`
	Payload     map[string]any `json:"payload"`
	TriggeredAt time.Time      `json:"triggered_at"`
}

type AlertRule struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Symbol          string    `json:"symbol"`
	RuleType        string    `json:"rule_type"`
	Operator        string    `json:"operator"`
	Threshold       float64   `json:"threshold"`
	Enabled         bool      `json:"enabled"`
	CooldownSeconds int       `json:"cooldown_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}

type AgentEvidence struct {
	Type      string    `json:"type"`
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Summary   string    `json:"summary"`
}

type AgentChatRequest struct {
	SessionID   string   `json:"session_id"`
	Question    string   `json:"question"`
	Symbols     []string `json:"symbols"`
	TimeHorizon string   `json:"time_horizon"`
}

type AgentChatResponse struct {
	SessionID     string          `json:"session_id"`
	Answer        string          `json:"answer"`
	Symbols       []string        `json:"symbols"`
	Trend         string          `json:"trend"`
	RiskLevel     string          `json:"risk_level"`
	Support       []float64       `json:"support"`
	Resistance    []float64       `json:"resistance"`
	Evidence      []AgentEvidence `json:"evidence"`
	DataTimestamp time.Time       `json:"data_timestamp"`
	Disclaimer    string          `json:"disclaimer"`
}

type AuthConfigResponse struct {
	Enabled   bool   `json:"enabled"`
	Username  string `json:"username"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"public_key"`
}

type AuthLoginRequest struct {
	Username          string `json:"username"`
	Password          string `json:"password,omitempty"`
	EncryptedPassword string `json:"encrypted_password,omitempty"`
}

type AuthSession struct {
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

type AuthLoginResponse struct {
	Token     string      `json:"token"`
	Session   AuthSession `json:"session"`
	ExpiresAt time.Time   `json:"expires_at"`
}

type BacktestRequest struct {
	Strategy  string         `json:"strategy"`
	Symbol    string         `json:"symbol"`
	Interval  string         `json:"interval"`
	StartTime time.Time      `json:"start_time"`
	EndTime   time.Time      `json:"end_time"`
	Params    map[string]any `json:"params"`
}

type BacktestTrade struct {
	Side       string    `json:"side"`
	EntryTime  time.Time `json:"entry_time"`
	ExitTime   time.Time `json:"exit_time"`
	EntryPrice float64   `json:"entry_price"`
	ExitPrice  float64   `json:"exit_price"`
	Return     float64   `json:"return"`
}

type EquityPoint struct {
	Time   time.Time `json:"time"`
	Equity float64   `json:"equity"`
}

type BacktestResult struct {
	TotalReturn  float64         `json:"total_return"`
	WinRate      float64         `json:"win_rate"`
	ProfitFactor float64         `json:"profit_factor"`
	SharpeRatio  float64         `json:"sharpe_ratio"`
	MaxDrawdown  float64         `json:"max_drawdown"`
	TradeCount   int             `json:"trade_count"`
	Trades       []BacktestTrade `json:"trades"`
	EquityCurve  []EquityPoint   `json:"equity_curve"`
}

type BacktestResponse struct {
	JobID   string         `json:"job_id"`
	Result  BacktestResult `json:"result"`
	Status  string         `json:"status"`
	Created time.Time      `json:"created_at"`
}
