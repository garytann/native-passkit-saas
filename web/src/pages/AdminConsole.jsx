import React, { useEffect, useMemo, useState } from "react";
import {
  BadgeCheck,
  Building2,
  Check,
  ChevronRight,
  CreditCard,
  FileKey2,
  ImageUp,
  LogOut,
  Menu,
  RefreshCw,
  Search,
  ShieldCheck,
  X,
} from "lucide-react";
import BrandLogo from "../components/BrandLogo.jsx";
import { ADMIN_SESSION_KEY, API_BASE, API_VERSION_PREFIX } from "../config/api.js";

const steps = [
  { key: "payment", label: "Payment", icon: CreditCard },
  { key: "identity", label: "Apple identity", icon: ShieldCheck },
  { key: "certificate", label: "Certificate", icon: FileKey2 },
  { key: "design", label: "Pass design", icon: ImageUp },
  { key: "activate", label: "Review & activate", icon: BadgeCheck },
];

const assetNames = ["icon.png", "icon@2x.png", "icon@3x.png", "logo.png", "logo@2x.png", "logo@3x.png"];

export default function AdminConsolePage() {
  const [session, setSession] = useState(null);
  const [merchants, setMerchants] = useState([]);
  const [selectedID, setSelectedID] = useState("");
  const [onboarding, setOnboarding] = useState(null);
  const [activeStep, setActiveStep] = useState("payment");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("loading");
  const [message, setMessage] = useState("");
  const [mobileMenu, setMobileMenu] = useState(false);

  useEffect(() => {
    const raw = window.sessionStorage.getItem(ADMIN_SESSION_KEY);
    if (!raw) {
      window.location.href = "/admin/login";
      return;
    }
    try {
      const parsed = JSON.parse(raw);
      if (!parsed?.token || parsed?.admin?.role !== "platform_admin") throw new Error("invalid session");
      setSession(parsed);
    } catch {
      window.sessionStorage.removeItem(ADMIN_SESSION_KEY);
      window.location.href = "/admin/login";
    }
  }, []);

  useEffect(() => {
    if (session?.token) loadMerchants(session.token);
  }, [session]);

  useEffect(() => {
    if (session?.token && selectedID) loadOnboarding(selectedID, session.token);
  }, [selectedID, session]);

  async function api(path, options = {}) {
    const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}${path}`, {
      ...options,
      headers: { Authorization: `Bearer ${session?.token}`, ...(options.headers ?? {}) },
    });
    const payload = await response.json().catch(() => null);
    if (!response.ok) {
      if (response.status === 401) logout();
      throw new Error(payload?.error ?? `Request returned ${response.status}`);
    }
    return payload;
  }

  async function loadMerchants(token = session?.token) {
    setStatus("loading");
    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/admin/merchants`, { headers: { Authorization: `Bearer ${token}` } });
      const payload = await response.json().catch(() => null);
      if (!response.ok) throw new Error(payload?.error ?? `Merchant list returned ${response.status}`);
      const list = payload?.merchants ?? [];
      setMerchants(list);
      setSelectedID((current) => current || list[0]?.merchant_id || "");
      setStatus("ready");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Unable to load merchants");
      setStatus("error");
    }
  }

  async function loadOnboarding(merchantID = selectedID, token = session?.token) {
    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/admin/merchants/${encodeURIComponent(merchantID)}/onboarding`, { headers: { Authorization: `Bearer ${token}` } });
      const payload = await response.json().catch(() => null);
      if (!response.ok) throw new Error(payload?.error ?? `Onboarding status returned ${response.status}`);
      setOnboarding(payload);
      setActiveStep(firstOpenStep(payload));
      setMessage("");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Unable to load onboarding status");
    }
  }

  async function runAction(action, successMessage) {
    setStatus("saving");
    setMessage("");
    try {
      await action();
      await loadOnboarding();
      await loadMerchants();
      setMessage(successMessage);
      setStatus("ready");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Unable to save this step");
      setStatus("error");
    }
  }

  function logout() {
    window.sessionStorage.removeItem(ADMIN_SESSION_KEY);
    window.location.href = "/admin/login";
  }

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) return merchants;
    return merchants.filter((merchant) => [merchant.merchant, merchant.merchant_email, merchant.merchant_id].some((value) => value?.toLowerCase().includes(query)));
  }, [merchants, search]);

  if (!session) return null;

  return (
    <div className="admin-console">
      <header className="admin-console-header">
        <a href="/admin" aria-label="Just Loyalty administration"><BrandLogo variant="dark" className="admin-console-logo" /></a>
        <div className="admin-console-header-actions">
          <span>{session.admin.email}</span>
          <button className="icon-button" type="button" onClick={logout} title="Sign out"><LogOut size={18} /></button>
        </div>
        <button className="icon-button admin-mobile-menu" type="button" onClick={() => setMobileMenu(true)} aria-label="Open merchant list"><Menu size={19} /></button>
      </header>

      <aside className={`admin-merchant-rail ${mobileMenu ? "open" : ""}`}>
        <div className="admin-rail-heading">
          <div><span className="eyebrow">Operations</span><h1>Merchant onboarding</h1></div>
          <button className="icon-button admin-rail-close" type="button" onClick={() => setMobileMenu(false)} aria-label="Close merchant list"><X size={18} /></button>
        </div>
        <label className="admin-search"><Search size={16} /><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search merchants" /></label>
        <div className="admin-merchant-list">
          {filtered.map((merchant) => (
            <button className={`admin-merchant-row ${selectedID === merchant.merchant_id ? "active" : ""}`} type="button" key={merchant.merchant_id} onClick={() => { setSelectedID(merchant.merchant_id); setMobileMenu(false); }}>
              <span className="merchant-initial">{merchant.merchant?.slice(0, 1)}</span>
              <span><strong>{merchant.merchant}</strong><small>{merchant.merchant_id}</small></span>
              <span className={`status-dot ${merchant.payment_status}`} title={`Payment ${merchant.payment_status}`} />
            </button>
          ))}
          {status !== "loading" && filtered.length === 0 ? <p className="admin-empty">No merchants found.</p> : null}
        </div>
      </aside>

      <main className="admin-workspace">
        {onboarding ? (
          <>
            <section className="admin-merchant-heading">
              <div>
                <span className="eyebrow">{onboarding.merchant.merchant_id}</span>
                <h2>{onboarding.merchant.merchant}</h2>
                <p>{onboarding.merchant.merchant_email}</p>
              </div>
              <div className="admin-heading-actions">
                <span className={`onboarding-badge ${onboarding.onboarding_status}`}>{statusLabel(onboarding.onboarding_status)}</span>
                <button className="icon-button" type="button" onClick={() => loadOnboarding()} title="Refresh status"><RefreshCw size={17} /></button>
              </div>
            </section>

            <nav className="onboarding-stepper" aria-label="Onboarding steps">
              {steps.map((step, index) => {
                const complete = stepComplete(step.key, onboarding);
                const Icon = step.icon;
                return (
                  <button key={step.key} type="button" className={`${activeStep === step.key ? "active" : ""} ${complete ? "complete" : ""}`} onClick={() => setActiveStep(step.key)}>
                    <span>{complete ? <Check size={16} /> : <Icon size={16} />}</span>
                    <small>Step {index + 1}</small><strong>{step.label}</strong>
                  </button>
                );
              })}
            </nav>

            <section className="onboarding-panel">
              {activeStep === "payment" ? <PaymentStep onboarding={onboarding} busy={status === "saving"} onSave={(paymentStatus) => runAction(() => api(`/admin/merchants/${selectedID}/payment-status`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ status: paymentStatus }) }), `Payment marked ${paymentStatus}.`)} /> : null}
              {activeStep === "identity" ? <IdentityStep onboarding={onboarding} busy={status === "saving"} onSave={(values) => runAction(async () => {
                await api(`/admin/merchants/${selectedID}/apple-wallet/issuer`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ ownership_mode: "platform_managed", team_identifier: values.teamIdentifier, organization_name: values.organizationName }) });
                if (!onboarding.pass_type_configured) await api(`/admin/merchants/${selectedID}/apple-wallet/pass-types`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ pass_type_identifier: values.passTypeIdentifier, pass_style: "storeCard" }) });
              }, "Apple identity saved.")} /> : null}
              {activeStep === "certificate" ? <CertificateStep onboarding={onboarding} busy={status === "saving"} onSave={(certificate, privateKey) => runAction(async () => {
                const form = new FormData(); form.append("pass_type_identifier", onboarding.pass_type.pass_type_identifier); form.append("pass_certificate", certificate); form.append("private_key", privateKey);
                await api(`/admin/merchants/${selectedID}/apple-wallet/certificate-files`, { method: "POST", body: form });
              }, "Certificate validated and staged.")} /> : null}
              {activeStep === "design" ? <DesignStep onboarding={onboarding} busy={status === "saving"} onSave={(values, files) => runAction(async () => {
                let assetBundleRef = onboarding.design?.asset_bundle_ref || onboarding.merchant.merchant_slug;
                if (files) { const form = new FormData(); assetNames.forEach((name) => form.append(name, files[name])); const uploaded = await api(`/admin/merchants/${selectedID}/apple-wallet/pass-assets`, { method: "PUT", body: form }); assetBundleRef = uploaded.asset_bundle_ref; }
                await api(`/admin/merchants/${selectedID}/apple-wallet/pass-design`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ pass_type_identifier: onboarding.pass_type.pass_type_identifier, program_name: values.programName, pass_description: values.passDescription, stamp_goal: Number(values.stampGoal), background_color: values.backgroundColor, foreground_color: values.foregroundColor, label_color: values.labelColor, asset_bundle_ref: assetBundleRef, support_text: values.supportText, terms_text: values.termsText, campaign_display_mode: "details" }) });
              }, "Pass artwork and design saved.")} /> : null}
              {activeStep === "activate" ? <ActivationStep onboarding={onboarding} busy={status === "saving"} onActivate={() => runAction(() => api(`/admin/merchants/${selectedID}/apple-wallet/activate`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ pass_type_identifier: onboarding.pass_type?.pass_type_identifier }) }), "Merchant Apple Wallet issuance is active.")} /> : null}
            </section>
            {message ? <p className={`admin-message ${status === "error" ? "error" : ""}`}>{message}</p> : null}
          </>
        ) : <div className="admin-loading"><Building2 size={26} /><p>{status === "loading" ? "Loading merchants" : "Select a merchant to begin"}</p></div>}
      </main>
      {mobileMenu ? <button type="button" className="admin-mobile-backdrop" onClick={() => setMobileMenu(false)} aria-label="Close merchant list" /> : null}
    </div>
  );
}

function PaymentStep({ onboarding, busy, onSave }) {
  const active = onboarding.payment_complete;
  return <StepShell eyebrow="Account access" title="Confirm membership payment" description="Pass issuance remains blocked until the merchant's membership is active."><div className="payment-summary"><CreditCard size={22} /><div><strong>Payment is {active ? "active" : "inactive"}</strong><p>{active ? "This merchant can continue to Apple Wallet provisioning." : "Confirm payment before provisioning credentials."}</p></div></div><button className="primary-action" disabled={busy} type="button" onClick={() => onSave(active ? "inactive" : "active")}>{active ? "Mark inactive" : "Confirm payment"}<ChevronRight size={17} /></button></StepShell>;
}

function IdentityStep({ onboarding, busy, onSave }) {
  const suggested = `pass.com.justloyalty.${onboarding.merchant.merchant_slug.replaceAll("-", "")}.loyalty`;
  const [values, setValues] = useState({ teamIdentifier: onboarding.issuer?.team_identifier ?? "", organizationName: onboarding.issuer?.organization_name ?? onboarding.merchant.merchant, passTypeIdentifier: onboarding.pass_type?.pass_type_identifier ?? suggested });
  return <StepShell eyebrow="Apple Developer" title="Configure Apple identity" description="Create this Pass Type ID in Apple Developer, then save the exact identifiers here."><form className="onboarding-form" onSubmit={(event) => { event.preventDefault(); onSave(values); }}><Field label="Team identifier"><input value={values.teamIdentifier} onChange={(event) => setValues({ ...values, teamIdentifier: event.target.value })} required /></Field><Field label="Organization name"><input value={values.organizationName} onChange={(event) => setValues({ ...values, organizationName: event.target.value })} required /></Field><Field label="Pass Type Identifier" wide><input value={values.passTypeIdentifier} onChange={(event) => setValues({ ...values, passTypeIdentifier: event.target.value })} readOnly={onboarding.pass_type_configured} required /></Field><button className="primary-action" disabled={busy || !onboarding.payment_complete} type="submit">Save Apple identity<ChevronRight size={17} /></button></form></StepShell>;
}

function CertificateStep({ onboarding, busy, onSave }) {
  const [certificate, setCertificate] = useState(null); const [privateKey, setPrivateKey] = useState(null);
  return <StepShell eyebrow="Signing credential" title="Upload and validate certificate" description="Upload the PEM certificate returned by Apple and the private key that created its CSR."><form className="onboarding-form upload-form" onSubmit={(event) => { event.preventDefault(); onSave(certificate, privateKey); }}><FileField label="Pass certificate" hint="pass-cert.pem" onChange={setCertificate} /><FileField label="Private key" hint="pass-key.pem" onChange={setPrivateKey} /><div className="secure-upload-note"><ShieldCheck size={18} /><span>The private key is stored with restricted permissions and is never returned by the API.</span></div><button className="primary-action" disabled={busy || !certificate || !privateKey || !onboarding.pass_type_configured} type="submit">Upload and validate<ChevronRight size={17} /></button></form></StepShell>;
}

function DesignStep({ onboarding, busy, onSave }) {
  const existing = onboarding.design;
  const [values, setValues] = useState({ programName: existing?.program_name ?? `${onboarding.merchant.merchant} Loyalty Program`, passDescription: existing?.pass_description ?? `Collect stamps and enjoy rewards at ${onboarding.merchant.merchant}.`, stampGoal: existing?.stamp_goal ?? 10, backgroundColor: existing?.background_color ?? "#5434F5", foregroundColor: existing?.foreground_color ?? "#FFFFFF", labelColor: existing?.label_color ?? "#E1DCFF", supportText: existing?.support_text ?? `Contact ${onboarding.merchant.merchant} for loyalty card assistance.`, termsText: existing?.terms_text ?? "Collect one stamp for each eligible purchase. Rewards are subject to outlet terms." });
  const [files, setFiles] = useState({}); const completeFiles = assetNames.every((name) => files[name]);
  return <StepShell eyebrow="Wallet appearance" title="Upload artwork and define the pass" description="The same validated artwork and fields are used for both the preview and issued Wallet pass."><form className="onboarding-form design-form" onSubmit={(event) => { event.preventDefault(); onSave(values, completeFiles ? files : null); }}><Field label="Program name"><input value={values.programName} onChange={(event) => setValues({ ...values, programName: event.target.value })} required /></Field><Field label="Stamp goal"><input type="number" min="1" max="100" value={values.stampGoal} onChange={(event) => setValues({ ...values, stampGoal: event.target.value })} required /></Field><Field label="Pass description" wide><textarea value={values.passDescription} onChange={(event) => setValues({ ...values, passDescription: event.target.value })} required /></Field><div className="color-fields"><ColorField label="Background" value={values.backgroundColor} onChange={(value) => setValues({ ...values, backgroundColor: value })} /><ColorField label="Text" value={values.foregroundColor} onChange={(value) => setValues({ ...values, foregroundColor: value })} /><ColorField label="Labels" value={values.labelColor} onChange={(value) => setValues({ ...values, labelColor: value })} /></div><div className="asset-file-grid">{assetNames.map((name) => <FileField key={name} label={name} hint={files[name]?.name ?? (onboarding.assets_uploaded ? "Already uploaded" : "PNG required")} accept="image/png" onChange={(file) => setFiles((current) => ({ ...current, [name]: file }))} />)}</div><Field label="Support text" wide><input value={values.supportText} onChange={(event) => setValues({ ...values, supportText: event.target.value })} /></Field><Field label="Terms" wide><textarea value={values.termsText} onChange={(event) => setValues({ ...values, termsText: event.target.value })} /></Field><button className="primary-action" disabled={busy || (!onboarding.assets_uploaded && !completeFiles) || !onboarding.certificate_validated} type="submit">Save pass design<ChevronRight size={17} /></button></form></StepShell>;
}

function ActivationStep({ onboarding, busy, onActivate }) {
  const checks = [["Payment active", onboarding.payment_complete], ["Issuer configured", onboarding.issuer_configured], ["Pass Type ID associated", onboarding.pass_type_configured], ["Certificate validated", onboarding.certificate_validated], ["Artwork validated", onboarding.assets_uploaded], ["Pass design configured", onboarding.design_configured]];
  return <StepShell eyebrow="Launch review" title="Activate merchant issuance" description="All checks must pass before this merchant can generate signed Apple Wallet passes."><div className="activation-checks">{checks.map(([label, ready]) => <div key={label} className={ready ? "ready" : "pending"}><span>{ready ? <Check size={15} /> : null}</span><strong>{label}</strong><small>{ready ? "Ready" : "Required"}</small></div>)}</div>{onboarding.onboarding_status === "active" ? <div className="activation-success"><BadgeCheck size={24} /><div><strong>Apple Wallet issuance is active</strong><p>Customers can now receive this merchant's signed loyalty pass.</p></div></div> : <button className="primary-action" type="button" disabled={busy || !onboarding.can_activate} onClick={onActivate}>Activate merchant<BadgeCheck size={18} /></button>}</StepShell>;
}

function StepShell({ eyebrow, title, description, children }) { return <div className="step-shell"><header><span className="eyebrow">{eyebrow}</span><h3>{title}</h3><p>{description}</p></header>{children}</div>; }
function Field({ label, wide, children }) { return <label className={wide ? "wide" : ""}><span>{label}</span>{children}</label>; }
function FileField({ label, hint, onChange, accept = ".pem" }) { return <label className="file-field"><span>{label}</span><input type="file" accept={accept} onChange={(event) => onChange(event.target.files?.[0] ?? null)} required={hint !== "Already uploaded"} /><small>{hint}</small></label>; }
function ColorField({ label, value, onChange }) { return <label className="color-field"><span>{label}</span><div><input type="color" value={value} onChange={(event) => onChange(event.target.value.toUpperCase())} /><input value={value} pattern="#[0-9A-Fa-f]{6}" onChange={(event) => onChange(event.target.value)} /></div></label>; }
function statusLabel(value) { return ({ awaiting_payment: "Awaiting payment", needs_issuer: "Issuer required", needs_pass_type: "Pass Type required", needs_certificate: "Certificate required", needs_design: "Design required", ready: "Ready to activate", active: "Active" })[value] ?? "Provisioning"; }
function firstOpenStep(status) { if (!status.payment_complete) return "payment"; if (!status.issuer_configured || !status.pass_type_configured) return "identity"; if (!status.certificate_validated) return "certificate"; if (!status.design_configured || !status.assets_uploaded) return "design"; return "activate"; }
function stepComplete(step, status) { return ({ payment: status.payment_complete, identity: status.issuer_configured && status.pass_type_configured, certificate: status.certificate_validated, design: status.design_configured && status.assets_uploaded, activate: status.onboarding_status === "active" })[step]; }
