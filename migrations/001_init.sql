DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS timescaledb;
EXCEPTION
    WHEN undefined_file OR feature_not_supported OR invalid_parameter_value THEN
        RAISE NOTICE 'timescaledb extension not installed, skipping extension creation';
END $$;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS symbols (
    symbol              varchar(32) PRIMARY KEY,
    base_asset          varchar(32) NOT NULL,
    quote_asset         varchar(32) NOT NULL DEFAULT 'USDT',
    market_type         varchar(16) NOT NULL DEFAULT 'spot',
    enabled             boolean NOT NULL DEFAULT true,
    sort_order          int NOT NULL DEFAULT 100,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS market_price (
    symbol              varchar(32) NOT NULL REFERENCES symbols(symbol),
    price               numeric(30,10) NOT NULL,
    source              varchar(32) NOT NULL DEFAULT 'binance',
    ts                  timestamptz NOT NULL,
    ingested_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(symbol, ts)
);

CREATE TABLE IF NOT EXISTS market_kline (
    symbol              varchar(32) NOT NULL REFERENCES symbols(symbol),
    interval            varchar(8) NOT NULL,
    open_time           timestamptz NOT NULL,
    close_time          timestamptz NOT NULL,
    open                numeric(30,10) NOT NULL,
    high                numeric(30,10) NOT NULL,
    low                 numeric(30,10) NOT NULL,
    close               numeric(30,10) NOT NULL,
    volume              numeric(30,10) NOT NULL,
    quote_volume        numeric(30,10),
    trades              bigint,
    source              varchar(32) NOT NULL DEFAULT 'binance',
    ingested_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(symbol, interval, open_time)
);

CREATE TABLE IF NOT EXISTS market_funding (
    symbol              varchar(32) NOT NULL REFERENCES symbols(symbol),
    funding_rate        numeric(20,10) NOT NULL,
    funding_time        timestamptz NOT NULL,
    source              varchar(32) NOT NULL DEFAULT 'binance_futures',
    ingested_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(symbol, funding_time)
);

CREATE TABLE IF NOT EXISTS market_open_interest (
    symbol              varchar(32) NOT NULL REFERENCES symbols(symbol),
    open_interest       numeric(30,10) NOT NULL,
    source              varchar(32) NOT NULL DEFAULT 'binance_futures',
    ts                  timestamptz NOT NULL,
    ingested_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(symbol, ts)
);

CREATE TABLE IF NOT EXISTS market_analysis (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol              varchar(32) NOT NULL REFERENCES symbols(symbol),
    interval            varchar(8) NOT NULL,
    ts                  timestamptz NOT NULL,
    trend               varchar(24) NOT NULL,
    market_regime       varchar(32) NOT NULL,
    risk_level          varchar(16) NOT NULL,
    risk_score          int NOT NULL CHECK (risk_score BETWEEN 0 AND 100),
    rsi14               numeric(10,4),
    macd                numeric(30,10),
    macd_signal         numeric(30,10),
    macd_hist           numeric(30,10),
    ema20               numeric(30,10),
    ema60               numeric(30,10),
    ema200              numeric(30,10),
    atr14               numeric(30,10),
    bb_upper            numeric(30,10),
    bb_middle           numeric(30,10),
    bb_lower            numeric(30,10),
    support_levels      jsonb NOT NULL DEFAULT '[]',
    resistance_levels   jsonb NOT NULL DEFAULT '[]',
    summary             text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE(symbol, interval, ts)
);

CREATE TABLE IF NOT EXISTS market_risk (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol              varchar(32) NOT NULL REFERENCES symbols(symbol),
    ts                  timestamptz NOT NULL,
    total_score         int NOT NULL CHECK (total_score BETWEEN 0 AND 100),
    rsi_score           int NOT NULL DEFAULT 0,
    funding_score       int NOT NULL DEFAULT 0,
    oi_score            int NOT NULL DEFAULT 0,
    volatility_score    int NOT NULL DEFAULT 0,
    volume_score        int NOT NULL DEFAULT 0,
    news_score          int NOT NULL DEFAULT 0,
    explanation         jsonb NOT NULL DEFAULT '{}',
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE(symbol, ts)
);

CREATE TABLE IF NOT EXISTS news_items (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source              varchar(64) NOT NULL,
    title               text NOT NULL,
    url                 text NOT NULL UNIQUE,
    published_at        timestamptz NOT NULL,
    related_symbols     text[] NOT NULL DEFAULT '{}',
    sentiment           varchar(16) NOT NULL DEFAULT 'neutral',
    sentiment_score     numeric(8,4) NOT NULL DEFAULT 0,
    summary             text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS etf_flows (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset               varchar(16) NOT NULL,
    provider            varchar(64) NOT NULL DEFAULT 'manual',
    flow_date           date NOT NULL,
    net_flow_usd        numeric(30,2) NOT NULL,
    total_volume_usd    numeric(30,2),
    note                text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE(asset, provider, flow_date)
);

CREATE TABLE IF NOT EXISTS whale_transactions (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    chain               varchar(32) NOT NULL,
    asset               varchar(32) NOT NULL,
    amount              numeric(30,10) NOT NULL,
    amount_usd          numeric(30,2),
    from_label          varchar(128),
    to_label            varchar(128),
    tx_hash             varchar(128) UNIQUE,
    direction           varchar(32) NOT NULL DEFAULT 'unknown',
    ts                  timestamptz NOT NULL,
    source              varchar(64) NOT NULL DEFAULT 'mock',
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS alert_rules (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                varchar(128) NOT NULL,
    symbol              varchar(32),
    rule_type           varchar(32) NOT NULL,
    operator            varchar(8) NOT NULL,
    threshold           numeric(30,10) NOT NULL,
    enabled             boolean NOT NULL DEFAULT true,
    cooldown_seconds    int NOT NULL DEFAULT 1800,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS alert_events (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id             uuid REFERENCES alert_rules(id),
    symbol              varchar(32),
    severity            varchar(16) NOT NULL DEFAULT 'info',
    message             text NOT NULL,
    payload             jsonb NOT NULL DEFAULT '{}',
    triggered_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS agent_sessions (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title               varchar(256) NOT NULL DEFAULT 'New Research Session',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS agent_messages (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id          uuid NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    role                varchar(16) NOT NULL,
    content             text NOT NULL,
    evidence            jsonb NOT NULL DEFAULT '[]',
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS backtest_jobs (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    strategy            varchar(64) NOT NULL,
    symbol              varchar(32) NOT NULL,
    interval            varchar(8) NOT NULL,
    start_time          timestamptz NOT NULL,
    end_time            timestamptz NOT NULL,
    params              jsonb NOT NULL DEFAULT '{}',
    status              varchar(16) NOT NULL DEFAULT 'pending',
    created_at          timestamptz NOT NULL DEFAULT now(),
    finished_at         timestamptz
);

CREATE TABLE IF NOT EXISTS backtest_results (
    job_id              uuid PRIMARY KEY REFERENCES backtest_jobs(id) ON DELETE CASCADE,
    total_return        numeric(20,8) NOT NULL,
    win_rate            numeric(10,4) NOT NULL,
    profit_factor       numeric(20,8),
    sharpe_ratio        numeric(20,8),
    max_drawdown        numeric(20,8) NOT NULL,
    trade_count         int NOT NULL,
    trades              jsonb NOT NULL DEFAULT '[]',
    equity_curve        jsonb NOT NULL DEFAULT '[]',
    created_at          timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN
        PERFORM create_hypertable('market_price', 'ts', if_not_exists => TRUE);
        PERFORM create_hypertable('market_kline', 'open_time', if_not_exists => TRUE);
        PERFORM create_hypertable('market_funding', 'funding_time', if_not_exists => TRUE);
        PERFORM create_hypertable('market_open_interest', 'ts', if_not_exists => TRUE);
        PERFORM create_hypertable('market_analysis', 'ts', if_not_exists => TRUE);
        PERFORM create_hypertable('market_risk', 'ts', if_not_exists => TRUE);
        PERFORM create_hypertable('whale_transactions', 'ts', if_not_exists => TRUE);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_kline_symbol_interval_time ON market_kline(symbol, interval, open_time DESC);
CREATE INDEX IF NOT EXISTS idx_analysis_symbol_interval_time ON market_analysis(symbol, interval, ts DESC);
CREATE INDEX IF NOT EXISTS idx_price_symbol_time ON market_price(symbol, ts DESC);
CREATE INDEX IF NOT EXISTS idx_news_published ON news_items(published_at DESC);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN
        EXECUTE $view$
            CREATE MATERIALIZED VIEW IF NOT EXISTS market_price_1m
            WITH (timescaledb.continuous) AS
            SELECT symbol,
                   time_bucket('1 minute', ts) AS bucket,
                   first(price, ts) AS open,
                   max(price) AS high,
                   min(price) AS low,
                   last(price, ts) AS close,
                   count(*) AS samples
            FROM market_price
            GROUP BY symbol, bucket
            WITH NO DATA
        $view$;
        PERFORM add_continuous_aggregate_policy(
            'market_price_1m',
            start_offset => INTERVAL '7 days',
            end_offset => INTERVAL '1 minute',
            schedule_interval => INTERVAL '1 minute',
            if_not_exists => TRUE
        );
    END IF;
END $$;
