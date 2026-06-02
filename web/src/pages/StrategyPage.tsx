import { useMemo, useState } from "react";
import { EChart } from "../components/EChart";
import { Panel } from "../components/Panel";
import { BacktestResponse, apiPost } from "../lib/api";

export function StrategyPage() {
  const [response, setResponse] = useState<BacktestResponse | null>(null);
  const [loading, setLoading] = useState(false);

  async function runBacktest(strategy: string) {
    setLoading(true);
    try {
      const now = new Date();
      const start = new Date(now.getTime() - 8 * 24 * 3600 * 1000);
      const data = await apiPost<BacktestResponse>("/backtest", {
        strategy,
        symbol: "BTCUSDT",
        interval: "1h",
        start_time: start.toISOString(),
        end_time: now.toISOString(),
        params: {},
      });
      setResponse(data);
    } finally {
      setLoading(false);
    }
  }

  const option = useMemo(
    () => ({
      xAxis: { type: "category", data: response?.result.equity_curve.map((item) => item.time.slice(5, 16)) ?? [], axisLabel: { color: "#94a3b8" } },
      yAxis: { type: "value", axisLabel: { color: "#94a3b8" } },
      series: [{ type: "line", data: response?.result.equity_curve.map((item) => item.equity) ?? [], lineStyle: { color: "#f59e0b" } }],
    }),
    [response],
  );

  return (
    <div className="page-grid">
      <Panel title="Strategy Lab" subtitle="RSI / EMA crossover / MACD">
        <div className="button-row">
          <button className="primary-button" disabled={loading} onClick={() => runBacktest("ema_crossover")}>
            跑 EMA Crossover
          </button>
          <button className="secondary-button" disabled={loading} onClick={() => runBacktest("rsi")}>
            跑 RSI
          </button>
          <button className="secondary-button" disabled={loading} onClick={() => runBacktest("macd")}>
            跑 MACD
          </button>
        </div>
        {response ? (
          <div className="metric-grid">
            <article className="metric-card"><p className="muted">Return</p><strong>{response.result.total_return.toFixed(2)}%</strong></article>
            <article className="metric-card"><p className="muted">Win rate</p><strong>{response.result.win_rate.toFixed(2)}%</strong></article>
            <article className="metric-card"><p className="muted">Drawdown</p><strong>{response.result.max_drawdown.toFixed(2)}%</strong></article>
            <article className="metric-card"><p className="muted">Trades</p><strong>{response.result.trade_count}</strong></article>
          </div>
        ) : null}
      </Panel>
      <Panel title="Equity Curve" subtitle="策略权益曲线">
        <EChart option={option} />
      </Panel>
    </div>
  );
}
