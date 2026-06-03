import { readAuthToken } from "./authStore";
export const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:50800/api/v1";

export class APIError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function apiGet<T>(path: string): Promise<T> {
  const token = readAuthToken();
  const response = await fetch(`${API_BASE}${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (!response.ok) {
    throw new APIError(response.status, await response.text());
  }
  return response.json() as Promise<T>;
}

export async function apiPost<T>(path: string, body: unknown): Promise<T> {
  const token = readAuthToken();
  const response = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw new APIError(response.status, await response.text());
  }
  return response.json() as Promise<T>;
}

export type OverviewCard = {
  symbol: string;
  price: number;
  change_24h: number;
  trend: string;
  risk_level: string;
  risk_score: number;
};

export type Snapshot = {
  symbol: string;
  price: number;
  change_24h: number;
  funding_rate: number;
  open_interest: number;
  trend: string;
  risk_level: string;
  risk_score: number;
  support: number[];
  resistance: number[];
  summary: string;
  market_regime: string;
};

export type Kline = {
  open_time: string;
  close_time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

export type Analysis = {
  symbol: string;
  ts: string;
  trend: string;
  market_regime: string;
  risk_level: string;
  risk_score: number;
  rsi14?: number;
  macd?: number;
  macd_signal?: number;
  macd_hist?: number;
  ema20?: number;
  ema60?: number;
  ema200?: number;
  atr14?: number;
  support_levels: number[];
  resistance_levels: number[];
  summary: string;
};

export type Risk = {
  symbol: string;
  ts: string;
  total_score: number;
  risk_level: string;
  rsi_score: number;
  funding_score: number;
  oi_score: number;
  volatility_score: number;
  volume_score: number;
  news_score: number;
  explanation: Record<string, unknown>;
};

export type NewsItem = {
  id: string;
  title: string;
  source: string;
  sentiment: string;
  summary: string;
  published_at: string;
};

export type ETFFlow = {
  id: string;
  asset: string;
  provider: string;
  flow_date: string;
  net_flow_usd: number;
  total_volume_usd: number;
  note: string;
};

export type AlertEvent = {
  id: string;
  symbol: string;
  severity: string;
  message: string;
  triggered_at: string;
};

export type AgentResponse = {
  answer: string;
  trend: string;
  risk_level: string;
  support: number[];
  resistance: number[];
  evidence: Array<{ type: string; summary: string; timestamp: string }>;
  data_timestamp: string;
};

export type AuthSession = {
  username: string;
  expires_at: string;
};

export type AuthConfig = {
  enabled: boolean;
  username: string;
  algorithm: string;
  public_key: string;
};

export type AuthLoginResponse = {
  token: string;
  session: AuthSession;
  expires_at: string;
};

export type BacktestResponse = {
  job_id: string;
  status: string;
  result: {
    total_return: number;
    win_rate: number;
    profit_factor: number;
    sharpe_ratio: number;
    max_drawdown: number;
    trade_count: number;
    trades: Array<{ side: string; return: number; exit_time: string }>;
    equity_curve: Array<{ time: string; equity: number }>;
  };
};
