import { FormEvent, useEffect, useState } from "react";
import { AuthConfig } from "../lib/api";
import { fetchAuthConfig, login } from "../lib/auth";

type LoginPageProps = {
  onLoggedIn: (token: string, username: string) => void;
};

export function LoginPage({ onLoggedIn }: LoginPageProps) {
  const [config, setConfig] = useState<AuthConfig | null>(null);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    fetchAuthConfig()
      .then((data) => {
        setConfig(data);
        setUsername(data.username || "admin");
      })
      .catch((reason) => {
        setError(reason instanceof Error ? reason.message : "无法读取认证配置");
      });
  }, []);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      const response = await login(username.trim(), password);
      onLoggedIn(response.token, response.session.username);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "登录失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="auth-shell">
      <section className="auth-card">
        <p className="eyebrow">NovaQuant Access</p>
        <h1>Secure Login</h1>
        <p className="muted">
          口令会在前端使用 {config?.algorithm ?? "RSA-OAEP-256"} 加密后再提交。
        </p>
        <form className="stack" onSubmit={handleSubmit}>
          <label className="field">
            <span>Username</span>
            <input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" />
          </label>
          <label className="field">
            <span>Password</span>
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
            />
          </label>
          {error ? <p className="error-banner">{error}</p> : null}
          <button className="primary-button" type="submit" disabled={submitting || !config}>
            {submitting ? "Signing in..." : "Sign In"}
          </button>
        </form>
      </section>
    </main>
  );
}
