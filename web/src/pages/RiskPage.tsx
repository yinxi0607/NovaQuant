import { useEffect, useState } from "react";
import { MetricGrid } from "../components/MetricGrid";
import { Panel } from "../components/Panel";
import { AlertEvent, Risk, apiGet } from "../lib/api";

export function RiskPage() {
  const [risk, setRisk] = useState<Risk | null>(null);
  const [alerts, setAlerts] = useState<AlertEvent[]>([]);

  useEffect(() => {
    apiGet<Risk>("/risk/BTCUSDT").then(setRisk);
    apiGet<{ alerts: AlertEvent[] }>("/alerts").then((data) => setAlerts(data.alerts));
  }, []);

  return (
    <div className="page-grid">
      <Panel title="Risk Gauge" subtitle="BTC 风险中心">
        {risk ? (
          <MetricGrid
            items={[
              { label: "Total", value: `${risk.total_score}`, tone: "warn" },
              { label: "RSI", value: `${risk.rsi_score}`, tone: "warn" },
              { label: "Funding", value: `${risk.funding_score}`, tone: "neutral" },
              { label: "OI", value: `${risk.oi_score}`, tone: "neutral" },
              { label: "Volatility", value: `${risk.volatility_score}`, tone: "bear" },
              { label: "Volume", value: `${risk.volume_score}`, tone: "bull" },
            ]}
          />
        ) : null}
      </Panel>
      <Panel title="Alert Timeline" subtitle="规则引擎触发历史">
        <div className="list">
          {alerts.map((item) => (
            <article key={item.id} className="list-item">
              <div className="row-between">
                <strong>{item.symbol}</strong>
                <span className={`risk-pill risk-${item.severity === "critical" ? "high" : "medium"}`}>{item.severity}</span>
              </div>
              <p>{item.message}</p>
            </article>
          ))}
        </div>
      </Panel>
    </div>
  );
}
