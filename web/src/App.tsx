import { useEffect, useState } from "react";
import { NavLink, Route, Routes } from "react-router-dom";
import { OverviewPage } from "./pages/OverviewPage";
import { SymbolPage } from "./pages/SymbolPage";
import { ResearchPage } from "./pages/ResearchPage";
import { RiskPage } from "./pages/RiskPage";
import { WhalePage } from "./pages/WhalePage";
import { EtfPage } from "./pages/EtfPage";
import { NewsPage } from "./pages/NewsPage";
import { StrategyPage } from "./pages/StrategyPage";
import { SettingsPage } from "./pages/SettingsPage";
import { LoginPage } from "./pages/LoginPage";
import { AuthSession, apiGet } from "./lib/api";
import { AUTH_ENABLED, clearAuth, getStoredToken, getStoredUser, storeAuth } from "./lib/auth";

const links = [
  { to: "/", label: "Overview" },
  { to: "/research", label: "Research" },
  { to: "/risk", label: "Risk" },
  { to: "/whale", label: "Whale" },
  { to: "/etf", label: "ETF" },
  { to: "/news", label: "News" },
  { to: "/strategy", label: "Strategy" },
  { to: "/settings", label: "Settings" },
];

export default function App() {
  const [session, setSession] = useState<AuthSession | null>(() => {
    if (!AUTH_ENABLED) {
      return { username: "guest", expires_at: "" };
    }
    const username = getStoredUser();
    return username ? { username, expires_at: "" } : null;
  });
  const [bootstrapping, setBootstrapping] = useState(AUTH_ENABLED);

  useEffect(() => {
    if (!AUTH_ENABLED) {
      setBootstrapping(false);
      return;
    }
    const token = getStoredToken();
    if (!token) {
      setBootstrapping(false);
      return;
    }
    apiGet<AuthSession>("/auth/session")
      .then((data) => setSession(data))
      .catch(() => {
        clearAuth();
        setSession(null);
      })
      .finally(() => setBootstrapping(false));
  }, []);

  function handleLoggedIn(token: string, username: string) {
    const nextSession = { username, expires_at: "" };
    storeAuth(token, nextSession);
    setSession(nextSession);
  }

  function handleLogout() {
    clearAuth();
    setSession(null);
  }

  if (AUTH_ENABLED && bootstrapping) {
    return (
      <main className="auth-shell">
        <section className="auth-card">
          <p className="eyebrow">NovaQuant Access</p>
          <h1>Checking session</h1>
          <p className="muted">正在验证当前登录状态。</p>
        </section>
      </main>
    );
  }

  if (AUTH_ENABLED && !session) {
    return <LoginPage onLoggedIn={handleLoggedIn} />;
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div>
          <p className="eyebrow">NovaQuant v1</p>
          <h1>Crypto Research Console</h1>
          <p className="muted">
            Go backend + React dashboard for market overview, AI research,
            alerts, and backtesting.
          </p>
        </div>
        <nav className="nav">
          {links.map((link) => (
            <NavLink
              key={link.to}
              to={link.to}
              end={link.to === "/"}
              className={({ isActive }) => `nav-link ${isActive ? "active" : ""}`}
            >
              {link.label}
            </NavLink>
          ))}
          <NavLink to="/symbols/BTCUSDT" className={({ isActive }) => `nav-link ${isActive ? "active" : ""}`}>
            BTC Detail
          </NavLink>
        </nav>
      </aside>
      <main className="content">
        <header className="topbar">
          <div>
            <p className="eyebrow">Research Platform</p>
            <h2>Live dashboard from NovaQuant APIs</h2>
          </div>
          <div className="topbar-actions">
            {AUTH_ENABLED ? <span className="timestamp-pill">Signed in as {session?.username}</span> : null}
            <div className="timestamp-pill">
              {new Date().toLocaleString("zh-CN", { hour12: false })}
            </div>
            {AUTH_ENABLED ? (
              <button type="button" className="secondary-button" onClick={handleLogout}>
                Logout
              </button>
            ) : null}
          </div>
        </header>
        <Routes>
          <Route path="/" element={<OverviewPage />} />
          <Route path="/symbols/:symbol" element={<SymbolPage />} />
          <Route path="/research" element={<ResearchPage />} />
          <Route path="/risk" element={<RiskPage />} />
          <Route path="/whale" element={<WhalePage />} />
          <Route path="/etf" element={<EtfPage />} />
          <Route path="/news" element={<NewsPage />} />
          <Route path="/strategy" element={<StrategyPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </main>
    </div>
  );
}
