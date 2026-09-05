import React, { useEffect, useMemo, useRef, useState } from "react";
import jsQR from "jsqr";
import {
  Activity,
  Bell,
  CalendarDays,
  ChartNoAxesCombined,
  ChevronRight,
  CircleHelp,
  Download,
  Gauge,
  Home,
  Layers3,
  Mail,
  Megaphone,
  Menu,
  Plus,
  QrCode,
  Search,
  Send,
  Settings,
  Sparkles,
  Ticket,
  UserPlus,
  Users,
  WalletCards,
  X,
} from "lucide-react";
import BrandLogo from "./components/BrandLogo.jsx";
import { API_BASE, API_VERSION_PREFIX, DEFAULT_MERCHANT, MERCHANT_SESSION_KEY } from "./config/api";

const navItems = [
  { label: "Dashboard", icon: Home, key: "dashboard" },
  { label: "Passes", icon: WalletCards },
  { label: "Customers", icon: Users, key: "customers" },
  { label: "Campaigns", icon: Ticket, key: "campaigns" },
  { label: "Analytics", icon: ChartNoAxesCombined },
  { label: "Integrations", icon: Layers3 },
  { label: "Settings", icon: Settings },
];

const customerTabs = [
  { key: "members", label: "Members", icon: Users },
  { key: "registration", label: "Customer Registration", icon: UserPlus },
  { key: "scanner", label: "Stamp Scanner", icon: QrCode },
  { key: "engagement", label: "Customer Engagement", icon: Megaphone },
];

const STAMP_GOAL = 10;

export default function App() {
  const [session, setSession] = useState(null);
  const [activeView, setActiveView] = useState("dashboard");
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [customerTab, setCustomerTab] = useState("members");
  const [registration, setRegistration] = useState({
    name: "Taylor Example",
    email: "taylor@example.com",
    merchant: DEFAULT_MERCHANT,
  });
  const [members, setMembers] = useState([]);
  const [membersStatus, setMembersStatus] = useState("idle");
  const [membersError, setMembersError] = useState("");
  const [passLinks, setPassLinks] = useState(null);
  const [status, setStatus] = useState("idle");
  const [error, setError] = useState("");
  const [campaigns, setCampaigns] = useState([]);
  const [campaignsStatus, setCampaignsStatus] = useState("idle");
  const [campaignsError, setCampaignsError] = useState("");
  const [campaignForm, setCampaignForm] = useState({
    title: "",
    message: "",
  });
  const [campaignCreateStatus, setCampaignCreateStatus] = useState("idle");
  const [campaignCreateMessage, setCampaignCreateMessage] = useState("");

  useEffect(() => {
    const rawSession = window.localStorage.getItem(MERCHANT_SESSION_KEY);
    if (!rawSession) {
      window.location.href = "/login";
      return;
    }

    try {
      const parsedSession = JSON.parse(rawSession);
      if (!parsedSession?.token || !parsedSession?.merchant?.merchant) {
        window.localStorage.removeItem(MERCHANT_SESSION_KEY);
        window.location.href = "/login";
        return;
      }
      setSession(parsedSession);
      setRegistration((current) => ({
        ...current,
        merchant: parsedSession.merchant.merchant,
      }));
    } catch {
      window.localStorage.removeItem(MERCHANT_SESSION_KEY);
      window.location.href = "/login";
    }
  }, []);

  useEffect(() => {
    if (!session?.token) {
      return;
    }
    loadMembers(session.token);
    loadCampaigns(session.token);
  }, [session]);

  const previewUrl = useMemo(() => {
    const safeId = encodeURIComponent(passLinks?.member?.member_id ?? "member-1001");
    return `${API_BASE}${API_VERSION_PREFIX}/apple/passes/${safeId}/preview`;
  }, [passLinks]);

  async function registerMember() {
    setStatus("loading");
    setError("");
    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/customers/members/register`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(registration),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error ?? `Registration endpoint returned ${response.status}`);
      }
      const data = await response.json();
      setPassLinks(data);
      setStatus("ready");
      await loadMembers(session?.token);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to generate preview");
      setStatus("error");
    }
  }

  async function loadMembers(token) {
    if (!token) {
      return;
    }
    setMembersStatus("loading");
    setMembersError("");
    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/customers/members`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error ?? `Members endpoint returned ${response.status}`);
      }
      const data = await response.json();
      setMembers(data.members ?? []);
      setMembersStatus("ready");
    } catch (err) {
      setMembersError(err instanceof Error ? err.message : "Unable to load members");
      setMembersStatus("error");
      if (err instanceof Error && err.message === "unauthorized") {
        window.localStorage.removeItem(MERCHANT_SESSION_KEY);
        window.location.href = "/login";
      }
    }
  }

  async function loadCampaigns(token) {
    if (!token) {
      return;
    }
    setCampaignsStatus("loading");
    setCampaignsError("");
    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/campaigns`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error ?? `Campaigns endpoint returned ${response.status}`);
      }
      const data = await response.json();
      setCampaigns(data.campaigns ?? []);
      setCampaignsStatus("ready");
    } catch (err) {
      setCampaignsError(err instanceof Error ? err.message : "Unable to load campaigns");
      setCampaignsStatus("error");
      if (err instanceof Error && err.message === "unauthorized") {
        window.localStorage.removeItem(MERCHANT_SESSION_KEY);
        window.location.href = "/login";
      }
    }
  }

  async function createCampaign(event) {
    event.preventDefault();
    if (!session?.token) {
      return;
    }

    setCampaignCreateStatus("loading");
    setCampaignCreateMessage("");
    try {
      const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/campaigns`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${session.token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(campaignForm),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error ?? `Campaign creation returned ${response.status}`);
      }
      const data = await response.json();
      setCampaigns((current) => [data.campaign, ...current]);
      setCampaignForm({ title: "", message: "" });
      setCampaignCreateStatus("ready");
      setCampaignCreateMessage(
        `Queued ${data.delivery_count ?? 0} customers. Sent ${data.push_sent_count ?? 0} Wallet update pushes.`
      );
      await loadMembers(session.token);
    } catch (err) {
      setCampaignCreateStatus("error");
      setCampaignCreateMessage(err instanceof Error ? err.message : "Unable to create campaign");
    }
  }

  async function updateMemberStamps(memberID, stampDelta = 1) {
    if (!session?.token) {
      throw new Error("Login session is required");
    }

    const response = await fetch(`${API_BASE}${API_VERSION_PREFIX}/customers/members/${encodeURIComponent(memberID)}/stamps`, {
      method: "PUT",
      headers: {
        Authorization: `Bearer ${session.token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ stamp_delta: stampDelta }),
    });
    if (!response.ok) {
      const payload = await response.json().catch(() => null);
      throw new Error(payload?.error ?? `Stamp endpoint returned ${response.status}`);
    }

    const data = await response.json();
    await loadMembers(session.token);
    return data;
  }

  function logout() {
    window.localStorage.removeItem(MERCHANT_SESSION_KEY);
    window.location.href = "/login";
  }

  if (!session) {
    return null;
  }

  return (
    <div className={`app-shell ${mobileNavOpen ? "mobile-nav-open" : ""}`}>
      <Sidebar
        activeView={activeView}
        mobileOpen={mobileNavOpen}
        onClose={() => setMobileNavOpen(false)}
        onNavigate={(key) => {
          if (key) {
            setActiveView(key);
            setMobileNavOpen(false);
          }
        }}
      />
      {mobileNavOpen ? <button className="mobile-nav-backdrop" type="button" aria-label="Close navigation" onClick={() => setMobileNavOpen(false)} /> : null}

      <main className="workspace">
        <Topbar activeView={activeView} onNavigate={setActiveView} session={session} onLogout={logout} onOpenMenu={() => setMobileNavOpen(true)} />

        {activeView === "customers" ? (
          <CustomersContent
            customerTab={customerTab}
            setCustomerTab={setCustomerTab}
            registration={registration}
            setRegistration={setRegistration}
            previewUrl={previewUrl}
            passLinks={passLinks}
            status={status}
            error={error}
            members={members}
            membersStatus={membersStatus}
            membersError={membersError}
            session={session}
            onRegister={registerMember}
            onUpdateStamps={updateMemberStamps}
          />
        ) : activeView === "campaigns" ? (
          <CampaignsContent
            campaigns={campaigns}
            status={campaignsStatus}
            error={campaignsError}
            form={campaignForm}
            setForm={setCampaignForm}
            createStatus={campaignCreateStatus}
            createMessage={campaignCreateMessage}
            members={members}
            onCreate={createCampaign}
          />
        ) : (
          <DashboardContent members={members} membersStatus={membersStatus} membersError={membersError} />
        )}
      </main>
    </div>
  );
}

function Sidebar({ activeView, mobileOpen, onClose, onNavigate }) {
  return (
    <aside className={`sidebar ${mobileOpen ? "mobile-open" : ""}`} aria-label="Main navigation">
      <div className="sidebar-header">
        <BrandLogo variant="icon" className="sidebar-brand-icon" alt="" />
        <strong>Just Loyalty</strong>
      </div>
      <button
        className="icon-button sidebar-close"
        type="button"
        onClick={onClose}
        aria-label="Close navigation"
        title="Close navigation"
      >
        <X size={20} />
      </button>
      <nav className="icon-nav">
        {navItems.map((item) => (
          <button
            className={`nav-item ${item.key === activeView ? "active" : ""}`}
            key={item.label}
            aria-label={item.label}
            title={item.label}
            type="button"
            onClick={() => onNavigate(item.key)}
          >
            <item.icon size={20} />
            <span>{item.label}</span>
          </button>
        ))}
      </nav>
      <button className="nav-item support" aria-label="Help" title="Help" type="button">
        <CircleHelp size={20} />
        <span>Help</span>
      </button>
    </aside>
  );
}

function Topbar({ activeView, onNavigate, session, onLogout, onOpenMenu }) {
  return (
    <header className="topbar">
      <button className="round-control mobile-menu-button" type="button" onClick={onOpenMenu} aria-label="Open navigation" title="Open navigation">
        <Menu size={19} />
      </button>
      <div className="top-tabs">
        <button className={`tab ${activeView === "dashboard" ? "active" : ""}`} onClick={() => onNavigate("dashboard")}>
          <Gauge size={17} />
          Dashboard
        </button>
        <button className={`tab ${activeView === "customers" ? "active" : ""}`} onClick={() => onNavigate("customers")}>
          <Users size={17} />
          Customers
        </button>
        <button className={`tab ${activeView === "campaigns" ? "active" : ""}`} onClick={() => onNavigate("campaigns")}>
          <Ticket size={17} />
          Campaigns
        </button>
      </div>
      <div className="top-actions">
        <div className="team-stack" aria-label="Workspace users">
          <span>GC</span>
          <span>DB</span>
          <span>LT</span>
        </div>
        <button className="pill-control">
          <Plus size={17} />
          Add Manager
        </button>
        <button className="round-control" aria-label="Notifications" title="Notifications">
          <Bell size={18} />
        </button>
        <button className="round-control" aria-label="Messages" title="Messages">
          <Mail size={18} />
        </button>
        <button className="pill-control logout-action" type="button" onClick={onLogout}>
          Logout
        </button>
        <div className="avatar" aria-label="Admin profile">{merchantInitials(session?.merchant)}</div>
      </div>
    </header>
  );
}

function merchantInitials(merchant) {
  const first = merchant?.first_name?.[0] ?? "";
  const last = merchant?.last_name?.[0] ?? "";
  const initials = `${first}${last}`.toUpperCase();
  return initials || "SL";
}

function PageHeading({ current, title, actionLabel = "New Campaign" }) {
  return (
    <section className="page-heading">
      <div>
        <div className="breadcrumb">
          <span>Home</span>
          <ChevronRight size={14} />
          <span>{current}</span>
        </div>
        <h1>{title}</h1>
      </div>
      <div className="heading-actions">
        <button className="round-control" aria-label="Search" title="Search">
          <Search size={18} />
        </button>
        <button className="pill-control">
          <CalendarDays size={17} />
          Today
        </button>
        <button className="pill-control primary">
          <Plus size={17} />
          {actionLabel}
        </button>
      </div>
    </section>
  );
}

function DashboardContent({ members, membersStatus, membersError }) {
  const insights = dashboardInsights(members);

  return (
    <>
      <PageHeading current="Dashboard" title="Pass Management Console" />

      <section className="metrics-grid" aria-label="Dashboard summary">
        <MetricCard label="Total passes issued" value={formatNumber(insights.totalPasses)} delta="Merchant members" icon={WalletCards} />
        <MetricCard label="Active customers" value={formatNumber(insights.activeCustomers)} delta={formatShare(insights.activeCustomers, insights.totalPasses)} icon={Users} />
        <MetricCard label="Reward ready" value={formatNumber(insights.rewardReady)} delta={formatShare(insights.rewardReady, insights.totalPasses)} icon={Sparkles} />
        <MetricCard label="Average stamp progress" value={`${insights.averageStampProgress} / ${STAMP_GOAL}`} delta="Across active records" icon={QrCode} />
      </section>

      <section className="dashboard-grid">
        <ActivityPanel trend={insights.registrationTrend} status={membersStatus} error={membersError} />
        <ConsumerPanel segments={insights.statusSegments} totalMembers={insights.totalPasses} activeShare={insights.activeShare} status={membersStatus} error={membersError} />
      </section>
    </>
  );
}

function dashboardInsights(members) {
  const totalPasses = members.length;
  const activeCustomers = members.filter((member) => member.status === "active").length;
  const rewardReady = members.filter((member) => member.status === "reward_ready" || Number(member.stamp_progress) >= STAMP_GOAL).length;
  const activeInProgress = members.filter((member) => member.status === "active" && Number(member.stamp_progress) < STAMP_GOAL).length;
  const totalStampProgress = members.reduce((sum, member) => sum + Number(member.stamp_progress || 0), 0);
  const averageStampProgress = totalPasses === 0 ? 0 : Math.round((totalStampProgress / totalPasses) * 10) / 10;
  const statusSegments = [
    { label: "Active", value: activeInProgress },
    { label: "Reward ready", value: rewardReady },
    { label: "Other", value: Math.max(totalPasses - activeInProgress - rewardReady, 0) },
  ];

  return {
    totalPasses,
    activeCustomers,
    rewardReady,
    averageStampProgress,
    registrationTrend: registrationTrend(members),
    statusSegments,
    activeShare: totalPasses === 0 ? 0 : Math.round((activeCustomers / totalPasses) * 100),
  };
}

function registrationTrend(members) {
  const today = new Date();
  today.setHours(0, 0, 0, 0);

  const days = Array.from({ length: 7 }, (_, index) => {
    const date = new Date(today);
    date.setDate(today.getDate() - (6 - index));
    return {
      date,
      label: date.toLocaleDateString(undefined, { weekday: "short" }),
      value: 0,
    };
  });

  members.forEach((member) => {
    const createdAt = parseMemberDate(member.creation_date);
    if (!createdAt) {
      return;
    }
    createdAt.setHours(0, 0, 0, 0);
    const day = days.find((item) => item.date.getTime() === createdAt.getTime());
    if (day) {
      day.value += 1;
    }
  });

  const maxValue = Math.max(...days.map((day) => day.value), 1);
  return days.map((day) => ({
    ...day,
    height: Math.max((day.value / maxValue) * 100, day.value > 0 ? 12 : 0),
  }));
}

function parseMemberDate(value) {
  if (!value) {
    return null;
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

function formatNumber(value) {
  return new Intl.NumberFormat().format(value);
}

function formatShare(value, total) {
  if (total === 0) {
    return "No records yet";
  }
  return `${Math.round((value / total) * 100)}% of members`;
}

function CustomersContent(props) {
  const activeTab = customerTabs.find((tab) => tab.key === props.customerTab) ?? customerTabs[0];

  return (
    <>
      <PageHeading current="Customers" title="Customers" actionLabel="Add Customer" />

      <section className="customer-shell">
        <div className="section-tabs" role="tablist" aria-label="Customer sections">
          {customerTabs.map((tab) => (
            <button
              className={`section-tab ${tab.key === props.customerTab ? "active" : ""}`}
              key={tab.key}
              type="button"
              onClick={() => props.setCustomerTab(tab.key)}
            >
              <tab.icon size={17} />
              {tab.label}
            </button>
          ))}
        </div>

        <div className="customer-content">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">Customers</span>
              <h2>{activeTab.label}</h2>
            </div>
          </div>
          {props.customerTab === "members" ? (
            <MembersTable members={props.members} status={props.membersStatus} error={props.membersError} />
          ) : null}
          {props.customerTab === "registration" ? <PassGenerator {...props} /> : null}
          {props.customerTab === "scanner" ? <StampScanner onUpdateStamps={props.onUpdateStamps} /> : null}
          {props.customerTab === "engagement" ? <CustomerEngagement /> : null}
        </div>
      </section>
    </>
  );
}

function MetricCard({ label, value, delta, icon: Icon }) {
  return (
    <article className="metric-card">
      <div className="metric-icon">
        <Icon size={19} />
      </div>
      <span>{label}</span>
      <strong>{value}</strong>
      <em>{delta}</em>
    </article>
  );
}

function PassGenerator({ registration, setRegistration, previewUrl, passLinks, status, error, session, onRegister }) {
  const activePreviewUrl = passLinks?.preview_url ?? previewUrl;
  const passUrl = passLinks?.pass_url ?? "";
  const createdMember = passLinks?.member;
  const signupLink = merchantSignupLink(session?.merchant);

  function updateRegistration(field, value) {
    setRegistration((current) => ({ ...current, [field]: value }));
  }

  return (
    <article className="generator-panel">
      <div className="panel-heading">
        <div>
          <span className="eyebrow">Apple Wallet</span>
          <h2>Register customer</h2>
        </div>
        <button className="icon-button compact" aria-label="Pass actions" title="Pass actions">
          <Menu size={18} />
        </button>
      </div>

      <div className="generator-body">
        <section className="registration-workspace">
          <form
            className="generator-form"
            onSubmit={(event) => {
              event.preventDefault();
              onRegister();
            }}
          >
            <div className="merchant-link-box">
              <span>Customer signup link</span>
              <a href={signupLink} target="_blank" rel="noreferrer">{signupLink}</a>
            </div>
            <div className="form-stack">
              <label htmlFor="customerName">Name</label>
              <input
                id="customerName"
                value={registration.name}
                onChange={(event) => updateRegistration("name", event.target.value)}
              />
              <label htmlFor="customerEmail">Email</label>
              <input
                id="customerEmail"
                type="email"
                value={registration.email}
                onChange={(event) => updateRegistration("email", event.target.value)}
              />
              <label htmlFor="customerMerchant">Merchant</label>
              <input
                id="customerMerchant"
                value={registration.merchant}
                onChange={(event) => updateRegistration("merchant", event.target.value)}
                readOnly
              />
              <button className="primary-action" type="submit" disabled={status === "loading"}>
                <UserPlus size={18} />
                {status === "loading" ? "Registering" : "Register & Generate Pass"}
              </button>
            </div>
            {error ? <p className="form-error">{error}</p> : null}
          </form>

          <aside className="registration-summary" aria-label="Generated pass details">
            <div>
              <span className="summary-kicker">Pass output</span>
              <h3>{createdMember ? "Customer pass is ready" : "Generate a customer pass"}</h3>
              <p>
                {createdMember
                  ? "Use the links below to open the browser preview or download the signed Apple Wallet pass."
                  : "Register a customer to create their member record and wallet pass links."}
              </p>
            </div>

            {createdMember ? (
              <div className="registration-result">
                <span>Member ID</span>
                <strong>{createdMember.member_id}</strong>
                <em>{createdMember.status} · {createdMember.stamp_progress} stamps</em>
              </div>
            ) : (
              <div className="registration-empty-state">
                <WalletCards size={22} />
                <span>No pass generated yet</span>
              </div>
            )}

            <div className="generator-actions">
              <a className={`secondary-action ${createdMember ? "" : "disabled"}`} href={createdMember ? activePreviewUrl : undefined} target="_blank" rel="noreferrer">
                <WalletCards size={17} />
                Open Preview
              </a>
              <a className={`secondary-action ${passUrl ? "" : "disabled"}`} href={passUrl || undefined} target="_blank" rel="noreferrer">
                <Download size={17} />
                Download .pkpass
              </a>
            </div>
          </aside>
        </section>
      </div>
    </article>
  );
}

function merchantSignupLink(merchant) {
  const slug = merchant?.merchant_slug || slugify(merchant?.merchant || DEFAULT_MERCHANT);
  return `${window.location.origin}/m/${encodeURIComponent(slug)}/join`;
}

function CampaignsContent({ campaigns, status, error, form, setForm, createStatus, createMessage, members, onCreate }) {
  const activeMembers = members.filter((member) => member.status === "active").length;
  const latestCampaign = campaigns[0];

  function updateForm(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  return (
    <>
      <PageHeading current="Campaigns" title="Campaigns" actionLabel="Create Campaign" />

      <section className="campaign-shell">
        <div className="campaign-layout">
          <form className="campaign-form-panel" onSubmit={onCreate}>
            <div className="panel-heading tight">
              <div>
                <span className="eyebrow">Wallet push</span>
                <h2>Create promotion</h2>
              </div>
              <div className="campaign-icon">
                <Megaphone size={19} />
              </div>
            </div>
            <label htmlFor="campaignTitle">Campaign title</label>
            <input
              id="campaignTitle"
              placeholder="Double stamps weekend"
              value={form.title}
              onChange={(event) => updateForm("title", event.target.value)}
              required
            />
            <label htmlFor="campaignMessage">Push message</label>
            <textarea
              id="campaignMessage"
              placeholder="Earn double stamps this weekend when you visit any outlet."
              value={form.message}
              onChange={(event) => updateForm("message", event.target.value)}
              rows={5}
              required
            />
            <button className="primary-action" type="submit" disabled={createStatus === "loading"}>
              <Send size={18} />
              {createStatus === "loading" ? "Creating campaign" : "Create & Send Update"}
            </button>
            {createMessage ? <p className={`campaign-message ${createStatus === "error" ? "error" : ""}`}>{createMessage}</p> : null}
          </form>

          <aside className="campaign-summary-panel">
            <span className="summary-kicker">Audience</span>
            <h3>{formatNumber(activeMembers)} active customers</h3>
            <p>
              Campaigns target the logged-in merchant's customers. Customers with an Apple Wallet registration receive a pass update push; others remain queued until the pass is pulled again.
            </p>
            <div className="campaign-summary-stats">
              <span><strong>{formatNumber(campaigns.length)}</strong> campaigns</span>
              <span><strong>{latestCampaign ? formatDate(latestCampaign.creation_date) : "-"}</strong> latest</span>
            </div>
          </aside>
        </div>

        <div className="campaign-list-panel">
          <div className="panel-heading tight">
            <div>
              <span className="eyebrow">History</span>
              <h2>Recent campaigns</h2>
            </div>
          </div>
          {status === "loading" ? <p className="panel-state compact">Loading campaigns</p> : null}
          {error ? <p className="panel-state compact error">{error}</p> : null}
          {status !== "loading" && !error && campaigns.length === 0 ? (
            <p className="panel-state compact">No campaigns created yet.</p>
          ) : null}
          <div className="campaign-list">
            {campaigns.map((campaign) => (
              <article className="campaign-card" key={campaign.campaign_id}>
                <div>
                  <span className="status-pill">{campaign.status}</span>
                  <h3>{campaign.title}</h3>
                  <p>{campaign.message}</p>
                </div>
                <dl className="campaign-meta">
                  <div>
                    <dt>Campaign ID</dt>
                    <dd>{campaign.campaign_id}</dd>
                  </div>
                  <div>
                    <dt>Created</dt>
                    <dd>{formatDate(campaign.creation_date)}</dd>
                  </div>
                </dl>
              </article>
            ))}
          </div>
        </div>
      </section>
    </>
  );
}

function slugify(value) {
  return String(value ?? "merchant")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "") || "merchant";
}

function MembersTable({ members, status, error }) {
  const activeMembers = members.filter((member) => member.status === "active").length;
  const rewardReadyMembers = members.filter((member) => member.status === "reward_ready").length;

  return (
    <div className="members-panel">
      <div className="table-summary">
        <MetricCard label="Onboarded users" value={String(members.length)} delta="Scoped" icon={Users} />
        <MetricCard label="Active members" value={String(activeMembers)} delta="Merchant only" icon={WalletCards} />
        <MetricCard label="Reward ready" value={String(rewardReadyMembers)} delta="Live data" icon={Sparkles} />
      </div>
      <div className="table-wrap">
        {status === "loading" ? <p className="table-state">Loading members</p> : null}
        {error ? <p className="table-state error">{error}</p> : null}
        <table>
          <thead>
            <tr>
              <th>Customer ID</th>
              <th>Name</th>
              <th>Email</th>
              <th>Stamp progress</th>
              <th>Status</th>
              <th>Joined</th>
            </tr>
          </thead>
          <tbody>
            {members.map((member) => (
              <tr key={member.member_id}>
                <td data-label="Customer ID">{member.member_id}</td>
                <td data-label="Name">{member.customer_name}</td>
                <td data-label="Email">{member.customer_email}</td>
                <td data-label="Stamp progress">{member.stamp_progress} / 10</td>
                <td data-label="Status"><span className="status-pill">{member.status}</span></td>
                <td data-label="Joined">{formatDate(member.creation_date)}</td>
              </tr>
            ))}
            {status !== "loading" && members.length === 0 ? (
              <tr>
                <td colSpan={6}>No members registered for this merchant yet.</td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function StampScanner({ onUpdateStamps }) {
  const videoRef = useRef(null);
  const canvasRef = useRef(null);
  const streamRef = useRef(null);
  const scanTimerRef = useRef(null);
  const detectorRef = useRef(null);
  const detectInProgressRef = useRef(false);
  const stampUpdateInProgressRef = useRef(false);
  const [memberID, setMemberID] = useState("");
  const [cameraStatus, setCameraStatus] = useState("idle");
  const [status, setStatus] = useState("idle");
  const [message, setMessage] = useState("");
  const [lastUpdate, setLastUpdate] = useState(null);

  useEffect(() => {
    return () => stopCamera();
  }, []);

  async function startCamera() {
    setMessage("");
    setStatus("idle");
    setCameraStatus("idle");
    if (!navigator.mediaDevices?.getUserMedia) {
      setCameraStatus("error");
      setMessage("Camera access is not available in this browser. Use HTTPS on your phone, or enter the member ID manually.");
      return;
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: {
          facingMode: { ideal: "environment" },
          width: { ideal: 1280 },
          height: { ideal: 720 },
        },
        audio: false,
      });
      streamRef.current = stream;
      if (videoRef.current) {
        videoRef.current.srcObject = stream;
        await videoRef.current.play();
      }
      setCameraStatus("scanning");
      setMessage("Point the camera at the customer's Apple Wallet QR code.");
      scanTimerRef.current = window.setInterval(scanFrame, 700);
      await scanFrame();
    } catch (err) {
      setCameraStatus("error");
      setMessage(cameraErrorMessage(err));
    }
  }

  function stopCamera() {
    if (scanTimerRef.current) {
      window.clearInterval(scanTimerRef.current);
      scanTimerRef.current = null;
    }
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
    }
    if (videoRef.current) {
      videoRef.current.srcObject = null;
    }
    setCameraStatus((current) => current === "scanning" ? "idle" : current);
  }

  async function scanFrame() {
    const video = videoRef.current;
    const canvas = canvasRef.current;
    if (!video || !canvas || video.readyState < 2 || detectInProgressRef.current || stampUpdateInProgressRef.current) {
      return;
    }

    detectInProgressRef.current = true;
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const context = canvas.getContext("2d", { willReadFrequently: true });
    if (!context) {
      detectInProgressRef.current = false;
      return;
    }
    context.drawImage(video, 0, 0, canvas.width, canvas.height);

    try {
      const rawValue = await readQRCode(canvas, context);
      if (!rawValue) {
        return;
      }
      const scannedID = memberIDFromQRCode(rawValue);
      if (scannedID) {
        setMemberID(scannedID);
        stopCamera();
        await updateStampForMember(scannedID, "scan");
      } else {
        setMessage("QR code scanned, but no member ID was found.");
      }
    } catch {
      setMessage("Unable to read this camera frame.");
    } finally {
      detectInProgressRef.current = false;
    }
  }

  async function addStamp(event) {
    event.preventDefault();
    await updateStampForMember(memberID, "manual");
  }

  async function updateStampForMember(value, source) {
    const safeMemberID = normalizeMemberIDInput(value);
    if (!safeMemberID) {
      setMessage("Scan or enter a member ID first.");
      return;
    }
    if (stampUpdateInProgressRef.current) {
      return;
    }

    stampUpdateInProgressRef.current = true;
    setStatus("loading");
    setMessage(source === "scan" ? `Scanned ${safeMemberID}. Updating stamp progress...` : "");
    try {
      const result = await onUpdateStamps(safeMemberID, 1);
      setLastUpdate(result);
      setStatus("ready");
      setMessage(`Added 1 stamp for ${result.member.customer_name}.${walletUpdateMessage(result.wallet_update)}`);
    } catch (err) {
      setStatus("error");
      setMessage(err instanceof Error ? err.message : "Unable to update stamp progress");
    } finally {
      stampUpdateInProgressRef.current = false;
    }
  }

  async function readQRCode(canvas, context) {
    if ("BarcodeDetector" in window) {
      try {
        detectorRef.current ??= new window.BarcodeDetector({ formats: ["qr_code"] });
        const codes = await detectorRef.current.detect(canvas);
        if (codes.length > 0) {
          return codes[0].rawValue;
        }
      } catch {
        detectorRef.current = null;
      }
    }

    const imageData = context.getImageData(0, 0, canvas.width, canvas.height);
    const result = jsQR(imageData.data, imageData.width, imageData.height, {
      inversionAttempts: "attemptBoth",
    });
    return result?.data ?? "";
  }

  return (
    <article className="scanner-panel">
      <div className="scanner-grid">
        <section className="scanner-camera">
          <div className="panel-heading tight">
            <div>
              <span className="eyebrow">Wallet QR</span>
              <h2>Scan customer pass</h2>
            </div>
          </div>
          <div className="camera-frame">
            <video ref={videoRef} autoPlay muted playsInline />
            <canvas ref={canvasRef} hidden />
            {cameraStatus !== "scanning" ? (
              <div className="camera-placeholder">
                <QrCode size={42} />
                <span>Camera preview</span>
              </div>
            ) : null}
          </div>
          <div className="generator-actions scanner-actions">
            <button className="secondary-action" type="button" onClick={startCamera} disabled={cameraStatus === "scanning"}>
              <QrCode size={17} />
              Start Scanner
            </button>
            <button className="secondary-action" type="button" onClick={stopCamera}>
              Stop
            </button>
          </div>
        </section>

        <form className="scanner-form" onSubmit={addStamp}>
          <div className="panel-heading tight">
            <div>
              <span className="eyebrow">Stamp update</span>
              <h2>Add one stamp</h2>
            </div>
          </div>
          <label htmlFor="scannerMemberID">Member ID</label>
          <input
            id="scannerMemberID"
            placeholder="MBR-ABC12345"
            value={memberID}
            onChange={(event) => setMemberID(normalizeMemberIDInput(event.target.value))}
          />
          <button className="primary-action" type="submit" disabled={status === "loading"}>
            <Plus size={18} />
            {status === "loading" ? "Adding Stamp" : "Add Stamp"}
          </button>
          {message ? <p className={`scanner-message ${status === "error" || (cameraStatus === "error" && status !== "ready") ? "error" : ""}`}>{message}</p> : null}
          {lastUpdate ? (
            <div className="stamp-result">
              <span>{lastUpdate.member.member_id}</span>
              <strong>{lastUpdate.event.new_stamp_progress} / 10 stamps</strong>
              <em>{lastUpdate.member.status}</em>
            </div>
          ) : null}
        </form>
      </div>
    </article>
  );
}

function memberIDFromQRCode(value) {
  const text = String(value ?? "").trim();
  const stampPayload = text.match(/^stamp-card:(MBR-[A-Z0-9]+)$/i);
  if (stampPayload) {
    return stampPayload[1].toUpperCase();
  }
  const memberID = text.match(/MBR-[A-Z0-9]+/i);
  return memberID ? memberID[0].toUpperCase() : "";
}

function normalizeMemberIDInput(value) {
  return String(value ?? "").trim().toUpperCase();
}

function walletUpdateMessage(walletUpdate) {
  if (!walletUpdate) {
    return "";
  }
  if (walletUpdate.push_sent_count > 0) {
    return ` Wallet refresh pushed to ${walletUpdate.push_sent_count} device${walletUpdate.push_sent_count === 1 ? "" : "s"}.`;
  }
  if (walletUpdate.registered_pass_count === 0) {
    return " No Apple Wallet device is registered yet.";
  }
  if (walletUpdate.last_error) {
    return " Wallet refresh could not be pushed.";
  }
  return "";
}

function cameraErrorMessage(err) {
  if (!(err instanceof Error)) {
    return "Unable to start camera.";
  }
  if (err.name === "NotAllowedError") {
    return "Camera permission was denied. Allow camera access in the browser and try again.";
  }
  if (err.name === "NotFoundError") {
    return "No camera was found on this device.";
  }
  if (err.name === "NotReadableError") {
    return "The camera is already in use by another app or browser tab.";
  }
  if (window.location.protocol !== "https:" && window.location.hostname !== "localhost" && window.location.hostname !== "127.0.0.1") {
    return "Camera access requires HTTPS on mobile browsers. Open the UAT site over HTTPS and try again.";
  }
  return err.message || "Unable to start camera.";
}

function formatDate(value) {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  }).format(new Date(value));
}

function CustomerEngagement() {
  return (
    <article className="engagement-panel">
      <div className="engagement-card">
        <Megaphone size={28} />
        <h3>Wallet notification triggers</h3>
        <p>
          Next phase: select a customer ID, choose a promotional event, and trigger an iOS Wallet update/notification after the customer has added the pass.
        </p>
      </div>
      <div className="engagement-form">
        <label htmlFor="engagementCustomer">Customer ID</label>
        <input id="engagementCustomer" value="member-1001" readOnly />
        <label htmlFor="engagementEvent">Promotional event</label>
        <input id="engagementEvent" value="Double stamps weekend" readOnly />
        <button className="primary-action" type="button" disabled>
          <Megaphone size={18} />
          Trigger in next phase
        </button>
      </div>
    </article>
  );
}

function ActivityPanel({ trend, status, error }) {
  const hasTrend = trend.some((day) => day.value > 0);

  return (
    <article className="activity-panel">
      <div className="panel-heading tight">
        <div>
          <span className="eyebrow">Activity</span>
          <h2>Customer registrations</h2>
        </div>
        <button className="round-control small" aria-label="Activity filters" title="Activity filters">
          <Activity size={16} />
        </button>
      </div>
      {status === "loading" ? <p className="panel-state">Loading registrations</p> : null}
      {error ? <p className="panel-state error">{error}</p> : null}
      {status !== "loading" && !error && hasTrend ? (
        <div className="bars" aria-label="Last seven days customer registrations">
          {trend.map((day) => (
            <div className="bar-column" key={day.date.toISOString()}>
              <span style={{ height: `${day.height}%` }} className={day.value > 0 ? "highlight" : ""} />
              <strong>{day.value}</strong>
              <em>{day.label}</em>
            </div>
          ))}
        </div>
      ) : null}
      {status !== "loading" && !error && !hasTrend ? (
        <p className="panel-state">No customer registrations in the last 7 days.</p>
      ) : null}
    </article>
  );
}

function ConsumerPanel({ segments, totalMembers, activeShare, status, error }) {
  return (
    <article className="consumer-panel">
      <div className="panel-heading tight">
        <div>
          <span className="eyebrow">Segments</span>
          <h2>Customer status mix</h2>
        </div>
      </div>
      {status === "loading" ? <p className="panel-state">Loading customers</p> : null}
      {error ? <p className="panel-state error">{error}</p> : null}
      {status !== "loading" && !error && totalMembers > 0 ? (
        <>
          <div className="donut" aria-label="Active customer share" style={{ "--active-share": `${activeShare * 3.6}deg` }}>
            <span>{activeShare}%</span>
          </div>
          <div className="segment-stats">
            {segments.map((segment) => (
              <span key={segment.label}><strong>{formatNumber(segment.value)}</strong> {segment.label}</span>
            ))}
          </div>
        </>
      ) : null}
      {status !== "loading" && !error && totalMembers === 0 ? (
        <p className="panel-state">No customer records available yet.</p>
      ) : null}
    </article>
  );
}
