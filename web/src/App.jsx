import React, { useEffect, useMemo, useState } from "react";
import {
  Activity,
  Bell,
  CalendarDays,
  ChartNoAxesCombined,
  ChevronRight,
  CircleHelp,
  Coffee,
  Download,
  Gauge,
  Home,
  Layers3,
  Mail,
  Megaphone,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  QrCode,
  Search,
  Settings,
  Sparkles,
  Ticket,
  UserPlus,
  Users,
  WalletCards,
} from "lucide-react";
import { API_BASE, API_VERSION_PREFIX, DEFAULT_MERCHANT, MERCHANT_SESSION_KEY } from "./config/api";

const navItems = [
  { label: "Dashboard", icon: Home, key: "dashboard" },
  { label: "Passes", icon: WalletCards },
  { label: "Customers", icon: Users, key: "customers" },
  { label: "Campaigns", icon: Ticket },
  { label: "Analytics", icon: ChartNoAxesCombined },
  { label: "Integrations", icon: Layers3 },
  { label: "Settings", icon: Settings },
];

const customerTabs = [
  { key: "members", label: "Members", icon: Users },
  { key: "registration", label: "Customer Registration", icon: UserPlus },
  { key: "engagement", label: "Customer Engagement", icon: Megaphone },
];

const activityBars = [42, 56, 48, 68, 86, 72, 80];
const trendLabels = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

export default function App() {
  const [session, setSession] = useState(null);
  const [activeView, setActiveView] = useState("dashboard");
  const [sidebarExpanded, setSidebarExpanded] = useState(false);
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

  function logout() {
    window.localStorage.removeItem(MERCHANT_SESSION_KEY);
    window.location.href = "/login";
  }

  if (!session) {
    return null;
  }

  return (
    <div className={`app-shell ${sidebarExpanded ? "sidebar-open" : ""}`}>
      <Sidebar
        activeView={activeView}
        expanded={sidebarExpanded}
        onToggle={() => setSidebarExpanded((value) => !value)}
        onNavigate={(key) => {
          if (key) {
            setActiveView(key);
          }
        }}
      />

      <main className="workspace">
        <Topbar activeView={activeView} onNavigate={setActiveView} session={session} onLogout={logout} />

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
            onRegister={registerMember}
          />
        ) : (
          <DashboardContent />
        )}
      </main>
    </div>
  );
}

function Sidebar({ activeView, expanded, onToggle, onNavigate }) {
  return (
    <aside className={`sidebar ${expanded ? "expanded" : ""}`} aria-label="Main navigation">
      <div className="sidebar-header">
        <div className="brand-mark">
          <Coffee size={23} strokeWidth={2.2} />
        </div>
        {expanded ? <strong>Pass Console</strong> : null}
      </div>
      <button
        className="nav-item sidebar-toggle"
        type="button"
        onClick={onToggle}
        aria-label={expanded ? "Collapse sidebar" : "Expand sidebar"}
        title={expanded ? "Collapse sidebar" : "Expand sidebar"}
      >
        {expanded ? <PanelLeftClose size={20} /> : <PanelLeftOpen size={20} />}
        {expanded ? <span>Collapse</span> : null}
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
            {expanded ? <span>{item.label}</span> : null}
          </button>
        ))}
      </nav>
      <button className="nav-item support" aria-label="Help" title="Help" type="button">
        <CircleHelp size={20} />
        {expanded ? <span>Help</span> : null}
      </button>
    </aside>
  );
}

function Topbar({ activeView, onNavigate, session, onLogout }) {
  return (
    <header className="topbar">
      <div className="top-tabs">
        <button className={`tab ${activeView === "dashboard" ? "active" : ""}`} onClick={() => onNavigate("dashboard")}>
          <Gauge size={17} />
          Dashboard
        </button>
        <button className={`tab ${activeView === "customers" ? "active" : ""}`} onClick={() => onNavigate("customers")}>
          <Users size={17} />
          Customers
        </button>
        <button className="tab">
          <Activity size={17} />
          Reports
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
        <button className="pill-control" type="button" onClick={onLogout}>
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

function DashboardContent() {
  return (
    <>
      <PageHeading current="Dashboard" title="Pass Management Console" />

      <section className="metrics-grid" aria-label="Dashboard summary">
        <MetricCard label="Total passes issued" value="1,284" delta="+12.8%" icon={WalletCards} />
        <MetricCard label="Active customers" value="842" delta="+8.4%" icon={Users} />
        <MetricCard label="Rewards redeemed" value="126" delta="+5.1%" icon={Sparkles} />
        <MetricCard label="Scan events" value="3,918" delta="+18.2%" icon={QrCode} />
      </section>

      <section className="dashboard-grid">
        <ActivityPanel />
        <ConsumerPanel />
      </section>
    </>
  );
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

function PassGenerator({ registration, setRegistration, previewUrl, passLinks, status, error, onRegister }) {
  const activePreviewUrl = passLinks?.preview_url ?? previewUrl;
  const passUrl = passLinks?.pass_url ?? "";
  const createdMember = passLinks?.member;

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
        <form
          className="generator-form"
          onSubmit={(event) => {
            event.preventDefault();
            onRegister();
          }}
        >
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
          {createdMember ? (
            <div className="registration-result">
              <span>Member ID</span>
              <strong>{createdMember.member_id}</strong>
              <em>{createdMember.status} · {createdMember.stamp_progress} stamps</em>
            </div>
          ) : null}
        </form>

        <div className="pass-preview-shell">
          <iframe title="Apple Wallet pass preview" src={activePreviewUrl} />
        </div>
      </div>

      <div className="generator-actions">
        <a className="secondary-action" href={activePreviewUrl} target="_blank" rel="noreferrer">
          <WalletCards size={17} />
          Open Preview
        </a>
        <a className={`secondary-action ${passUrl ? "" : "disabled"}`} href={passUrl || undefined} target="_blank" rel="noreferrer">
          <Download size={17} />
          Download .pkpass
        </a>
      </div>
    </article>
  );
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
                <td>{member.member_id}</td>
                <td>{member.customer_name}</td>
                <td>{member.customer_email}</td>
                <td>{member.stamp_progress} / 10</td>
                <td><span className="status-pill">{member.status}</span></td>
                <td>{formatDate(member.creation_date)}</td>
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

function ActivityPanel() {
  return (
    <article className="activity-panel">
      <div className="panel-heading tight">
        <div>
          <span className="eyebrow">Activity</span>
          <h2>Consumer trends</h2>
        </div>
        <button className="round-control small" aria-label="Activity filters" title="Activity filters">
          <Activity size={16} />
        </button>
      </div>
      <div className="bars" aria-label="Weekly activity trend">
        {activityBars.map((height, index) => (
          <div className="bar-column" key={trendLabels[index]}>
            <span style={{ height: `${height}%` }} className={index === 4 ? "highlight" : ""} />
            <em>{trendLabels[index]}</em>
          </div>
        ))}
      </div>
    </article>
  );
}

function ConsumerPanel() {
  return (
    <article className="consumer-panel">
      <div className="panel-heading tight">
        <div>
          <span className="eyebrow">Segments</span>
          <h2>Consumer activity</h2>
        </div>
      </div>
      <div className="donut" aria-label="Consumer segment split">
        <span>68%</span>
      </div>
      <div className="segment-stats">
        <span><strong>480</strong> New</span>
        <span><strong>312</strong> Returning</span>
        <span><strong>50</strong> Dormant</span>
      </div>
    </article>
  );
}
