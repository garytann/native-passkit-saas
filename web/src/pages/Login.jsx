import React, { useState } from "react";
import {
  ArrowRight,
  Eye,
  LockKeyhole,
  Mail,
} from "lucide-react";
import BrandLogo from "../components/BrandLogo.jsx";
import { API_BASE, API_VERSION_PREFIX, MERCHANT_SESSION_KEY } from "../config/api";

export default function LoginPage() {
  const [credentials, setCredentials] = useState({
    username: "",
    password: "",
  });
  const [showPassword, setShowPassword] = useState(false);
  const [status, setStatus] = useState("idle");
  const [error, setError] = useState("");

  function updateCredentials(field, value) {
    setCredentials((current) => ({ ...current, [field]: value }));
  }

  async function submitLogin(event) {
    event.preventDefault();
    setStatus("loading");
    setError("");

    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/merchants/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: credentials.username,
          password: credentials.password,
        }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error ?? `Login endpoint returned ${response.status}`);
      }
      const data = await response.json();
      window.localStorage.setItem(MERCHANT_SESSION_KEY, JSON.stringify({
        token: data.token,
        merchant: data.merchant,
      }));
      window.location.href = "/dashboard";
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to login");
      setStatus("error");
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-shell" aria-label="Merchant login">
        <aside className="auth-visual-panel">
          <BrandLogo variant="dark" className="auth-panel-logo" />
          <div className="auth-visual-copy">
            <h1>Run loyalty with clarity and momentum</h1>
            <p>Manage customers, issue wallet passes, and keep reward activity moving from one focused console.</p>
          </div>
        </aside>

        <section className="auth-form-panel">
          <div className="auth-form-header">
            <BrandLogo variant="icon" className="auth-form-logo" />
            <h2>Merchant login</h2>
            <p>Access your loyalty dashboard and manage your customer rewards.</p>
          </div>

          <form className="auth-form" onSubmit={submitLogin}>
            <label htmlFor="loginUsername">Username</label>
            <div className="auth-input">
              <Mail size={17} />
              <input
                id="loginUsername"
                type="email"
                autoComplete="username"
                placeholder="merchant@example.com"
                value={credentials.username}
                onChange={(event) => updateCredentials("username", event.target.value)}
                required
              />
            </div>

            <label htmlFor="loginPassword">Password</label>
            <div className="auth-input">
              <LockKeyhole size={17} />
              <input
                id="loginPassword"
                type={showPassword ? "text" : "password"}
                autoComplete="current-password"
                placeholder="Enter your password"
                value={credentials.password}
                onChange={(event) => updateCredentials("password", event.target.value)}
                required
              />
              <button
                className="auth-icon-button"
                type="button"
                onClick={() => setShowPassword((value) => !value)}
                aria-label={showPassword ? "Hide password" : "Show password"}
                title={showPassword ? "Hide password" : "Show password"}
              >
                <Eye size={16} />
              </button>
            </div>

            <button className="primary-action auth-submit" type="submit" disabled={status === "loading"}>
              {status === "loading" ? "Logging in" : "Login"}
              <ArrowRight size={18} />
            </button>
          </form>

          {error ? <p className="form-error auth-error">{error}</p> : null}

          <p className="auth-switch">
            Don't have an account? <a href="/merchants/register">Sign up</a>
          </p>
        </section>
      </section>
    </main>
  );
}
