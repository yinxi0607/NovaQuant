import { render, screen, waitFor } from "@testing-library/react";
import { RiskPage } from "./RiskPage";

const riskPayload = {
  symbol: "BTCUSDT",
  ts: "2026-06-04T00:00:00Z",
  total_score: 58,
  risk_level: "medium",
  rsi_score: 62,
  funding_score: 48,
  oi_score: 51,
  volatility_score: 73,
  volume_score: 45,
  news_score: 28,
  explanation: {},
};

describe("RiskPage", () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.includes("/risk/BTCUSDT")) {
        return Promise.resolve(new Response(JSON.stringify(riskPayload)));
      }
      if (url.includes("/alerts")) {
        return Promise.resolve(new Response(JSON.stringify({ items: [] })));
      }
      return Promise.resolve(new Response(JSON.stringify({})));
    }) as typeof fetch;
  });

  it("renders risk metrics and survives an invalid alerts payload", async () => {
    render(<RiskPage />);

    await waitFor(() => expect(screen.getByText("Risk Gauge")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText("58")).toBeInTheDocument());

    expect(screen.getByText("alerts payload invalid")).toBeInTheDocument();
    expect(screen.getByText("No alerts available.")).toBeInTheDocument();
  });
});
