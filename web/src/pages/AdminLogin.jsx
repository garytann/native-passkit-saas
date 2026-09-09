import React, { useState } from "react";
import { ArrowRight, Eye, LockKeyhole, Mail, ShieldCheck } from "lucide-react";
import BrandLogo from "../components/BrandLogo.jsx";
import { ADMIN_SESSION_KEY, API_BASE, API_VERSION_PREFIX } from "../config/api.js";

export default function AdminLoginPage() {
  const [credentials, setCredentials] = useState({ email: "", password: "" });
  const [showPassword, setShowPassword] = useState(false);
  const [status, setStatus] = useState("idle");
  const [error, setError] = useState("");

  async function login(event) {
    event.preventDefault();
    setStatus("loading");
    setError("");
    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/admin/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(credentials),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        throw new Error(payload?.error ?? `Admin login returned ${response.status}`);
      }
      window.sessionStorage.setItem(ADMIN_SESSION_KEY, JSON.stringify(payload));
      window.location.href = "/admin";
    } catch (loginError) {
      setError(loginError instanceof Error ? loginError.message : "Unable to sign in");
      setStatus("error");
    }
  }

  return (
    <main className="auth-page admin-auth-page">
      <section className="auth-shell" aria-label="Platform administrator login">
        <aside className="auth-visual-panel admin-auth-visual">
          <BrandLogo variant="dark" className="auth-panel-logo" />
          <div className="auth-visual-copy">
            <span className="admin-security-label"><ShieldCheck size={17} /> Restricted workspace</span>
            <h1>Bring every merchant safely to launch</h1>
            <p>Review payment, Apple credentials, pass artwork, and activation from one controlled workflow.</p>
          </div>
        </aside>

        <section className="auth-form-panel">
          <div className="auth-form-header">
            <BrandLogo variant="icon" className="auth-form-logo" />
            <h2>Business administration</h2>
            <p>Sign in with an authorized platform administrator account.</p>
          </div>
          <form className="auth-form" onSubmit={login}>
            <label htmlFor="adminEmail">Email</label>
            <div className="auth-input">
              <Mail size={17} />
              <input id="adminEmail" type="email" autoComplete="username" value={credentials.email} onChange={(event) => setCredentials((current) => ({ ...current, email: event.target.value }))} required />
            </div>
            <label htmlFor="adminPassword">Password</label>
            <div className="auth-input">
              <LockKeyhole size={17} />
              <input id="adminPassword" type={showPassword ? "text" : "password"} autoComplete="current-password" value={credentials.password} onChange={(event) => setCredentials((current) => ({ ...current, password: event.target.value }))} required />
              <button className="auth-icon-button" type="button" onClick={() => setShowPassword((value) => !value)} aria-label={showPassword ? "Hide password" : "Show password"}>
                <Eye size={16} />
              </button>
            </div>
            <button className="primary-action auth-submit" type="submit" disabled={status === "loading"}>
              {status === "loading" ? "Signing in" : "Open admin console"}<ArrowRight size={18} />
            </button>
          </form>
          {error ? <p className="form-error auth-error">{error}</p> : null}
          <p className="auth-switch"><a href="/login">Return to merchant login</a></p>
        </section>
      </section>
    </main>
  );
}
