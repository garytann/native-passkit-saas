import React, { useState } from "react";
import {
  ArrowRight,
  Check,
  Sparkles,
} from "lucide-react";
import { API_BASE, API_VERSION_PREFIX, SAAS_BRAND_NAME } from "../config/api";

export default function MerchantsRegisterPage() {
  const [registration, setRegistration] = useState({
    firstName: "",
    lastName: "",
    company: "",
    email: "",
    password: "",
  });
  const [createdMerchant, setCreatedMerchant] = useState(null);
  const [status, setStatus] = useState("idle");
  const [error, setError] = useState("");

  function updateRegistration(field, value) {
    setRegistration((current) => ({ ...current, [field]: value }));
  }

  async function submitRegistration(event) {
    event.preventDefault();
    setStatus("loading");
    setError("");

    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/merchants/register`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          first_name: registration.firstName,
          last_name: registration.lastName,
          company: registration.company,
          merchant_email: registration.email,
          password: registration.password,
        }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error ?? `Registration endpoint returned ${response.status}`);
      }
      const data = await response.json();
      setCreatedMerchant(data.merchant);
      setStatus("ready");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to register merchant");
      setStatus("error");
    }
  }

  return (
    <div className="public-page merchant-register-page">
      <header className="public-header">
        <div className="public-brand" aria-label={SAAS_BRAND_NAME}>
          <span className="auth-mark small">
            <Sparkles size={20} />
          </span>
          <strong>{SAAS_BRAND_NAME}</strong>
        </div>
      </header>

      <main className="public-registration-layout merchant-registration-layout">
        <section className="public-copy" aria-labelledby="merchantRegistrationTitle">
          <h1 id="merchantRegistrationTitle">Create your merchant account</h1>
          <p>
            Start building your digital loyalty program today. Register your company and prepare to issue wallet-ready rewards.
          </p>
          <div className="public-benefits" aria-label="Merchant registration benefits">
            <span><Check size={16} /> Wallet pass issuance</span>
            <span><Check size={16} /> Customer registration tools</span>
            <span><Check size={16} /> Payment activation later</span>
          </div>
        </section>

        <section className="registration-card merchant-registration-card" aria-label="Merchant registration">
          {status === "ready" && createdMerchant ? (
            <div className="success-panel">
              <div className="success-icon">
                <Check size={28} />
              </div>
              <span className="eyebrow">Account created</span>
              <h2>Welcome to {SAAS_BRAND_NAME}</h2>
              <p>
                {createdMerchant.merchant} is registered. Payment status is currently <strong>{createdMerchant.payment_status}</strong>.
              </p>
              <div className="success-actions">
                <a className="primary-action" href="/login">
                  Go to Login
                  <ArrowRight size={18} />
                </a>
              </div>
            </div>
          ) : (
            <>
              <div className="panel-heading compact-heading">
                <div>
                  <span className="eyebrow">Merchant registration</span>
                  <h2>Sign up for access</h2>
                </div>
              </div>

              <form className="public-registration-form" onSubmit={submitRegistration}>
                <div className="auth-two-column">
                  <div>
                    <label htmlFor="merchantFirstName">First name</label>
                    <input
                      id="merchantFirstName"
                      autoComplete="given-name"
                      placeholder="Gary"
                      value={registration.firstName}
                      onChange={(event) => updateRegistration("firstName", event.target.value)}
                      required
                    />
                  </div>
                  <div>
                    <label htmlFor="merchantLastName">Last name</label>
                    <input
                      id="merchantLastName"
                      autoComplete="family-name"
                      placeholder="Lee"
                      value={registration.lastName}
                      onChange={(event) => updateRegistration("lastName", event.target.value)}
                      required
                    />
                  </div>
                </div>

                <label htmlFor="merchantCompany">Company</label>
                <input
                  id="merchantCompany"
                  autoComplete="organization"
                  placeholder="Daily Brew"
                  value={registration.company}
                  onChange={(event) => updateRegistration("company", event.target.value)}
                  required
                />

                <label htmlFor="merchantEmail">Email</label>
                <input
                  id="merchantEmail"
                  type="email"
                  autoComplete="email"
                  placeholder="owner@dailybrew.example"
                  value={registration.email}
                  onChange={(event) => updateRegistration("email", event.target.value)}
                  required
                />

                <label htmlFor="merchantPassword">Password</label>
                <input
                  id="merchantPassword"
                  type="password"
                  autoComplete="new-password"
                  placeholder="At least 8 characters"
                  value={registration.password}
                  onChange={(event) => updateRegistration("password", event.target.value)}
                  minLength={8}
                  required
                />

                <button className="primary-action public-submit" type="submit" disabled={status === "loading"}>
                  {status === "loading" ? "Creating Account" : "Create Account"}
                  <ArrowRight size={18} />
                </button>
              </form>

              {error ? <p className="form-error public-error">{error}</p> : null}

              <p className="auth-switch public-note">
                Already have an account? <a href="/login">Login</a>
              </p>
            </>
          )}
        </section>
      </main>
    </div>
  );
}
