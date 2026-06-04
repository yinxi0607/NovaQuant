package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"NovaQuant/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repository) ListSymbols(ctx context.Context) ([]domain.Symbol, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT symbol, base_asset, quote_asset, market_type, enabled, sort_order, updated_at
		FROM symbols
		WHERE enabled = true
		ORDER BY sort_order, symbol
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Symbol
	for rows.Next() {
		var item domain.Symbol
		if err := rows.Scan(&item.Symbol, &item.BaseAsset, &item.QuoteAsset, &item.MarketType, &item.Enabled, &item.SortOrder, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListEnabledSymbols(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT symbol FROM symbols WHERE enabled = true ORDER BY sort_order, symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		result = append(result, symbol)
	}
	return result, rows.Err()
}

func (r *Repository) GetMarketOverview(ctx context.Context, symbols []string) ([]domain.OverviewCard, error) {
	query := `
		SELECT s.symbol,
		       COALESCE(lp.price, lk.close, 0)::double precision AS price,
		       COALESCE(((COALESCE(lp.price, lk.close, 0) / NULLIF(prev.close, 0)) - 1) * 100, 0)::double precision AS change_24h,
		       COALESCE(a.trend, 'unknown') AS trend,
		       COALESCE(a.risk_level, 'unknown') AS risk_level,
		       COALESCE(r.total_score, 0) AS risk_score,
		       COALESCE(a.ts, lk.close_time, now()) AS updated_at,
		       COALESCE(a.market_regime, 'unknown') AS market_phase
		FROM symbols s
			LEFT JOIN LATERAL (
			    SELECT price::double precision AS price, ts
			    FROM market_price
			    WHERE symbol = s.symbol AND ts <= now()
			    ORDER BY ts DESC
			    LIMIT 1
			) lp ON true
			LEFT JOIN LATERAL (
			    SELECT close::double precision AS close, close_time
			    FROM market_kline
			    WHERE symbol = s.symbol AND interval = '1h' AND close_time <= now()
			    ORDER BY close_time DESC
			    LIMIT 1
			) lk ON true
		LEFT JOIN LATERAL (
		    SELECT close::double precision AS close
		    FROM market_kline
		    WHERE symbol = s.symbol AND interval = '1h' AND close_time <= now() - interval '24 hours'
		    ORDER BY close_time DESC
		    LIMIT 1
		) prev ON true
			LEFT JOIN LATERAL (
			    SELECT trend, market_regime, risk_level, ts
			    FROM market_analysis
			    WHERE symbol = s.symbol AND interval = '1h' AND ts <= now()
			    ORDER BY ts DESC
			    LIMIT 1
			) a ON true
			LEFT JOIN LATERAL (
			    SELECT total_score
			    FROM market_risk
			    WHERE symbol = s.symbol AND ts <= now()
			    ORDER BY ts DESC
			    LIMIT 1
			) r ON true
		WHERE s.enabled = true
	`
	args := []any{}
	if len(symbols) > 0 {
		query += ` AND s.symbol = ANY($1)`
		args = append(args, symbols)
	}
	query += ` ORDER BY s.sort_order, s.symbol`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.OverviewCard
	for rows.Next() {
		var item domain.OverviewCard
		if err := rows.Scan(&item.Symbol, &item.Price, &item.Change24H, &item.Trend, &item.RiskLevel, &item.RiskScore, &item.UpdatedAt, &item.MarketPhase); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) GetMarketSnapshot(ctx context.Context, symbol string) (domain.MarketSnapshot, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT s.symbol,
		       COALESCE(lp.price, lk.close, 0)::double precision AS price,
		       COALESCE(((COALESCE(lp.price, lk.close, 0) / NULLIF(prev.close, 0)) - 1) * 100, 0)::double precision AS change_24h,
		       COALESCE(f.funding_rate, 0)::double precision AS funding_rate,
		       COALESCE(oi.open_interest, 0)::double precision AS open_interest,
		       COALESCE(a.trend, 'unknown') AS trend,
		       COALESCE(a.risk_level, 'unknown') AS risk_level,
		       COALESCE(r.total_score, 0) AS risk_score,
		       COALESCE(lp.ts, lk.close_time, now()) AS updated_at,
		       COALESCE(a.ts, lk.close_time, now()) AS analysis_ts,
		       COALESCE(a.support_levels, '[]'::jsonb),
		       COALESCE(a.resistance_levels, '[]'::jsonb),
		       COALESCE(a.summary, ''),
		       COALESCE(a.market_regime, 'unknown'),
		       COALESCE(lk.close_time, now()) AS latest_kline_ts
		FROM symbols s
			LEFT JOIN LATERAL (
			    SELECT price::double precision AS price, ts
			    FROM market_price
			    WHERE symbol = s.symbol AND ts <= now()
			    ORDER BY ts DESC
			    LIMIT 1
			) lp ON true
			LEFT JOIN LATERAL (
			    SELECT close::double precision AS close, close_time
			    FROM market_kline
			    WHERE symbol = s.symbol AND interval = '1h' AND close_time <= now()
			    ORDER BY close_time DESC
			    LIMIT 1
			) lk ON true
		LEFT JOIN LATERAL (
		    SELECT close::double precision AS close
		    FROM market_kline
		    WHERE symbol = s.symbol AND interval = '1h' AND close_time <= now() - interval '24 hours'
		    ORDER BY close_time DESC
		    LIMIT 1
		) prev ON true
			LEFT JOIN LATERAL (
			    SELECT funding_rate::double precision AS funding_rate
			    FROM market_funding
			    WHERE symbol = s.symbol AND funding_time <= now()
			    ORDER BY funding_time DESC
			    LIMIT 1
			) f ON true
			LEFT JOIN LATERAL (
			    SELECT open_interest::double precision AS open_interest
			    FROM market_open_interest
			    WHERE symbol = s.symbol AND ts <= now()
			    ORDER BY ts DESC
			    LIMIT 1
			) oi ON true
			LEFT JOIN LATERAL (
			    SELECT trend, market_regime, risk_level, ts, support_levels, resistance_levels, summary
			    FROM market_analysis
			    WHERE symbol = s.symbol AND interval = '1h' AND ts <= now()
			    ORDER BY ts DESC
			    LIMIT 1
			) a ON true
			LEFT JOIN LATERAL (
			    SELECT total_score
			    FROM market_risk
			    WHERE symbol = s.symbol AND ts <= now()
			    ORDER BY ts DESC
			    LIMIT 1
			) r ON true
		WHERE s.symbol = $1
	`, symbol)

	var snapshot domain.MarketSnapshot
	var supportRaw []byte
	var resistanceRaw []byte
	if err := row.Scan(
		&snapshot.Symbol,
		&snapshot.Price,
		&snapshot.Change24H,
		&snapshot.FundingRate,
		&snapshot.OpenInterest,
		&snapshot.Trend,
		&snapshot.RiskLevel,
		&snapshot.RiskScore,
		&snapshot.UpdatedAt,
		&snapshot.AnalysisTS,
		&supportRaw,
		&resistanceRaw,
		&snapshot.Summary,
		&snapshot.MarketRegime,
		&snapshot.LatestKlineTS,
	); err != nil {
		return snapshot, err
	}
	snapshot.Support = decodeFloatSlice(supportRaw)
	snapshot.Resistance = decodeFloatSlice(resistanceRaw)
	return snapshot, nil
}

func (r *Repository) ListKlines(ctx context.Context, symbol, interval string, limit int) ([]domain.Kline, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT symbol, interval, open_time, close_time,
		       open::double precision, high::double precision, low::double precision, close::double precision,
		       volume::double precision, COALESCE(quote_volume, 0)::double precision, COALESCE(trades, 0)
		FROM (
		    SELECT *
		    FROM market_kline
		    WHERE symbol = $1 AND interval = $2 AND open_time <= now()
		    ORDER BY open_time DESC
		    LIMIT $3
		) q
		ORDER BY open_time ASC
	`, symbol, interval, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanKlines(rows)
}

func (r *Repository) ListKlinesBetween(ctx context.Context, symbol, interval string, start, end time.Time) ([]domain.Kline, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT symbol, interval, open_time, close_time,
		       open::double precision, high::double precision, low::double precision, close::double precision,
		       volume::double precision, COALESCE(quote_volume, 0)::double precision, COALESCE(trades, 0)
		FROM market_kline
		WHERE symbol = $1 AND interval = $2 AND open_time >= $3 AND close_time <= $4 AND open_time <= now()
		ORDER BY open_time ASC
	`, symbol, interval, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanKlines(rows)
}

func (r *Repository) ListRecentFunding(ctx context.Context, symbol string, limit int) ([]domain.FundingRate, error) {
	rows, err := r.pool.Query(ctx, `
			SELECT symbol, funding_rate::double precision, funding_time
			FROM market_funding
			WHERE symbol = $1 AND funding_time <= now()
			ORDER BY funding_time DESC
			LIMIT $2
		`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.FundingRate
	for rows.Next() {
		var item domain.FundingRate
		if err := rows.Scan(&item.Symbol, &item.FundingRate, &item.FundingTime); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListRecentOpenInterest(ctx context.Context, symbol string, limit int) ([]domain.OpenInterest, error) {
	rows, err := r.pool.Query(ctx, `
			SELECT symbol, open_interest::double precision, ts
			FROM market_open_interest
			WHERE symbol = $1 AND ts <= now()
			ORDER BY ts DESC
			LIMIT $2
		`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.OpenInterest
	for rows.Next() {
		var item domain.OpenInterest
		if err := rows.Scan(&item.Symbol, &item.OpenInterest, &item.TS); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) GetLatestAnalysis(ctx context.Context, symbol, interval string) (domain.Analysis, error) {
	row := r.pool.QueryRow(ctx, `
			SELECT symbol, interval, ts, trend, market_regime, risk_level, risk_score,
			       rsi14::double precision, macd::double precision, macd_signal::double precision, macd_hist::double precision,
			       ema20::double precision, ema60::double precision, ema200::double precision, atr14::double precision,
			       bb_upper::double precision, bb_middle::double precision, bb_lower::double precision,
			       support_levels, resistance_levels, summary
			FROM market_analysis
			WHERE symbol = $1 AND interval = $2 AND ts <= now()
			ORDER BY ts DESC
			LIMIT 1
		`, symbol, interval)

	return scanAnalysisRow(row)
}

func (r *Repository) GetLatestRisk(ctx context.Context, symbol string) (domain.Risk, error) {
	row := r.pool.QueryRow(ctx, `
			SELECT symbol, ts, total_score, rsi_score, funding_score, oi_score, volatility_score, volume_score, news_score, explanation
			FROM market_risk
			WHERE symbol = $1 AND ts <= now()
			ORDER BY ts DESC
			LIMIT 1
		`, symbol)

	var item domain.Risk
	var explanationRaw []byte
	if err := row.Scan(
		&item.Symbol,
		&item.TS,
		&item.TotalScore,
		&item.RSIScore,
		&item.FundingScore,
		&item.OIScore,
		&item.VolatilityScore,
		&item.VolumeScore,
		&item.NewsScore,
		&explanationRaw,
	); err != nil {
		return item, err
	}
	item.RiskLevel = riskLevel(item.TotalScore)
	item.Explanation = decodeJSONMap(explanationRaw)
	return item, nil
}

func (r *Repository) ListNews(ctx context.Context, symbol string, limit int) ([]domain.NewsItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, source, title, url, published_at, related_symbols, sentiment, sentiment_score::double precision, summary
		FROM news_items
		WHERE ($1 = '' OR $1 = ANY(related_symbols))
		  AND source <> 'seed'
		ORDER BY published_at DESC
		LIMIT $2
	`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.NewsItem
	for rows.Next() {
		var item domain.NewsItem
		if err := rows.Scan(
			&item.ID,
			&item.Source,
			&item.Title,
			&item.URL,
			&item.PublishedAt,
			&item.RelatedSymbols,
			&item.Sentiment,
			&item.SentimentScore,
			&item.Summary,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListETFFlows(ctx context.Context, asset string, limit int) ([]domain.ETFFlow, error) {
	rows, err := r.pool.Query(ctx, `
		WITH ranked AS (
			SELECT id::text,
			       asset,
			       provider,
			       flow_date,
			       net_flow_usd::double precision AS net_flow_usd,
			       COALESCE(total_volume_usd, 0)::double precision AS total_volume_usd,
			       COALESCE(note, '') AS note,
			       ROW_NUMBER() OVER (
			           PARTITION BY asset, flow_date
			           ORDER BY created_at DESC
			       ) AS rn
			FROM etf_flows
			WHERE ($1 = '' OR asset = $1)
			  AND provider <> 'seed'
		)
		SELECT id, asset, provider, flow_date, net_flow_usd, total_volume_usd, note
		FROM ranked
		WHERE rn = 1
		ORDER BY flow_date DESC
		LIMIT $2
	`, strings.ToUpper(asset), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.ETFFlow
	for rows.Next() {
		var item domain.ETFFlow
		if err := rows.Scan(&item.ID, &item.Asset, &item.Provider, &item.FlowDate, &item.NetFlowUSD, &item.TotalVolumeUSD, &item.Note); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListWhaleTransactions(ctx context.Context, asset string, limit int) ([]domain.WhaleTransaction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, chain, asset, amount::double precision, COALESCE(amount_usd, 0)::double precision,
		       COALESCE(from_label, ''), COALESCE(to_label, ''), COALESCE(tx_hash, ''), direction, ts, source
		FROM whale_transactions
		WHERE $1 = '' OR asset = $1
		ORDER BY ts DESC
		LIMIT $2
	`, strings.ToUpper(asset), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.WhaleTransaction
	for rows.Next() {
		var item domain.WhaleTransaction
		if err := rows.Scan(
			&item.ID,
			&item.Chain,
			&item.Asset,
			&item.Amount,
			&item.AmountUSD,
			&item.FromLabel,
			&item.ToLabel,
			&item.TxHash,
			&item.Direction,
			&item.TS,
			&item.Source,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListAlerts(ctx context.Context, symbol string, limit int) ([]domain.AlertEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, rule_id::text, COALESCE(symbol, ''), severity, message, payload, triggered_at
		FROM alert_events
		WHERE $1 = '' OR symbol = $1
		ORDER BY triggered_at DESC
		LIMIT $2
	`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.AlertEvent
	for rows.Next() {
		var item domain.AlertEvent
		var ruleID *string
		var payloadRaw []byte
		if err := rows.Scan(&item.ID, &ruleID, &item.Symbol, &item.Severity, &item.Message, &payloadRaw, &item.TriggeredAt); err != nil {
			return nil, err
		}
		item.RuleID = ruleID
		item.Payload = decodeJSONMap(payloadRaw)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListAlertRules(ctx context.Context) ([]domain.AlertRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name, COALESCE(symbol, ''), rule_type, operator, threshold::double precision, enabled, cooldown_seconds, created_at
		FROM alert_rules
		WHERE enabled = true
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.AlertRule
	for rows.Next() {
		var item domain.AlertRule
		if err := rows.Scan(&item.ID, &item.Name, &item.Symbol, &item.RuleType, &item.Operator, &item.Threshold, &item.Enabled, &item.CooldownSeconds, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) LatestAlertForRule(ctx context.Context, ruleID string) (time.Time, error) {
	var triggeredAt time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT triggered_at
		FROM alert_events
		WHERE rule_id = $1::uuid
		ORDER BY triggered_at DESC
		LIMIT 1
	`, ruleID).Scan(&triggeredAt)
	if err != nil {
		return time.Time{}, err
	}
	return triggeredAt, nil
}

func (r *Repository) InsertAlertEvent(ctx context.Context, ruleID, symbol, severity, message string, payload map[string]any) error {
	raw, _ := json.Marshal(payload)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO alert_events(rule_id, symbol, severity, message, payload)
		VALUES ($1::uuid, $2, $3, $4, $5)
	`, ruleID, symbol, severity, message, raw)
	return err
}

func (r *Repository) UpsertPrice(ctx context.Context, symbol string, price float64, ts time.Time, source string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO market_price(symbol, price, ts, source)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT(symbol, ts) DO UPDATE
		SET price = EXCLUDED.price, source = EXCLUDED.source, ingested_at = now()
	`, symbol, price, ts, source)
	return err
}

func (r *Repository) UpsertKlines(ctx context.Context, klines []domain.Kline, source string) error {
	batch := &pgx.Batch{}
	for _, item := range klines {
		batch.Queue(`
			INSERT INTO market_kline(symbol, interval, open_time, close_time, open, high, low, close, volume, quote_volume, trades, source)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT(symbol, interval, open_time) DO UPDATE
			SET close_time = EXCLUDED.close_time,
			    open = EXCLUDED.open,
			    high = EXCLUDED.high,
			    low = EXCLUDED.low,
			    close = EXCLUDED.close,
			    volume = EXCLUDED.volume,
			    quote_volume = EXCLUDED.quote_volume,
			    trades = EXCLUDED.trades,
			    source = EXCLUDED.source,
			    ingested_at = now()
		`, item.Symbol, item.Interval, item.OpenTime, item.CloseTime, item.Open, item.High, item.Low, item.Close, item.Volume, item.QuoteVolume, item.Trades, source)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range klines {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpsertFunding(ctx context.Context, rows []domain.FundingRate, source string) error {
	batch := &pgx.Batch{}
	for _, item := range rows {
		batch.Queue(`
			INSERT INTO market_funding(symbol, funding_rate, funding_time, source)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT(symbol, funding_time) DO UPDATE
			SET funding_rate = EXCLUDED.funding_rate, source = EXCLUDED.source, ingested_at = now()
		`, item.Symbol, item.FundingRate, item.FundingTime, source)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range rows {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpsertOpenInterest(ctx context.Context, row domain.OpenInterest, source string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO market_open_interest(symbol, open_interest, ts, source)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT(symbol, ts) DO UPDATE
		SET open_interest = EXCLUDED.open_interest, source = EXCLUDED.source, ingested_at = now()
	`, row.Symbol, row.OpenInterest, row.TS, source)
	return err
}

func (r *Repository) UpsertAnalysis(ctx context.Context, item domain.Analysis) error {
	supportRaw, _ := json.Marshal(item.SupportLevels)
	resistanceRaw, _ := json.Marshal(item.ResistanceLevels)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO market_analysis(symbol, interval, ts, trend, market_regime, risk_level, risk_score, rsi14, macd, macd_signal, macd_hist,
		                            ema20, ema60, ema200, atr14, bb_upper, bb_middle, bb_lower, support_levels, resistance_levels, summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		ON CONFLICT(symbol, interval, ts) DO UPDATE
		SET trend = EXCLUDED.trend,
		    market_regime = EXCLUDED.market_regime,
		    risk_level = EXCLUDED.risk_level,
		    risk_score = EXCLUDED.risk_score,
		    rsi14 = EXCLUDED.rsi14,
		    macd = EXCLUDED.macd,
		    macd_signal = EXCLUDED.macd_signal,
		    macd_hist = EXCLUDED.macd_hist,
		    ema20 = EXCLUDED.ema20,
		    ema60 = EXCLUDED.ema60,
		    ema200 = EXCLUDED.ema200,
		    atr14 = EXCLUDED.atr14,
		    bb_upper = EXCLUDED.bb_upper,
		    bb_middle = EXCLUDED.bb_middle,
		    bb_lower = EXCLUDED.bb_lower,
		    support_levels = EXCLUDED.support_levels,
		    resistance_levels = EXCLUDED.resistance_levels,
		    summary = EXCLUDED.summary,
		    created_at = now()
	`, item.Symbol, item.Interval, item.TS, item.Trend, item.MarketRegime, item.RiskLevel, item.RiskScore, item.RSI14, item.MACD, item.MACDSignal, item.MACDHist,
		item.EMA20, item.EMA60, item.EMA200, item.ATR14, item.BBUpper, item.BBMiddle, item.BBLower, supportRaw, resistanceRaw, item.Summary)
	return err
}

func (r *Repository) UpsertRisk(ctx context.Context, item domain.Risk) error {
	explanationRaw, _ := json.Marshal(item.Explanation)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO market_risk(symbol, ts, total_score, rsi_score, funding_score, oi_score, volatility_score, volume_score, news_score, explanation)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT(symbol, ts) DO UPDATE
		SET total_score = EXCLUDED.total_score,
		    rsi_score = EXCLUDED.rsi_score,
		    funding_score = EXCLUDED.funding_score,
		    oi_score = EXCLUDED.oi_score,
		    volatility_score = EXCLUDED.volatility_score,
		    volume_score = EXCLUDED.volume_score,
		    news_score = EXCLUDED.news_score,
		    explanation = EXCLUDED.explanation,
		    created_at = now()
	`, item.Symbol, item.TS, item.TotalScore, item.RSIScore, item.FundingScore, item.OIScore, item.VolatilityScore, item.VolumeScore, item.NewsScore, explanationRaw)
	return err
}

func (r *Repository) UpsertETFFlows(ctx context.Context, rows []domain.ETFFlow) error {
	batch := &pgx.Batch{}
	for _, item := range rows {
		batch.Queue(`
			INSERT INTO etf_flows(asset, provider, flow_date, net_flow_usd, total_volume_usd, note)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT(asset, provider, flow_date) DO UPDATE
			SET net_flow_usd = EXCLUDED.net_flow_usd,
			    total_volume_usd = EXCLUDED.total_volume_usd,
			    note = EXCLUDED.note,
			    created_at = now()
		`, item.Asset, item.Provider, item.FlowDate, item.NetFlowUSD, item.TotalVolumeUSD, item.Note)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range rows {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpsertNewsItems(ctx context.Context, rows []domain.NewsItem) error {
	batch := &pgx.Batch{}
	for _, item := range rows {
		batch.Queue(`
			INSERT INTO news_items(source, title, url, published_at, related_symbols, sentiment, sentiment_score, summary)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT(url) DO UPDATE
			SET source = EXCLUDED.source,
			    title = EXCLUDED.title,
			    published_at = EXCLUDED.published_at,
			    related_symbols = EXCLUDED.related_symbols,
			    sentiment = EXCLUDED.sentiment,
			    sentiment_score = EXCLUDED.sentiment_score,
			    summary = EXCLUDED.summary,
			    created_at = now()
		`, item.Source, item.Title, item.URL, item.PublishedAt, item.RelatedSymbols, item.Sentiment, item.SentimentScore, item.Summary)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range rows {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) EnsureAgentSession(ctx context.Context, sessionID, title string) (string, error) {
	if sessionID == "" {
		var id string
		err := r.pool.QueryRow(ctx, `
			INSERT INTO agent_sessions(title)
			VALUES ($1)
			RETURNING id::text
		`, title).Scan(&id)
		return id, err
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO agent_sessions(id, title, updated_at)
		VALUES ($1::uuid, $2, now())
		ON CONFLICT(id) DO UPDATE SET updated_at = now()
	`, sessionID, title)
	return sessionID, err
}

func (r *Repository) StoreAgentMessage(ctx context.Context, sessionID, role, content string, evidence []domain.AgentEvidence) error {
	raw, _ := json.Marshal(evidence)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO agent_messages(session_id, role, content, evidence)
		VALUES ($1::uuid, $2, $3, $4)
	`, sessionID, role, content, raw)
	return err
}

func (r *Repository) CreateBacktestJob(ctx context.Context, req domain.BacktestRequest) (string, time.Time, error) {
	raw, _ := json.Marshal(req.Params)
	var id string
	var created time.Time
	err := r.pool.QueryRow(ctx, `
		INSERT INTO backtest_jobs(strategy, symbol, interval, start_time, end_time, params, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'completed')
		RETURNING id::text, created_at
	`, req.Strategy, req.Symbol, req.Interval, req.StartTime, req.EndTime, raw).Scan(&id, &created)
	return id, created, err
}

func (r *Repository) StoreBacktestResult(ctx context.Context, jobID string, result domain.BacktestResult) error {
	tradesRaw, _ := json.Marshal(result.Trades)
	equityRaw, _ := json.Marshal(result.EquityCurve)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO backtest_results(job_id, total_return, win_rate, profit_factor, sharpe_ratio, max_drawdown, trade_count, trades, equity_curve)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT(job_id) DO UPDATE
		SET total_return = EXCLUDED.total_return,
		    win_rate = EXCLUDED.win_rate,
		    profit_factor = EXCLUDED.profit_factor,
		    sharpe_ratio = EXCLUDED.sharpe_ratio,
		    max_drawdown = EXCLUDED.max_drawdown,
		    trade_count = EXCLUDED.trade_count,
		    trades = EXCLUDED.trades,
		    equity_curve = EXCLUDED.equity_curve,
		    created_at = now()
	`, jobID, result.TotalReturn, result.WinRate, result.ProfitFactor, result.SharpeRatio, result.MaxDrawdown, result.TradeCount, tradesRaw, equityRaw)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE backtest_jobs SET finished_at = now(), status = 'completed' WHERE id = $1::uuid`, jobID)
	return err
}

func scanKlines(rows pgx.Rows) ([]domain.Kline, error) {
	var result []domain.Kline
	for rows.Next() {
		var item domain.Kline
		if err := rows.Scan(
			&item.Symbol,
			&item.Interval,
			&item.OpenTime,
			&item.CloseTime,
			&item.Open,
			&item.High,
			&item.Low,
			&item.Close,
			&item.Volume,
			&item.QuoteVolume,
			&item.Trades,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func scanAnalysisRow(row pgx.Row) (domain.Analysis, error) {
	var item domain.Analysis
	var supportRaw []byte
	var resistanceRaw []byte
	if err := row.Scan(
		&item.Symbol,
		&item.Interval,
		&item.TS,
		&item.Trend,
		&item.MarketRegime,
		&item.RiskLevel,
		&item.RiskScore,
		&item.RSI14,
		&item.MACD,
		&item.MACDSignal,
		&item.MACDHist,
		&item.EMA20,
		&item.EMA60,
		&item.EMA200,
		&item.ATR14,
		&item.BBUpper,
		&item.BBMiddle,
		&item.BBLower,
		&supportRaw,
		&resistanceRaw,
		&item.Summary,
	); err != nil {
		return item, err
	}
	item.SupportLevels = decodeFloatSlice(supportRaw)
	item.ResistanceLevels = decodeFloatSlice(resistanceRaw)
	return item, nil
}

func decodeFloatSlice(raw []byte) []float64 {
	if len(raw) == 0 {
		return []float64{}
	}
	var values []float64
	if err := json.Unmarshal(raw, &values); err == nil {
		return values
	}
	return []float64{}
}

func decodeJSONMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	return map[string]any{}
}

func riskLevel(score int) string {
	switch {
	case score >= 70:
		return "high"
	case score >= 40:
		return "medium"
	default:
		return "low"
	}
}

func MustDecode[T any](value []byte, fallback T) T {
	var out T
	if len(value) == 0 {
		return fallback
	}
	if err := json.Unmarshal(value, &out); err != nil {
		return fallback
	}
	return out
}

func QueryErrorf(path string, err error) error {
	return fmt.Errorf("%s: %w", path, err)
}
