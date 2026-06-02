import { useEffect, useMemo, useState } from "react";
import { EChart } from "../components/EChart";
import { Panel } from "../components/Panel";
import { ETFFlow, apiGet } from "../lib/api";

export function EtfPage() {
  const [btc, setBtc] = useState<ETFFlow[]>([]);
  const [eth, setEth] = useState<ETFFlow[]>([]);

  useEffect(() => {
    apiGet<{ flows: ETFFlow[] }>("/etf?asset=BTC&page_size=30").then((data) => setBtc(data.flows));
    apiGet<{ flows: ETFFlow[] }>("/etf?asset=ETH&page_size=30").then((data) => setEth(data.flows));
  }, []);

  const chartOption = useMemo(
    () => ({
      tooltip: { trigger: "axis" },
      legend: { textStyle: { color: "#cbd5e1" } },
      xAxis: {
        type: "category",
        data: [...btc].reverse().map((item) => item.flow_date.slice(5, 10)),
        axisLabel: { color: "#94a3b8" },
      },
      yAxis: { type: "value", axisLabel: { color: "#94a3b8" } },
      series: [
        {
          name: "BTC ETF",
          type: "bar",
          data: [...btc].reverse().map((item) => item.net_flow_usd),
          itemStyle: { color: "#f59e0b" },
        },
        {
          name: "ETH ETF",
          type: "line",
          smooth: true,
          data: [...eth].reverse().map((item) => item.net_flow_usd),
          lineStyle: { color: "#3b82f6", width: 3 },
        },
      ],
    }),
    [btc, eth],
  );

  return (
    <div className="page-grid">
      <Panel title="ETF Monitor" subtitle="BTC / ETH 近 30 天 ETF 流数据">
        <EChart option={chartOption} />
      </Panel>
      <Panel title="ETF Summary" subtitle="最新、7日、30日净流入">
        <div className="focus-grid">
          {[
            buildSummary("BTC", btc),
            buildSummary("ETH", eth),
          ].map((item) => (
            <article key={item.asset} className="focus-card">
              <h3>{item.asset} ETF</h3>
              <p className="muted">Latest {formatUSD(item.latest)}</p>
              <p className="muted">7D {formatUSD(item.weekly)}</p>
              <p className="muted">30D {formatUSD(item.monthly)}</p>
            </article>
          ))}
        </div>
      </Panel>
    </div>
  );
}

function buildSummary(asset: string, rows: ETFFlow[]) {
  return {
    asset,
    latest: rows[0]?.net_flow_usd ?? 0,
    weekly: rows.slice(0, 7).reduce((sum, item) => sum + item.net_flow_usd, 0),
    monthly: rows.slice(0, 30).reduce((sum, item) => sum + item.net_flow_usd, 0),
  };
}

function formatUSD(value: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    notation: "compact",
    maximumFractionDigits: 2,
  }).format(value);
}
