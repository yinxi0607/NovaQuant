import { useEffect, useState } from "react";
import { Panel } from "../components/Panel";
import { apiGet } from "../lib/api";

type WhaleTransaction = {
  id: string;
  asset: string;
  amount: number;
  amount_usd: number;
  direction: string;
  from_label: string;
  to_label: string;
};

export function WhalePage() {
  const [rows, setRows] = useState<WhaleTransaction[]>([]);

  useEffect(() => {
    apiGet<{ transactions: WhaleTransaction[] }>("/whale?asset=BTC").then((data) => setRows(data.transactions));
  }, []);

  return (
    <Panel title="Whale Tracker" subtitle="Mock + API provider compatible">
      <div className="list">
        {rows.map((item) => (
          <article key={item.id} className="list-item">
            <div className="row-between">
              <strong>{item.asset}</strong>
              <span className={`risk-pill risk-${item.direction === "outflow" ? "medium" : "low"}`}>{item.direction}</span>
            </div>
            <p>{item.amount.toFixed(2)} units · ${item.amount_usd.toLocaleString()}</p>
            <p className="muted">
              {item.from_label} → {item.to_label}
            </p>
          </article>
        ))}
      </div>
    </Panel>
  );
}
