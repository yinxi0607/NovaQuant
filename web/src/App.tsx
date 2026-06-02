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
          <div className="timestamp-pill">
            {new Date().toLocaleString("zh-CN", { hour12: false })}
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
