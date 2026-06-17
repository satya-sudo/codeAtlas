import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "../styles.css";

const CONFIG = {
  authBaseUrl: "http://localhost:8061",
  repoBaseUrl: "http://localhost:8062",
  tokenStorageKey: "codeatlas.auth.token"
};

const mockData = {
  overview: [
    { label: "total commits", value: "18.4k", trend: "+6.2% sync-over-sync" },
    { label: "contributors", value: "42", trend: "11 active this week" },
    { label: "files tracked", value: "1,284", trend: "62 hot this month" },
    { label: "modules", value: "14", trend: "3 high-risk ownership zones" }
  ],
  hotspots: [
    { path: "src/payment/service.go", commits: 530, churn: "+12.4k / -8.1k", risk: "Critical" },
    { path: "src/billing/invoice.go", commits: 412, churn: "+9.6k / -6.4k", risk: "High" },
    { path: "src/auth/session.go", commits: 278, churn: "+5.3k / -3.2k", risk: "Medium" },
    { path: "src/notifications/router.go", commits: 203, churn: "+4.1k / -2.7k", risk: "Medium" }
  ],
  ownership: [
    { module: "payment", owner: "satyam", share: 62, backup: "john", risk: "bus factor 1" },
    { module: "billing", owner: "maria", share: 51, backup: "alex", risk: "bus factor 2" },
    { module: "auth", owner: "satyam", share: 47, backup: "denise", risk: "knowledge spreading" }
  ],
  expertise: [
    { engineer: "satyam", module: "payment", score: 87, insight: "Highest recency-weighted ownership and review depth." },
    { engineer: "maria", module: "billing", score: 81, insight: "Strong depth across invoice and reconciliation surfaces." },
    { engineer: "denise", module: "auth", score: 73, insight: "Fastest recent growth in security and session changes." }
  ],
  activity: [
    { title: "Repository onboarding pending", detail: "Install the GitHub App or connect a repository to replace sample insights with live data." },
    { title: "Auth verified", detail: "GitHub OAuth and Postgres-backed user persistence are working." },
    { title: "Analytics pipeline next", detail: "Hotspots, ownership, expertise, and graph panels are ready for repo-service data." }
  ],
  graph: {
    nodes: [
      { id: "dev-satyam", label: "satyam", type: "developer", x: 94, y: 74 },
      { id: "dev-maria", label: "maria", type: "developer", x: 94, y: 224 },
      { id: "mod-payment", label: "payment", type: "module", x: 302, y: 84 },
      { id: "mod-billing", label: "billing", type: "module", x: 302, y: 230 },
      { id: "file-service", label: "service.go", type: "file", x: 540, y: 54 },
      { id: "file-invoice", label: "invoice.go", type: "file", x: 540, y: 164 },
      { id: "file-ledger", label: "ledger.go", type: "file", x: 540, y: 274 }
    ],
    edges: [
      ["dev-satyam", "mod-payment"],
      ["dev-maria", "mod-billing"],
      ["mod-payment", "file-service"],
      ["mod-billing", "file-invoice"],
      ["mod-billing", "file-ledger"],
      ["file-service", "file-invoice"]
    ]
  }
};

function App() {
  const [activeSection, setActiveSection] = useState("overview");
  const [token, setToken] = useState(() => localStorage.getItem(CONFIG.tokenStorageKey) || "");
  const [user, setUser] = useState(null);
  const [authError, setAuthError] = useState("");
  const [toast, setToast] = useState("");

  useEffect(() => {
    const currentUrl = new URL(window.location.href);
    const redirectToken = currentUrl.searchParams.get("token");
    const redirectError = currentUrl.searchParams.get("error");

    if (redirectToken) {
      localStorage.setItem(CONFIG.tokenStorageKey, redirectToken);
      setToken(redirectToken);
      setToast("GitHub connected.");
      currentUrl.searchParams.delete("token");
      window.history.replaceState({}, "", currentUrl.pathname || "/");
    }

    if (redirectError) {
      setAuthError(redirectError);
      currentUrl.searchParams.delete("error");
      window.history.replaceState({}, "", currentUrl.pathname || "/");
    }
  }, []);

  useEffect(() => {
    if (!token) {
      setUser(null);
      return;
    }

    let cancelled = false;

    async function loadCurrentUser() {
      try {
        const response = await fetch(`${CONFIG.authBaseUrl}/auth/me`, {
          headers: {
            Authorization: `Bearer ${token}`
          }
        });

        if (!response.ok) {
          throw new Error(`auth check failed with status ${response.status}`);
        }

        const payload = await response.json();
        if (!cancelled) {
          setUser(payload.user);
          setAuthError("");
        }
      } catch (error) {
        console.error(error);
        if (!cancelled) {
          setUser(null);
          setAuthError("Session could not be verified against auth-service.");
        }
      }
    }

    loadCurrentUser();

    return () => {
      cancelled = true;
    };
  }, [token]);

  useEffect(() => {
    if (!toast) {
      return undefined;
    }

    const timer = window.setTimeout(() => {
      setToast("");
    }, 2800);

    return () => window.clearTimeout(timer);
  }, [toast]);

  const navItems = useMemo(
    () => ["overview", "hotspots", "ownership", "expertise", "graph", "activity"],
    []
  );

  const handleLogout = () => {
    localStorage.removeItem(CONFIG.tokenStorageKey);
    setToken("");
    setUser(null);
    setAuthError("");
    setToast("Session cleared.");
  };

  const focusSection = (section) => {
    setActiveSection(section);
    requestAnimationFrame(() => {
      document.querySelector(`[data-panel="${section}"]`)?.scrollIntoView({
        behavior: "smooth",
        block: "start"
      });
    });
  };

  return (
    <>
      <div className="shell">
        <Sidebar
          activeSection={activeSection}
          authError={authError}
          navItems={navItems}
          onSelect={focusSection}
          user={user}
        />
        <main className="main">
          <Hero onFocusGraph={() => focusSection("graph")} onLogout={handleLogout} user={user} />
          <section className="content-grid">
            <OverviewPanel items={mockData.overview} />
            <HotspotsPanel items={mockData.hotspots} />
            <OwnershipPanel items={mockData.ownership} />
            <ExpertisePanel items={mockData.expertise} />
            <GraphPanel graph={mockData.graph} />
            <ActivityPanel items={mockData.activity} />
          </section>
        </main>
      </div>
      <div className={`toast ${toast ? "is-visible" : ""}`}>{toast}</div>
    </>
  );
}

function Sidebar({ activeSection, authError, navItems, onSelect, user }) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-mark">CA</div>
        <div className="brand-copy">
          <h1>CodeAtlas</h1>
          <p className="caption">Repository analytics</p>
        </div>
      </div>

      <div className="sidebar-card">
        <p className="caption">Session</p>
        <div className="status-cluster" style={{ marginTop: 12 }}>
          {user ? (
            <>
              <div className="status-chip">
                <span className="status-dot"></span>
                <span>
                  Signed in as <strong>{user.username}</strong>
                </span>
              </div>
              <p className="muted">GitHub ID {user.github_id}. Auth service is connected.</p>
            </>
          ) : (
            <>
              <div className="status-chip">
                <span
                  className="status-dot"
                  style={{
                    background: "var(--orange)",
                    boxShadow: "0 0 0 8px rgba(255, 157, 77, 0.12)"
                  }}
                ></span>
                <span>Sign in to continue</span>
              </div>
              <p className="muted">
                {authError || "The current page uses sample repository data until repo-service is connected."}
              </p>
            </>
          )}
        </div>
      </div>

      <nav className="nav" aria-label="Primary navigation">
        {navItems.map((item) => (
          <button
            key={item}
            type="button"
            className={activeSection === item ? "is-active" : ""}
            onClick={() => onSelect(item)}
          >
            {capitalize(item)}
          </button>
        ))}
      </nav>

      <div className="sidebar-card">
        <p className="caption">Local ports</p>
        <div className="status-cluster" style={{ marginTop: 14 }}>
          <span className="pill">auth · 8061</span>
          <span className="pill">repo · 8062</span>
          <span className="pill">webhook · 8063</span>
          <span className="pill">frontend · 6060</span>
        </div>
      </div>

      <div className="sidebar-card">
        <p className="caption">Current state</p>
        <p className="muted" style={{ marginTop: 12 }}>
          Auth works. Repository onboarding and real analytics are the next backend integrations for this frontend.
        </p>
      </div>
    </aside>
  );
}

function Hero({ onFocusGraph, onLogout, user }) {
  return (
    <section className="hero" id="overview">
      <div className="hero-grid">
        <div>
          <span className="pill">CodeAtlas v1</span>
          <h2>Understand repository ownership, hotspots, and change patterns.</h2>
          <p>
            This frontend is a simple shell for the backend you are building. GitHub login is live, the layout is ready for repository data, and the current cards show the shape of the product without overcomplicating the UI.
          </p>
          <div className="hero-actions">
            <a className="button button-primary" href={`${CONFIG.authBaseUrl}/auth/github/login`}>
              Sign in with GitHub
            </a>
            <button type="button" className="button button-secondary" onClick={onFocusGraph}>
              View graph section
            </button>
            {user ? (
              <button type="button" className="button button-danger" onClick={onLogout}>
                Sign out
              </button>
            ) : null}
          </div>
          {user ? (
            <div className="repo-card">
              <div>
                <span className="tag">authenticated user</span>
                <strong>{user.username}</strong>
                <p className="muted">Login is working. The next step is connecting repositories through repo-service.</p>
              </div>
              <div className="mono">github:{user.github_id}</div>
            </div>
          ) : (
            <div className="empty-state">
              The login button redirects through auth-service and returns here with a token. Once repo-service is ready, these sample cards can be replaced with live repository data.
            </div>
          )}
        </div>

        <div className="hero-stack">
          <HeroMetric
            label="auth"
            value="live"
            description="GitHub OAuth, local user persistence, and token verification are working."
          />
          <HeroMetric
            label="repo flow"
            value="next"
            description="Repository connect, backfill, and webhook setup are the next backend milestones."
          />
          <HeroMetric
            label="frontend"
            value="ready"
            description={
              <>
                The layout is in React now, with a simpler structure that is easier to evolve as real APIs arrive.
              </>
            }
          />
        </div>
      </div>
    </section>
  );
}

function HeroMetric({ description, label, value }) {
  return (
    <div className="hero-metric">
      <p className="caption">{label}</p>
      <strong>{value}</strong>
      <p className="muted">{description}</p>
    </div>
  );
}

function OverviewPanel({ items }) {
  return (
    <section className="panel span-12" data-panel="overview">
      <div className="panel-header">
        <div>
          <p className="caption">Repository overview</p>
          <h3>Key numbers</h3>
          <p className="muted">These cards will be replaced with real repository metrics once onboarding is connected.</p>
        </div>
        <span className="pill">sample data</span>
      </div>
      <div className="overview-grid">
        {items.map((item) => (
          <article className="stat-card" key={item.label}>
            <span className="stat-label">{item.label}</span>
            <strong>{item.value}</strong>
            <span className="trend">{item.trend}</span>
          </article>
        ))}
      </div>
    </section>
  );
}

function HotspotsPanel({ items }) {
  return (
    <section className="panel span-7" data-panel="hotspots">
      <div className="panel-header">
        <div>
          <p className="caption">Hotspots</p>
          <h3>Most frequently changed files</h3>
        </div>
        <span className="pill">commit count + churn</span>
      </div>
      <table className="table">
        <thead>
          <tr>
            <th>Path</th>
            <th>Commits</th>
            <th>Churn</th>
            <th>Risk</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.path}>
              <td className="mono">{item.path}</td>
              <td>{item.commits}</td>
              <td>{item.churn}</td>
              <td>{item.risk}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function OwnershipPanel({ items }) {
  return (
    <section className="panel span-5" data-panel="ownership">
      <div className="panel-header">
        <div>
          <p className="caption">Ownership</p>
          <h3>Module ownership</h3>
        </div>
        <span className="pill">bus factor preview</span>
      </div>
      <div className="ownership-list">
        {items.map((item) => (
          <article className="ownership-card" key={item.module}>
            <div className="row">
              <div>
                <strong>{item.module}</strong>
                <p className="muted">Primary {item.owner} · backup {item.backup}</p>
              </div>
              <span className="tag">{item.risk}</span>
            </div>
            <div className="meter">
              <span style={{ width: `${item.share}%` }}></span>
            </div>
            <p className="footnote" style={{ marginTop: 10 }}>
              {item.owner} currently owns {item.share}% of recent change weight.
            </p>
          </article>
        ))}
      </div>
    </section>
  );
}

function ExpertisePanel({ items }) {
  return (
    <section className="panel span-4" data-panel="expertise">
      <div className="panel-header">
        <div>
          <p className="caption">Expertise map</p>
          <h3>Suggested reviewers</h3>
        </div>
      </div>
      <div className="expert-list">
        {items.map((item) => (
          <article className="expert-card" key={`${item.engineer}-${item.module}`}>
            <p className="caption">{item.module}</p>
            <strong>{item.engineer}</strong>
            <div className="expert-score">{item.score}</div>
            <p className="muted">{item.insight}</p>
          </article>
        ))}
      </div>
    </section>
  );
}

function GraphPanel({ graph }) {
  return (
    <section className="panel span-8" data-panel="graph">
      <div className="panel-header">
        <div>
          <p className="caption">Knowledge graph</p>
          <h3>Developer, module, and file links</h3>
        </div>
        <span className="pill">svg preview</span>
      </div>
      <div className="graph-wrap">
        <GraphSvg graph={graph} />
      </div>
      <div className="legend">
        <span>
          <i style={{ background: "var(--teal)" }}></i> Developers
        </span>
        <span>
          <i style={{ background: "var(--orange)" }}></i> Modules
        </span>
        <span>
          <i style={{ background: "var(--gold)" }}></i> Files
        </span>
      </div>
    </section>
  );
}

function GraphSvg({ graph }) {
  const nodeMap = new Map(graph.nodes.map((node) => [node.id, node]));

  return (
    <svg viewBox="0 0 640 320" role="img" aria-label="Knowledge graph preview">
      {graph.edges.map(([fromId, toId]) => {
        const from = nodeMap.get(fromId);
        const to = nodeMap.get(toId);
        return (
          <line
            key={`${fromId}-${toId}`}
            x1={from.x}
            y1={from.y}
            x2={to.x}
            y2={to.y}
            stroke="rgba(255,255,255,0.18)"
            strokeWidth="2"
          />
        );
      })}
      {graph.nodes.map((node) => (
        <g key={node.id}>
          <circle
            cx={node.x}
            cy={node.y}
            r={node.type === "file" ? 22 : 28}
            fill={node.type === "developer" ? "var(--teal)" : node.type === "module" ? "var(--orange)" : "var(--gold)"}
            opacity="0.95"
          ></circle>
          <text
            x={node.x}
            y={node.y + 4}
            textAnchor="middle"
            fill="#061017"
            fontFamily="IBM Plex Mono, monospace"
            fontSize="11"
          >
            {node.label}
          </text>
        </g>
      ))}
    </svg>
  );
}

function ActivityPanel({ items }) {
  return (
    <section className="panel span-12" data-panel="activity">
      <div className="panel-header">
        <div>
          <p className="caption">Next steps</p>
          <h3>What still needs backend data</h3>
        </div>
      </div>
      <div className="activity-list">
        {items.map((item) => (
          <article className="activity-card" key={item.title}>
            <strong>{item.title}</strong>
            <p className="muted">{item.detail}</p>
          </article>
        ))}
      </div>
    </section>
  );
}

function capitalize(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

createRoot(document.getElementById("app")).render(<App />);
