import React, { useState } from "react";
import {
  ArrowRight,
  Check,
  WalletCards,
} from "lucide-react";
import BrandLogo from "../components/BrandLogo.jsx";
import { API_BASE, API_VERSION_PREFIX, DEFAULT_MERCHANT, SAAS_BRAND_NAME } from "../config/api";

export default function MembersRegisterPage({ merchantSlug = "" }) {
  const [registration, setRegistration] = useState({
    name: "",
    email: "",
  });
  const [passLinks, setPassLinks] = useState(null);
  const [status, setStatus] = useState("idle");
  const [error, setError] = useState("");

  const createdMember = passLinks?.member;
  const merchantLabel = merchantSlug ? merchantNameFromSlug(merchantSlug) : DEFAULT_MERCHANT;

  function updateRegistration(field, value) {
    setRegistration((current) => ({ ...current, [field]: value }));
  }

  async function submitRegistration(event) {
    event.preventDefault();
    setStatus("loading");
    setError("");

    try {
      const response = await fetch(registrationEndpoint(merchantSlug), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(registrationPayload(registration, merchantSlug)),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error ?? `Registration endpoint returned ${response.status}`);
      }
      const data = await response.json();
      setPassLinks(data);
      setStatus("ready");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to register member");
      setStatus("error");
    }
  }

  return (
    <div className="public-page member-register-page">
      <header className="public-header">
        <div className="public-brand" aria-label={`${merchantLabel} rewards powered by ${SAAS_BRAND_NAME}`}>
          <BrandLogo className="public-brand-logo" />
          <span className="public-brand-divider" aria-hidden="true" />
          <strong>{merchantLabel}</strong>
        </div>
      </header>

      <main className="public-registration-layout merchant-registration-layout">
        <section className="public-copy" aria-labelledby="registrationTitle">
          <h1 id="registrationTitle">Join the Reward Club</h1>
          <p>
            Begin your digital loyalty card journey today. Earn points with confidence and convenience.
          </p>
          <div className="public-benefits" aria-label="Registration benefits">
            <span><Check size={16} /> Instant pass link</span>
            <span><Check size={16} /> Mobile friendly</span>
            <span><Check size={16} /> No password needed</span>
          </div>
        </section>

        <section className="registration-card merchant-registration-card" aria-label="Member registration">
          {status === "ready" && createdMember ? (
            <SuccessPanel passLinks={passLinks} />
          ) : (
            <>
              <div className="panel-heading compact-heading">
                <div>
                  <span className="eyebrow">Member registration</span>
                  <h2>Create your loyalty pass</h2>
                </div>
              </div>

              <form className="public-registration-form" onSubmit={submitRegistration}>
                <label htmlFor="memberName">Name</label>
                <input
                  id="memberName"
                  autoComplete="name"
                  placeholder="Taylor Example"
                  value={registration.name}
                  onChange={(event) => updateRegistration("name", event.target.value)}
                  required
                />

                <label htmlFor="memberEmail">Email</label>
                <input
                  id="memberEmail"
                  type="email"
                  autoComplete="email"
                  placeholder="taylor@example.com"
                  value={registration.email}
                  onChange={(event) => updateRegistration("email", event.target.value)}
                  required
                />

                <button className="primary-action public-submit" type="submit" disabled={status === "loading"}>
                  {status === "loading" ? "Creating Pass" : "Create Pass"}
                  <ArrowRight size={18} />
                </button>
              </form>

              {error ? <p className="form-error public-error">{error}</p> : null}

              <p className="public-note">
                For now this opens the generated pass directly. Later this same flow can email the pass link after registration.
              </p>
            </>
          )}
        </section>
      </main>
    </div>
  );
}

function registrationEndpoint(merchantSlug) {
  if (merchantSlug) {
    return `${API_BASE}${API_VERSION_PREFIX}/public/merchants/${encodeURIComponent(merchantSlug)}/members/register`;
  }
  return `${API_BASE}${API_VERSION_PREFIX}/customers/members/register`;
}

function registrationPayload(registration, merchantSlug) {
  if (merchantSlug) {
    return {
      name: registration.name,
      email: registration.email,
    };
  }
  return {
    name: registration.name,
    email: registration.email,
    merchant: DEFAULT_MERCHANT,
  };
}

function merchantNameFromSlug(slug) {
  return slug
    .split("-")
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

function SuccessPanel({ passLinks }) {
  const member = passLinks.member;

  return (
    <div className="success-panel">
      <div className="success-icon">
        <Check size={28} />
      </div>
      <span className="eyebrow">Pass created</span>
      <h2>Your loyalty pass is ready</h2>
      <p>
        Member ID <strong>{member.member_id}</strong> has been registered for {member.merchant}.
      </p>

      <div className="success-actions">
        <a className="primary-action" href={passLinks.pass_url}>
          <WalletCards size={18} />
          Add to Apple Wallet
        </a>
        <a className="secondary-action" href={passLinks.preview_url} target="_blank" rel="noreferrer">
          Open Preview
        </a>
      </div>
    </div>
  );
}
