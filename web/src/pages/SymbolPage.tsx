import { useEffect, useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { EChart } from "../components/EChart";
import { MetricGrid } from "../components/MetricGrid";
import { Panel } from "../components/Panel";
import { Analysis, Kline, Risk, Snapshot, apiGet } from "../lib/api";

export function SymbolPage() {
  const { symbol = "BTCUSDT" } = useParams();
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [analysis, setAnalysis] = useState<Analysis | null>(null);
  const [risk, setRisk] = useState<Risk | null>(null);
  const [klines, setKlines] = useState<Kline[]>([]);

  useEffect(() => {
    apiGet<Snapshot>(`/market/${symbol}`).then(setSnapshot);
    apiGet<Analysis>(`/analysis/${symbol}?interval=1h`).then(setAnalysis);
    apiGet<Risk>(`/risk/${symbol}`).then(setRisk);
    apiGet<{ ohlcv: Kline[] }>(`/klines/${symbol}?interval=1h&limit=72`).then((data) => setKlines(data.ohlcv));
  }, [symbol]);

  const option = useMemo(
    () => ({
      backgroundColor: "transparent",
      tooltip: { trigger: "axis" },
      xAxis: { type: "category", data: klines.map((item) => item.close_time.slice(5, 16)), axisLabel: { color: "#94a3b8" } },
      yAxis: [{ scale: true, axisLabel: { color: "#94a3b8" } }],
      series: [
        {
          type: "candlestick",
          data: klines.map((item) => [item.open, item.close, item.low, item.high]),
          itemStyle: {
            color: "#22c55e",
            color0: "#ef4444",
            borderColor: "#22c55e",
            borderColor0: "#ef4444",
          },
        },
      ],
    }),
    [klines],
  );

  return (
    <div className="page-grid">
      <Panel title={`${symbol} Snapshot`} subtitle="实时快照与研究摘要">
        {snapshot ? (
          <MetricGrid
            items={[
              { label: "Price", value: `$${snapshot.price.toFixed(2)}`, tone: "bull" },
              { label: "24H", value: `${snapshot.change_24h.toFixed(2)}%`, tone: snapshot.change_24h >= 0 ? "bull" : "bear" },
              { label: "Funding", value: `${(snapshot.funding_rate * 100).toFixed(4)}%`, tone: "warn" },
              { label: "OI", value: snapshot.open_interest.toFixed(2), tone: "neutral" },
              { label: "Risk", value: `${snapshot.risk_score} / ${snapshot.risk_level}`, tone: "warn" },
            ]}
          />
        ) : null}
        <p className="summary-text">{snapshot?.summary}</p>
      </Panel>

      <Panel title="Candles" subtitle="1H candlestick">
        <EChart option={option} height={360} />
      </Panel>

      <Panel title="Indicators" subtitle="RSI、EMA、支撑压力">
        {analysis ? (
          <MetricGrid
            items={[
              { label: "RSI14", value: `${analysis.rsi14?.toFixed(2) ?? "--"}`, tone: "warn" },
              { label: "EMA20", value: `${analysis.ema20?.toFixed(2) ?? "--"}`, tone: "bull" },
              { label: "EMA60", value: `${analysis.ema60?.toFixed(2) ?? "--"}`, tone: "neutral" },
              { label: "EMA200", value: `${analysis.ema200?.toFixed(2) ?? "--"}`, tone: "neutral" },
              { label: "ATR14", value: `${analysis.atr14?.toFixed(2) ?? "--"}`, tone: "warn" },
            ]}
          />
        ) : null}
        <div className="tag-row">
          {(analysis?.support_levels ?? []).map((value) => (
            <span key={`s-${value}`} className="price-tag bull-tag">
              S {value.toFixed(2)}
            </span>
          ))}
          {(analysis?.resistance_levels ?? []).map((value) => (
            <span key={`r-${value}`} className="price-tag bear-tag">
              R {value.toFixed(2)}
            </span>
          ))}
        </div>
      </Panel>

      <Panel title="Risk Breakdown" subtitle="按因子拆解">
        {risk ? (
          <div className="bar-list">
            {[
              ["RSI", risk.rsi_score],
              ["Funding", risk.funding_score],
              ["OI", risk.oi_score],
              ["Volatility", risk.volatility_score],
              ["Volume", risk.volume_score],
              ["News", risk.news_score],
            ].map(([label, value]) => (
              <div key={label} className="bar-row">
                <span>{label}</span>
                <div className="bar-track">
                  <div className="bar-fill" style={{ width: `${value}%` }} />
                </div>
                <strong>{value}</strong>
              </div>
            ))}
          </div>
        ) : null}
      </Panel>
    </div>
  );
}
