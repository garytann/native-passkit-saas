import React, { useEffect, useMemo, useState } from "react";
import {
  BadgeCheck,
  ExternalLink,
  Info,
  RefreshCw,
  Save,
  Smartphone,
} from "lucide-react";
import { API_BASE, API_VERSION_PREFIX } from "../config/api.js";

const emptyForm = {
  programName: "",
  passDescription: "",
  stampGoal: 10,
  rewardTitle: "",
  stampInstructions: "",
  rewardDetails: "",
  supportText: "",
  termsText: "",
  websiteURL: "",
  instagramURL: "",
  whatsappURL: "",
  showCampaignOnFront: false,
};

export default function PassesPage({ session }) {
  const [form, setForm] = useState(emptyForm);
  const [design, setDesign] = useState(null);
  const [passTypeIdentifier, setPassTypeIdentifier] = useState("");
  const [status, setStatus] = useState("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (session?.token) loadDesign();
  }, [session?.token]);

  async function request(options = {}) {
    const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/merchants/me/apple-wallet/pass-design`, {
      ...options,
      headers: {
        Authorization: `Bearer ${session.token}`,
        ...(options.headers ?? {}),
      },
    });
    const payload = await response.json().catch(() => null);
    if (!response.ok) throw new Error(payload?.error ?? `Pass design returned ${response.status}`);
    return payload;
  }

  async function loadDesign() {
    setStatus("loading");
    setMessage("");
    try {
      const payload = await request();
      setDesign(payload.design);
      setPassTypeIdentifier(payload.pass_type_identifier);
      setForm(formFromDesign(payload.design));
      setStatus("ready");
    } catch (error) {
      setStatus("error");
      setMessage(error instanceof Error ? error.message : "Unable to load pass information");
    }
  }

  async function publish(event) {
    event.preventDefault();
    setStatus("saving");
    setMessage("");
    try {
      const payload = await request({
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          program_name: form.programName,
          pass_description: form.passDescription,
          stamp_goal: Number(form.stampGoal),
          reward_title: form.rewardTitle,
          stamp_instructions: form.stampInstructions,
          reward_details: form.rewardDetails,
          support_text: form.supportText,
          terms_text: form.termsText,
          links: linksFromForm(form),
          campaign_display_mode: form.showCampaignOnFront ? "front_and_details" : "details",
        }),
      });
      setDesign(payload.design);
      setPassTypeIdentifier(payload.pass_type_identifier);
      setForm(formFromDesign(payload.design));
      setStatus("ready");
      setMessage(publishMessage(payload));
    } catch (error) {
      setStatus("error");
      setMessage(error instanceof Error ? error.message : "Unable to publish pass information");
    }
  }

  const links = useMemo(() => linksFromForm(form), [form]);
  const update = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));
  const toggleCampaignFront = (event) => setForm((current) => ({ ...current, showCampaignOnFront: event.target.checked }));

  return (
    <div className="passes-page">
      <section className="passes-heading">
        <div>
          <span className="eyebrow">Apple Wallet</span>
          <h1>Pass information</h1>
          <p>Manage the details customers see when they open the information view on your loyalty pass.</p>
        </div>
        <div className="pass-version">
          <BadgeCheck size={18} />
          <span><strong>{design?.status === "active" ? "Published" : "Setup required"}</strong><small>{design ? `Version ${design.version}` : "No active design"}</small></span>
        </div>
      </section>

      {status === "loading" ? <div className="passes-state"><RefreshCw className="spin" size={22} /><span>Loading pass information</span></div> : null}

      {status !== "loading" && !design ? (
        <div className="passes-state error">
          <Info size={22} />
          <div><strong>Pass setup is not available</strong><p>{message || "Complete Apple Wallet onboarding before editing pass information."}</p></div>
          <button className="icon-button" type="button" onClick={loadDesign} title="Retry"><RefreshCw size={18} /></button>
        </div>
      ) : null}

      {design ? (
        <div className="passes-layout">
          <form className="pass-information-form" onSubmit={publish}>
            <PassFormSection title="Programme and reward" description="These details explain what the loyalty programme offers.">
              <PassField label="Programme name"><input value={form.programName} onChange={update("programName")} maxLength={80} required /></PassField>
              <PassField label="Stamp goal"><input type="number" min="1" max="100" value={form.stampGoal} readOnly title="The stamp goal is locked after pass activation" required /></PassField>
              <PassField label="Reward title"><input value={form.rewardTitle} onChange={update("rewardTitle")} maxLength={40} placeholder="Free Coffee" required /></PassField>
              <PassField label="Pass description" wide><textarea value={form.passDescription} onChange={update("passDescription")} maxLength={160} required /></PassField>
              <PassField label="How to collect stamps" wide><textarea value={form.stampInstructions} onChange={update("stampInstructions")} maxLength={500} required /></PassField>
              <PassField label="Reward details" wide><textarea value={form.rewardDetails} onChange={update("rewardDetails")} maxLength={500} required /></PassField>
            </PassFormSection>

            <PassFormSection title="Campaign placement" description="Control where customers see the latest active promotion.">
              <label className="pass-toggle-field wide">
                <span className="pass-toggle-copy">
                  <strong>Show latest promotion on pass front</strong>
                  <small>Uses the campaign title. The full message remains in Pass Details.</small>
                </span>
                <span className="pass-toggle-control">
                  <input type="checkbox" checked={form.showCampaignOnFront} onChange={toggleCampaignFront} />
                  <span aria-hidden="true" />
                </span>
              </label>
            </PassFormSection>

            <PassFormSection title="Useful links" description="Add direct links customers can open from Wallet.">
              <PassField label="Website" wide><input type="url" value={form.websiteURL} onChange={update("websiteURL")} placeholder="https://merchant.example" /></PassField>
              <PassField label="Instagram" wide><input type="url" value={form.instagramURL} onChange={update("instagramURL")} placeholder="https://instagram.com/merchant" /></PassField>
              <PassField label="WhatsApp" wide><input type="url" value={form.whatsappURL} onChange={update("whatsappURL")} placeholder="https://wa.me/6591234567" /></PassField>
            </PassFormSection>

            <PassFormSection title="Support and legal" description="Give customers a clear way to get help and understand the programme terms.">
              <PassField label="Customer support" wide><textarea value={form.supportText} onChange={update("supportText")} maxLength={500} required /></PassField>
              <PassField label="Terms and conditions" wide><textarea className="terms-input" value={form.termsText} onChange={update("termsText")} maxLength={5000} required /></PassField>
            </PassFormSection>

            <div className="pass-publish-bar">
              <div><strong>Publish to Apple Wallet</strong><span>Installed passes will be queued for a background refresh.</span></div>
              <button className="primary-action" type="submit" disabled={status === "saving"}>
                {status === "saving" ? <RefreshCw className="spin" size={18} /> : <Save size={18} />}
                {status === "saving" ? "Publishing" : "Save and publish"}
              </button>
            </div>
            {message ? <p className={`passes-message ${status === "error" ? "error" : ""}`}>{message}</p> : null}
          </form>

          <aside className="pass-details-preview" aria-label="Apple Wallet pass preview">
            <div className="preview-title"><Smartphone size={20} /><div><span>Pass preview</span><small>{passTypeIdentifier}</small></div></div>
            <div className="wallet-front-sheet">
              <div className="wallet-front-header"><span>Stamps</span><strong>0 / {form.stampGoal}</strong></div>
              <div className="wallet-front-primary"><strong>{form.programName || "Programme name"}</strong></div>
              <div className={`wallet-front-supporting ${form.showCampaignOnFront ? "with-promotion" : ""}`}>
                <div className="wallet-front-secondary"><span>Next Reward</span><strong>{form.rewardTitle || "Loyalty Reward"}</strong></div>
                {form.showCampaignOnFront ? (
                  <div className="wallet-front-auxiliary"><span>Latest Promotion</span><strong>Campaign title from Campaigns</strong></div>
                ) : null}
              </div>
            </div>
            <div className="wallet-preview-divider"><span>Information</span></div>
            <div className="wallet-details-sheet">
              <PreviewItem label="About This Programme" value={form.passDescription} />
              <PreviewItem label="Latest Promotion" value="Managed from Campaigns" muted />
              <PreviewItem label="How to Collect Stamps" value={form.stampInstructions} />
              <PreviewItem label="Reward Details" value={form.rewardDetails} />
              <PreviewItem label="Rewards Earned" value="Calculated for each member" muted />
              <PreviewItem label="Stamps Until Next Reward" value="Calculated for each member" muted />
              {links.map((link) => <PreviewItem key={link.link_type} label={link.label} value={link.url} link />)}
              <PreviewItem label="Questions? Contact the Merchant" value={form.supportText} />
              <PreviewItem label="Terms and Conditions" value={form.termsText} />
              <PreviewItem label="Powered by Just Loyalty" value="www.just-loyalty.com" link />
            </div>
          </aside>
        </div>
      ) : null}
    </div>
  );
}

function PassFormSection({ title, description, children }) {
  return <section className="pass-form-section"><header><h2>{title}</h2><p>{description}</p></header><div className="pass-form-grid">{children}</div></section>;
}

function PassField({ label, wide, children }) {
  return <label className={wide ? "wide" : ""}><span>{label}</span>{children}</label>;
}

function PreviewItem({ label, value, muted, link }) {
  return (
    <div className={`wallet-preview-item ${muted ? "muted" : ""}`}>
      <span>{label}</span>
      <p>{value || "Not provided"}{link && value ? <ExternalLink size={13} /> : null}</p>
    </div>
  );
}

function formFromDesign(design) {
  const links = Object.fromEntries((design?.links ?? []).map((link) => [link.link_type, link.url]));
  return {
    programName: design?.program_name ?? "",
    passDescription: design?.pass_description ?? "",
    stampGoal: design?.stamp_goal ?? 10,
    rewardTitle: design?.reward_title ?? "Loyalty Reward",
    stampInstructions: design?.stamp_instructions ?? "",
    rewardDetails: design?.reward_details ?? "",
    supportText: design?.support_text ?? "",
    termsText: design?.terms_text ?? "",
    websiteURL: links.website ?? "",
    instagramURL: links.instagram ?? "",
    whatsappURL: links.whatsapp ?? "",
    showCampaignOnFront: design?.campaign_display_mode === "front_and_details",
  };
}

function linksFromForm(form) {
  return [
    { link_type: "website", label: "Website", url: form.websiteURL.trim() },
    { link_type: "instagram", label: "Instagram", url: form.instagramURL.trim() },
    { link_type: "whatsapp", label: "WhatsApp", url: form.whatsappURL.trim() },
  ].filter((link) => link.url);
}

function publishMessage(payload) {
  const passCount = payload.updated_pass_count ?? 0;
  const pushCount = payload.push_sent_count ?? 0;
  if (pushCount > 0) return `Published version ${payload.design.version}. Sent ${pushCount} Wallet refresh push${pushCount === 1 ? "" : "es"}.`;
  if (passCount > 0) return `Published version ${payload.design.version}. Updated ${passCount} pass${passCount === 1 ? "" : "es"}; no registered devices required a push.`;
  return `Published version ${payload.design.version}. New passes will use these details.`;
}
