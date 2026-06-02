import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import App from "./App";

vi.mock("./components/EChart", () => ({
  EChart: () => <div data-testid="echart-mock" />,
}));

const overviewPayload = {
  cards: [
    { symbol: "BTCUSDT", price: 100000, change_24h: 2.3, trend: "bullish", risk_level: "medium", risk_score: 58 },
    { symbol: "ETHUSDT", price: 5000, change_24h: 1.1, trend: "bullish", risk_level: "low", risk_score: 32 },
  ],
};

const snapshotPayload = {
  price: 100000,
  change_24h: 2.3,
  funding_rate: 0.0001,
  open_interest: 1000,
  trend: "bullish",
  risk_level: "medium",
  risk_score: 58,
  support: [98000],
  resistance: [108000],
  summary: "summary",
  market_regime: "balanced",
};

const analysisPayload = {
  symbol: "BTCUSDT",
  ts: "2026-06-01T00:00:00Z",
  trend: "bullish",
  market_regime: "balanced",
  risk_level: "medium",
  risk_score: 58,
  rsi14: 61.2,
  support_levels: [98000],
  resistance_levels: [108000],
  summary: "analysis summary",
};

const etfPayload = {
  flows: Array.from({ length: 10 }).map((_, index) => ({
    id: `flow-${index}`,
    asset: "BTC",
    provider: "seed",
    flow_date: `2026-05-${String(index + 1).padStart(2, "0")}`,
    net_flow_usd: 1000000 - index * 1000,
    total_volume_usd: 10000000,
    note: "seed",
  })),
};

const agentPayload = {
  answer: "BTC multi-timeframe analysis result",
  trend: "bullish",
  risk_level: "medium",
  support: [98000],
  resistance: [108000],
  evidence: [],
  data_timestamp: "2026-06-01T00:00:00Z",
};

beforeEach(() => {
  globalThis.fetch = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = input.toString();
    if (url.includes("/market/overview")) {
      return Promise.resolve(new Response(JSON.stringify(overviewPayload)));
    }
    if (url.includes("/news")) {
      return Promise.resolve(new Response(JSON.stringify({ news: [] })));
    }
    if (url.includes("/market/")) {
      return Promise.resolve(new Response(JSON.stringify(snapshotPayload)));
    }
    if (url.includes("/analysis/")) {
      return Promise.resolve(new Response(JSON.stringify(analysisPayload)));
    }
    if (url.includes("/klines")) {
      return Promise.resolve(
        new Response(
          JSON.stringify({
            ohlcv: Array.from({ length: 35 }).map((_, index) => ({
              open_time: `2026-05-${String(index + 1).padStart(2, "0")}T00:00:00Z`,
              close_time: `2026-05-${String(index + 1).padStart(2, "0")}T01:00:00Z`,
              open: 100 + index,
              high: 101 + index,
              low: 99 + index,
              close: 100 + index,
              volume: 1000,
            })),
          }),
        ),
      );
    }
    if (url.includes("/etf")) {
      return Promise.resolve(new Response(JSON.stringify(etfPayload)));
    }
    if (init?.method === "POST" && url.includes("/agent/chat")) {
      return Promise.resolve(new Response(JSON.stringify(agentPayload)));
    }
    return Promise.resolve(new Response(JSON.stringify({})));
  }) as typeof fetch;
});

it("renders overview content", async () => {
  render(
    <MemoryRouter>
      <App />
    </MemoryRouter>,
  );
  await waitFor(() => expect(screen.getByText("BTC / ETH Trend Matrix")).toBeInTheDocument());
  expect(screen.getAllByText("BTCUSDT").length).toBeGreaterThan(0);
  expect(screen.getAllByText("ETHUSDT").length).toBeGreaterThan(0);
  expect(screen.getByText("AI Briefing")).toBeInTheDocument();
});
