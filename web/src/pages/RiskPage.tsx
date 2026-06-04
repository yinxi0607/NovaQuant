import { useEffect, useState } from "react";
import { MetricGrid } from "../components/MetricGrid";
import { Panel } from "../components/Panel";
import { APIError, AlertEvent, Risk, apiGet } from "../lib/api";
import { asArray } from "../lib/collections";

export function RiskPage() {
  const [risk, setRisk] = useState<Risk | null>(null);
  const [alerts, setAlerts] = useState<AlertEvent[]>([]);
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    let cancelled = false;

    async function load() {
      const errors: string[] = [];
      const [riskResult, alertsResult] = await Promise.allSettled([
        apiGet<Risk>("/risk/BTCUSDT"),
        apiGet<{ alerts?: AlertEvent[] | null }>("/alerts"),
      ]);

      if (cancelled) {
        return;
      }

      if (riskResult.status === "fulfilled") {
        setRisk(riskResult.value);
      } else {
        errors.push(formatLoadError("risk", riskResult.reason));
      }

      if (alertsResult.status === "fulfilled") {
        const rawAlerts = alertsResult.value.alerts;
        const nextAlerts = asArray(rawAlerts);
        setAlerts(nextAlerts);
        if (rawAlerts !== undefined && !Array.isArray(rawAlerts)) {
          errors.push("alerts payload invalid");
        }
      } else {
        errors.push(formatLoadError("alerts", alertsResult.reason));
      }

      setLoadError(errors.join(" | "));
    }

    load().catch((error) => {
      if (!cancelled) {
        setLoadError(error instanceof Error ? error.message : "risk page load failed");
      }
    });

    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="page-grid">
      {loadError ? <p className="error-banner">{loadError}</p> : null}
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
          {!alerts.length ? <p className="muted">No alerts available.</p> : null}
        </div>
      </Panel>
    </div>
  );
}

function formatLoadError(scope: string, reason: unknown) {
  if (reason instanceof APIError) {
    return `${scope} request failed (${reason.status})`;
  }
  if (reason instanceof Error) {
    return `${scope} request failed: ${reason.message}`;
  }
  return `${scope} request failed`;
}
