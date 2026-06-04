import { useEffect, useState } from "react";
import { Panel } from "../components/Panel";
import { apiGet } from "../lib/api";
import { asArray } from "../lib/collections";

type SymbolConfig = {
  symbol: string;
  enabled: boolean;
  market_type: string;
};

type Health = {
  status: string;
  services: Record<string, string>;
};

export function SettingsPage() {
  const [symbols, setSymbols] = useState<SymbolConfig[]>([]);
  const [health, setHealth] = useState<Health | null>(null);

  useEffect(() => {
    apiGet<{ symbols: SymbolConfig[] | null }>("/symbols").then((data) => setSymbols(asArray(data.symbols)));
    apiGet<Health>("/health").then(setHealth);
  }, []);

  return (
    <div className="page-grid two-column">
      <Panel title="Symbol Config" subtitle="当前启用币种">
        <div className="list">
          {symbols.map((item) => (
            <article key={item.symbol} className="list-item row-between">
              <strong>{item.symbol}</strong>
              <span className="muted">{item.market_type}</span>
            </article>
          ))}
        </div>
      </Panel>
      <Panel title="API Status" subtitle="health endpoint">
        <div className="list">
          <article className="list-item">
            <strong>Overall</strong>
            <p className="muted">{health?.status ?? "loading"}</p>
          </article>
          {Object.entries(health?.services ?? {}).map(([name, value]) => (
            <article key={name} className="list-item row-between">
              <strong>{name}</strong>
              <span className={`risk-pill risk-${value === "up" ? "low" : "high"}`}>{value}</span>
            </article>
          ))}
        </div>
      </Panel>
    </div>
  );
}
