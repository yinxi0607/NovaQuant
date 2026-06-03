import { useEffect, useMemo, useState } from "react";
import { EChart } from "../components/EChart";
import { MetricGrid } from "../components/MetricGrid";
import { Panel } from "../components/Panel";
import {
  AgentResponse,
  Analysis,
  ETFFlow,
  Kline,
  NewsItem,
  OverviewCard,
  Snapshot,
  apiGet,
  apiPost,
} from "../lib/api";

type TrendWindow = {
  label: string;
  change: number;
  trend: string;
  regime: string;
  summary: string;
};

type AssetView = {
  symbol: string;
  snapshot: Snapshot;
  day: TrendWindow;
  week: TrendWindow;
  month: TrendWindow;
  ai: AgentResponse;
};

type EtfSummary = {
  asset: string;
  latest: number;
  weekly: number;
  monthly: number;
  rows: ETFFlow[];
};

const focusSymbols = ["BTCUSDT", "ETHUSDT"] as const;

export function OverviewPage() {
  const [cards, setCards] = useState<OverviewCard[]>([]);
  const [news, setNews] = useState<NewsItem[]>([]);
  const [assetViews, setAssetViews] = useState<AssetView[]>([]);
  const [monthlySeries, setMonthlySeries] = useState<Record<string, Kline[]>>({});
  const [etfSummaries, setEtfSummaries] = useState<EtfSummary[]>([]);
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    async function load() {
      const errors: string[] = [];
      const [overviewResult, newsResult, btcEtfResult, ethEtfResult] = await Promise.allSettled([
        apiGet<{ cards: OverviewCard[] }>("/market/overview"),
        apiGet<{ news: NewsItem[] }>("/news?page_size=4"),
        apiGet<{ flows: ETFFlow[] }>("/etf?asset=BTC&page_size=30"),
        apiGet<{ flows: ETFFlow[] }>("/etf?asset=ETH&page_size=30"),
      ]);

      if (overviewResult.status === "fulfilled") {
        setCards(overviewResult.value.cards.filter((card) => focusSymbols.includes(card.symbol as (typeof focusSymbols)[number])));
      } else {
        errors.push("market overview unavailable");
      }
      if (newsResult.status === "fulfilled") {
        setNews(newsResult.value.news);
      } else {
        errors.push("news unavailable");
      }

      const etfItems: EtfSummary[] = [];
      if (btcEtfResult.status === "fulfilled") {
        etfItems.push(summarizeEtf("BTC", btcEtfResult.value.flows));
      } else {
        errors.push("BTC ETF unavailable");
      }
      if (ethEtfResult.status === "fulfilled") {
        etfItems.push(summarizeEtf("ETH", ethEtfResult.value.flows));
      } else {
        errors.push("ETH ETF unavailable");
      }
      setEtfSummaries(etfItems);

      const monthlyNext: Record<string, Kline[]> = {};
      const views = (
        await Promise.all(
        focusSymbols.map(async (symbol) => {
          const [snapshotResult, dayAnalysisResult, weekAnalysisResult, monthAnalysisResult, hourKlinesResult, fourHourKlinesResult, dayKlinesResult, aiResult] = await Promise.allSettled([
            apiGet<Snapshot>(`/market/${symbol}`),
            apiGet<Analysis>(`/analysis/${symbol}?interval=1h`),
            apiGet<Analysis>(`/analysis/${symbol}?interval=4h`),
            apiGet<Analysis>(`/analysis/${symbol}?interval=1d`),
            apiGet<{ ohlcv: Kline[] }>(`/klines/${symbol}?interval=1h&limit=48`),
            apiGet<{ ohlcv: Kline[] }>(`/klines/${symbol}?interval=4h&limit=50`),
            apiGet<{ ohlcv: Kline[] }>(`/klines/${symbol}?interval=1d&limit=35`),
            apiPost<AgentResponse>("/agent/chat", {
              question: `请从日、周、月三个周期分析 ${symbol} 的趋势和风险`,
              symbols: [symbol],
              time_horizon: "multi",
            }),
          ]);

          if (
            snapshotResult.status !== "fulfilled" ||
            dayAnalysisResult.status !== "fulfilled" ||
            weekAnalysisResult.status !== "fulfilled" ||
            monthAnalysisResult.status !== "fulfilled" ||
            hourKlinesResult.status !== "fulfilled" ||
            fourHourKlinesResult.status !== "fulfilled" ||
            dayKlinesResult.status !== "fulfilled"
          ) {
            errors.push(`${symbol} core data unavailable`);
            return null;
          }

          const snapshot = snapshotResult.value;
          const dayAnalysis = dayAnalysisResult.value;
          const weekAnalysis = weekAnalysisResult.value;
          const monthAnalysis = monthAnalysisResult.value;
          const hourKlines = hourKlinesResult.value;
          const fourHourKlines = fourHourKlinesResult.value;
          const dayKlines = dayKlinesResult.value;
          monthlyNext[symbol] = dayKlines.ohlcv;

          const ai =
            aiResult.status === "fulfilled"
              ? aiResult.value
              : fallbackAgentResponse(symbol, dayAnalysis, weekAnalysis, monthAnalysis);

          if (aiResult.status !== "fulfilled") {
            errors.push(`${symbol} AI brief unavailable`);
          }

          return {
            symbol,
            snapshot,
            day: buildTrendWindow("日数据", hourKlines.ohlcv, 24, dayAnalysis),
            week: buildTrendWindow("周数据", fourHourKlines.ohlcv, 42, weekAnalysis),
            month: buildTrendWindow("月数据", dayKlines.ohlcv, 30, monthAnalysis),
            ai,
          } satisfies AssetView;
        }),
        )
      ).filter(Boolean) as AssetView[];

      setAssetViews(views);
      setMonthlySeries(monthlyNext);
      setLoadError(errors.join(" | "));
    }

    load().catch((error) => {
      console.error(error);
      setLoadError(error instanceof Error ? error.message : "dashboard load failed");
    });
  }, []);

  const topMetrics = useMemo(
    () =>
      cards.map((card) => ({
        label: card.symbol,
        value: `${card.trend} · ${card.risk_score}`,
        tone: (card.change_24h >= 0 ? "bull" : "bear") as "bull" | "bear",
      })),
    [cards],
  );

  const chartOption = useMemo(
    () => ({
      backgroundColor: "transparent",
      tooltip: { trigger: "axis" },
      legend: { textStyle: { color: "#cbd5e1" } },
      xAxis: {
        type: "category",
        data: (monthlySeries.BTCUSDT ?? []).map((item) => item.close_time.slice(5, 10)),
        axisLabel: { color: "#94a3b8" },
      },
      yAxis: { type: "value", scale: true, axisLabel: { color: "#94a3b8" } },
      series: [
        {
          name: "BTC",
          type: "line",
          smooth: true,
          data: (monthlySeries.BTCUSDT ?? []).map((item) => item.close),
          lineStyle: { color: "#f59e0b", width: 3 },
        },
        {
          name: "ETH",
          type: "line",
          smooth: true,
          data: (monthlySeries.ETHUSDT ?? []).map((item) => item.close),
          lineStyle: { color: "#3b82f6", width: 3 },
        },
      ],
    }),
    [monthlySeries],
  );

  return (
    <div className="page-grid">
      {loadError ? <p className="error-banner">{loadError}</p> : null}
      <Panel title="BTC / ETH Trend Matrix" subtitle="日、周、月多周期趋势">
        <MetricGrid items={topMetrics} />
        <div className="focus-grid">
          {assetViews.map((view) => (
            <article key={view.symbol} className="focus-card">
              <div className="row-between">
                <div>
                  <h3>{view.symbol}</h3>
                  <p className="muted">
                    ${view.snapshot.price.toFixed(2)} · 24H {view.snapshot.change_24h.toFixed(2)}%
                  </p>
                </div>
                <span className={`risk-pill risk-${view.snapshot.risk_level}`}>{view.snapshot.risk_level}</span>
              </div>
              <div className="trend-window-grid">
                {[view.day, view.week, view.month].map((window) => (
                  <div key={window.label} className="trend-window">
                    <p className="muted">{window.label}</p>
                    <strong className={window.change >= 0 ? "tone-bull" : "tone-bear"}>{window.change.toFixed(2)}%</strong>
                    <p>{window.trend}</p>
                    <p className="muted">{window.regime}</p>
                  </div>
                ))}
              </div>
            </article>
          ))}
        </div>
      </Panel>

      <Panel title="AI Briefing" subtitle="BTC / ETH 自动研究结果">
        <div className="focus-grid">
          {assetViews.map((view) => (
            <article key={`${view.symbol}-ai`} className="focus-card">
              <div className="row-between">
                <h3>{view.symbol} AI</h3>
                <span className={`risk-pill risk-${view.ai.risk_level}`}>{view.ai.risk_level}</span>
              </div>
              <p className="summary-text">{view.ai.answer}</p>
              <div className="tag-row">
                <span className="price-tag bull-tag">日 {view.day.trend}</span>
                <span className="price-tag">{view.week.trend}</span>
                <span className="price-tag bear-tag">月 {view.month.trend}</span>
              </div>
              <p className="muted">更新时间 {new Date(view.ai.data_timestamp).toLocaleString("zh-CN", { hour12: false })}</p>
            </article>
          ))}
        </div>
      </Panel>

      <Panel title="30D Relative Trend" subtitle="BTC 与 ETH 月级收盘轨迹">
        <EChart option={chartOption} />
      </Panel>

      <Panel title="ETF Snapshot" subtitle="BTC / ETH 现货 ETF 资金流">
        <div className="focus-grid">
          {etfSummaries.map((summary) => (
            <article key={summary.asset} className="focus-card">
              <div className="row-between">
                <h3>{summary.asset} ETF</h3>
                <span className={`risk-pill ${summary.latest >= 0 ? "risk-low" : "risk-high"}`}>
                  {summary.latest >= 0 ? "inflow" : "outflow"}
                </span>
              </div>
              <div className="metric-grid">
                <article className="metric-card">
                  <p className="muted">Latest</p>
                  <strong>{formatUSD(summary.latest)}</strong>
                </article>
                <article className="metric-card">
                  <p className="muted">7D</p>
                  <strong>{formatUSD(summary.weekly)}</strong>
                </article>
                <article className="metric-card">
                  <p className="muted">30D</p>
                  <strong>{formatUSD(summary.monthly)}</strong>
                </article>
              </div>
            </article>
          ))}
        </div>
      </Panel>

      <Panel title="News Feed" subtitle="市场新闻与情绪样本">
        <div className="list">
          {news.map((item) => (
            <article key={item.id} className="list-item">
              <div className="row-between">
                <strong>{item.title}</strong>
                <span className={`risk-pill risk-${item.sentiment === "negative" ? "high" : item.sentiment === "positive" ? "low" : "medium"}`}>
                  {item.sentiment}
                </span>
              </div>
              <p className="muted">{item.summary}</p>
            </article>
          ))}
        </div>
      </Panel>
    </div>
  );
}

function buildTrendWindow(label: string, klines: Kline[], lookbackBars: number, analysis: Analysis): TrendWindow {
  return {
    label,
    change: computeChange(klines, lookbackBars),
    trend: analysis.trend,
    regime: analysis.market_regime,
    summary: analysis.summary,
  };
}

function computeChange(klines: Kline[], lookbackBars: number): number {
  if (klines.length < 2) {
    return 0;
  }
  const latest = klines[klines.length - 1]?.close ?? 0;
  const anchorIndex = Math.max(0, klines.length-1-lookbackBars);
  const anchor = klines[anchorIndex]?.close ?? latest;
  if (!anchor) {
    return 0;
  }
  return ((latest / anchor) - 1) * 100;
}

function summarizeEtf(asset: string, rows: ETFFlow[]): EtfSummary {
  const latest = rows[0]?.net_flow_usd ?? 0;
  const weekly = rows.slice(0, 7).reduce((sum, item) => sum + item.net_flow_usd, 0);
  const monthly = rows.slice(0, 30).reduce((sum, item) => sum + item.net_flow_usd, 0);
  return { asset, latest, weekly, monthly, rows };
}

function formatUSD(value: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    notation: "compact",
    maximumFractionDigits: 2,
  }).format(value);
}

function fallbackAgentResponse(symbol: string, day: Analysis, week: Analysis, month: Analysis): AgentResponse {
  return {
    answer: `${symbol} AI 摘要暂时不可用，先参考当前结构化分析结果。日线 ${day.trend}，周线 ${week.trend}，月线 ${month.trend}。`,
    trend: day.trend,
    risk_level: day.risk_level,
    support: month.support_levels,
    resistance: month.resistance_levels,
    evidence: [],
    data_timestamp: day.ts,
  };
}
