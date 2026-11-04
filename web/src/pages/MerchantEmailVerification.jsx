import React, { useEffect, useMemo, useState } from "react";
import { ArrowRight, Check, Mail, RefreshCw } from "lucide-react";
import BrandLogo from "../components/BrandLogo.jsx";
import {
  API_BASE,
  API_VERSION_PREFIX,
  PENDING_MERCHANT_VERIFICATION_KEY,
  SAAS_BRAND_NAME,
} from "../config/api";

function readPendingVerification() {
  try {
    return JSON.parse(window.sessionStorage.getItem(PENDING_MERCHANT_VERIFICATION_KEY) ?? "null") ?? {};
  } catch {
    return {};
  }
}

export default function MerchantEmailVerificationPage() {
  const pending = useMemo(readPendingVerification, []);
  const [email, setEmail] = useState(pending.merchantEmail ?? "");
  const [pin, setPIN] = useState("");
  const [status, setStatus] = useState("idle");
  const [error, setError] = useState("");
  const [secondsUntilResend, setSecondsUntilResend] = useState(() => {
    const availableAt = Date.parse(pending.resendAvailableAt ?? "");
    return Number.isFinite(availableAt) ? Math.max(0, Math.ceil((availableAt - Date.now()) / 1000)) : 0;
  });

  useEffect(() => {
    if (secondsUntilResend <= 0) return undefined;
    const timer = window.setInterval(() => {
      setSecondsUntilResend((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [secondsUntilResend]);

  function updatePIN(value) {
    setPIN(value.replace(/\D/g, "").slice(0, 6));
  }

  async function verify(event) {
    event.preventDefault();
    setStatus("loading");
    setError("");
    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/merchants/verify-email`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ merchant_email: email, pin }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error ?? `Verification endpoint returned ${response.status}`);
      }
      window.sessionStorage.removeItem(PENDING_MERCHANT_VERIFICATION_KEY);
      setStatus("verified");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to verify email");
      setStatus("error");
    }
  }

  async function resend() {
    if (!email || secondsUntilResend > 0 || status === "resending") return;
    setStatus("resending");
    setError("");
    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/merchants/verification/resend`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ merchant_email: email }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error ?? `Resend endpoint returned ${response.status}`);
      }
      setSecondsUntilResend(60);
      setStatus("resent");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to resend verification code");
      setStatus("error");
    }
  }

  function changeEmail() {
    window.sessionStorage.removeItem(PENDING_MERCHANT_VERIFICATION_KEY);
    window.location.href = "/merchants/register";
  }

  return (
    <div className="public-page merchant-verification-page">
      <header className="public-header">
        <div className="public-brand" aria-label={SAAS_BRAND_NAME}>
          <BrandLogo className="public-brand-logo" />
        </div>
      </header>

      <main className="verification-layout">
        <section className="registration-card verification-card" aria-label="Verify merchant email">
          {status === "verified" ? (
            <div className="success-panel">
              <div className="success-icon"><Check size={28} /></div>
              <span className="eyebrow">Email verified</span>
              <h1>Your merchant account is ready</h1>
              <p>Welcome to Just Loyalty. A welcome email is on its way, and you can now sign in to continue onboarding.</p>
              <div className="success-actions">
                <a className="primary-action" href="/login">Go to Login <ArrowRight size={18} /></a>
              </div>
            </div>
          ) : (
            <>
              <div className="verification-heading">
                <div className="verification-icon"><Mail size={22} /></div>
                <span className="eyebrow">Email verification</span>
                <h1>Check your inbox</h1>
                <p>Enter the six-digit code sent to your merchant email. The code expires after 10 minutes.</p>
              </div>

              <form className="public-registration-form verification-form" onSubmit={verify}>
                <label htmlFor="verificationEmail">Merchant email</label>
                <input
                  id="verificationEmail"
                  type="email"
                  autoComplete="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  required
                />
                <label htmlFor="verificationPIN">Verification code</label>
                <input
                  id="verificationPIN"
                  className="verification-pin-input"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  placeholder="000000"
                  value={pin}
                  onChange={(event) => updatePIN(event.target.value)}
                  minLength={6}
                  maxLength={6}
                  required
                />
                <button className="primary-action public-submit" type="submit" disabled={status === "loading" || pin.length !== 6}>
                  {status === "loading" ? "Verifying" : "Verify Email"}
                  <ArrowRight size={18} />
                </button>
              </form>

              {error ? <p className="form-error public-error">{error}</p> : null}
              {status === "resent" ? <p className="form-success">A new code will arrive shortly.</p> : null}

              <div className="verification-actions">
                <button type="button" className="text-action" onClick={resend} disabled={secondsUntilResend > 0 || status === "resending"}>
                  <RefreshCw size={15} />
                  {secondsUntilResend > 0 ? `Resend in ${secondsUntilResend}s` : status === "resending" ? "Sending" : "Resend code"}
                </button>
                <button type="button" className="text-action" onClick={changeEmail}>Use another email</button>
              </div>
            </>
          )}
        </section>
      </main>
    </div>
  );
}
