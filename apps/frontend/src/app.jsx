import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "../styles.css";

const CONFIG = {
  authBaseUrl: "http://localhost:8061",
  repoBaseUrl: "http://localhost:8062",
  tokenStorageKey: "codeatlas.auth.token"
};

const futureHotspots = [
  { path: "/auth/session.go", level: "High churn" },
  { path: "/db/schema.sql", level: "High churn" },
  { path: "/api/v1/user.ts", level: "Medium activity" }
];

const futureCoverage = [
  { label: "Core Engine", value: 72, tone: "primary" },
  { label: "UI Components", value: 45, tone: "secondary" }
];

function App() {
  const [token, setToken] = useState(() => localStorage.getItem(CONFIG.tokenStorageKey) || "");
  const [user, setUser] = useState(null);
  const [authError, setAuthError] = useState("");
  const [toast, setToast] = useState("");
  const [installationState, setInstallationState] = useState({
    installationId: "",
    setupAction: "",
    status: "idle",
    error: "",
    claimedInstallation: null
  });

  useEffect(() => {
    const currentUrl = new URL(window.location.href);
    const redirectToken = currentUrl.searchParams.get("token");
    const redirectError = currentUrl.searchParams.get("error");
    const installationId = currentUrl.searchParams.get("installation_id");
    const setupAction = currentUrl.searchParams.get("setup_action");

    if (redirectToken) {
      localStorage.setItem(CONFIG.tokenStorageKey, redirectToken);
      setToken(redirectToken);
      setToast("GitHub sign-in completed.");
    }

    if (redirectError) {
      setAuthError(decodeURIComponent(redirectError));
    }

    if (installationId) {
      setInstallationState((current) => ({
        ...current,
        installationId,
        setupAction: setupAction || "",
        status: "pending-claim",
        error: ""
      }));
      setToast("GitHub App returned. Finishing installation link.");
    }

    if (redirectToken || redirectError || installationId || setupAction) {
      currentUrl.searchParams.delete("token");
      currentUrl.searchParams.delete("error");
      currentUrl.searchParams.delete("installation_id");
      currentUrl.searchParams.delete("setup_action");
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

        if (response.status === 401) {
          localStorage.removeItem(CONFIG.tokenStorageKey);
          if (!cancelled) {
            setToken("");
            setUser(null);
            setAuthError("Session expired. Sign in again.");
          }
          return;
        }

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
          setAuthError("Could not verify the current session.");
        }
      }
    }

    loadCurrentUser();

    return () => {
      cancelled = true;
    };
  }, [token]);

  useEffect(() => {
    if (!token || !installationState.installationId || installationState.status === "claimed") {
      return;
    }

    if (installationState.status === "claiming") {
      return;
    }

    let cancelled = false;

    async function claimInstallation() {
      setInstallationState((current) => ({
        ...current,
        status: "claiming",
        error: ""
      }));

      try {
        const response = await fetch(`${CONFIG.repoBaseUrl}/integrations/github/installations/claim`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`
          },
          body: JSON.stringify({
            installation_id: Number(installationState.installationId)
          })
        });

        if (response.status === 401) {
          localStorage.removeItem(CONFIG.tokenStorageKey);
          if (!cancelled) {
            setToken("");
            setUser(null);
            setAuthError("Session expired before installation claim. Sign in again.");
            setInstallationState((current) => ({
              ...current,
              status: "failed",
              error: "Installation returned, but your session was no longer valid."
            }));
          }
          return;
        }

        if (!response.ok) {
          throw new Error(`claim installation failed with status ${response.status}`);
        }

        const payload = await response.json();
        if (!cancelled) {
          setInstallationState((current) => ({
            ...current,
            status: "claimed",
            claimedInstallation: payload.installation,
            error: ""
          }));
          setToast("GitHub App installation linked.");
        }
      } catch (error) {
        console.error(error);
        if (!cancelled) {
          setInstallationState((current) => ({
            ...current,
            status: "failed",
            error: "Could not claim the GitHub App installation."
          }));
        }
      }
    }

    claimInstallation();

    return () => {
      cancelled = true;
    };
  }, [token, installationState.installationId, installationState.status]);

  useEffect(() => {
    if (!toast) {
      return undefined;
    }

    const timer = window.setTimeout(() => {
      setToast("");
    }, 2800);

    return () => window.clearTimeout(timer);
  }, [toast]);

  const installStatus = useMemo(() => getInstallationStatusCopy(installationState), [installationState]);

  const handleLogout = () => {
    localStorage.removeItem(CONFIG.tokenStorageKey);
    setToken("");
    setUser(null);
    setAuthError("");
    setInstallationState({
      installationId: "",
      setupAction: "",
      status: "idle",
      error: "",
      claimedInstallation: null
    });
    setToast("Signed out.");
  };

  const handleInstallGitHubApp = async () => {
    if (!token) {
      setAuthError("Sign in before installing the GitHub App.");
      return;
    }

    setInstallationState((current) => ({
      ...current,
      status: "requesting-install-url",
      error: ""
    }));

    try {
      const response = await fetch(`${CONFIG.repoBaseUrl}/integrations/github/install`, {
        headers: {
          Authorization: `Bearer ${token}`
        }
      });

      if (response.status === 401) {
        localStorage.removeItem(CONFIG.tokenStorageKey);
        setToken("");
        setUser(null);
        setAuthError("Session expired. Sign in again before installing the GitHub App.");
        return;
      }

      if (!response.ok) {
        throw new Error(`installation url request failed with status ${response.status}`);
      }

      const payload = await response.json();
      window.location.assign(payload.install_url);
    } catch (error) {
      console.error(error);
      setInstallationState((current) => ({
        ...current,
        status: "failed",
        error: "Could not start the GitHub App installation flow."
      }));
    }
  };

  if (!user) {
    return (
      <>
        <div className="login-screen">
          <div className="login-grid" aria-hidden="true" />
          <main className="login-main">
            <section className="login-card">
              <div className="login-brand">
                <div className="login-brand-mark">CA</div>
                <span>CodeAtlas</span>
              </div>

              <header className="login-header">
                <h1>Sign in to CodeAtlas</h1>
                <p>Continue to your engineering intelligence workspace.</p>
              </header>

              <div className="login-actions">
                <a className="button button-primary button-full" href={`${CONFIG.authBaseUrl}/auth/github/login`}>
                  <svg aria-hidden="true" className="github-icon" viewBox="0 0 24 24">
                    <path d="M12 0C5.373 0 0 5.373 0 12c0 5.302 3.438 9.8 8.207 11.387.6.111.793-.261.793-.577V20.59c-3.338.726-4.042-1.416-4.042-1.416-.545-1.387-1.332-1.756-1.332-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.838 1.237 1.838 1.237 1.07 1.834 2.809 1.304 3.493.997.106-.775.418-1.305.762-1.604-2.666-.304-5.467-1.334-5.467-5.93 0-1.312.469-2.382 1.236-3.221-.124-.303-.536-1.524.117-3.176 0 0 1.008-.322 3.301 1.23A11.49 11.49 0 0 1 12 5.803c1.02.005 2.047.138 3.006.404 2.291-1.552 3.298-1.23 3.298-1.23.653 1.653.241 2.874.118 3.176.77.839 1.235 1.909 1.235 3.221 0 4.607-2.804 5.625-5.475 5.921.43.371.823 1.103.823 2.222v3.293c0 .319.192.689.801.576C20.565 21.798 24 17.302 24 12 24 5.373 18.627 0 12 0Z" />
                  </svg>
                  <span>Sign in with GitHub</span>
                </a>

                {authError ? <p className="login-error">{authError}</p> : null}
              </div>

              <footer className="login-footer">
                <div className="login-links">
                  <a href="/">Privacy Policy</a>
                  <span />
                  <a href="/">Terms of Service</a>
                </div>
                <p>© 2024 CodeAtlas Intelligence Inc.</p>
              </footer>
            </section>
          </main>
        </div>

        <div className={`toast ${toast ? "is-visible" : ""}`}>{toast}</div>
      </>
    );
  }

  return (
    <>
      <div className="onboarding-page">
        <header className="onboarding-topbar">
          <div className="topbar-brand">
            <div className="brand-copy">
              <strong>CodeAtlas</strong>
              <span>Engineering Intelligence</span>
            </div>
          </div>

          <div className="topbar-right">
            <div className="system-pill">
              <span className="system-dot" />
              <span>Status: Operational</span>
            </div>

            <div className="user-chip">
              <div className="avatar-shell">
                {user.avatar_url ? <img src={user.avatar_url} alt={user.username} /> : <span>{user.username.slice(0, 1).toUpperCase()}</span>}
              </div>
              <div className="user-chip-copy">
                <strong>{user.username}</strong>
                <button type="button" className="link-button" onClick={handleLogout}>
                  Sign out
                </button>
              </div>
            </div>
          </div>
        </header>

        <main className="onboarding-main">
          <section className="hero-section">
            <h1>Unlock deep codebase intelligence</h1>
            <p>
              Connect your GitHub account and install the CodeAtlas GitHub App to map ownership, identify hotspots, and prepare repository syncing for engineering insights.
            </p>

            <div className="hero-actions">
              <a className="button button-muted button-static" href={`${CONFIG.authBaseUrl}/auth/github/login`} aria-disabled="true">
                Signed in with GitHub
              </a>
              <button type="button" className="button button-primary" onClick={handleInstallGitHubApp}>
                Install GitHub App
              </button>
            </div>
          </section>

          <section className="flow-strip">
            <FlowStep label="Sign in" meta="Done" state="done" />
            <FlowStep label="Install App" meta={installationState.status === "idle" ? "Active" : installationState.status === "requesting-install-url" ? "Starting" : "Done"} state={installationState.status === "idle" || installationState.status === "requesting-install-url" ? "active" : "done"} />
            <FlowStep label="Return" meta={installationState.installationId ? "Done" : "Next"} state={installationState.installationId ? "done" : "idle"} />
            <FlowStep label="Claim" meta={installationState.status === "claimed" ? "Done" : installationState.status === "claiming" ? "Active" : "Wait"} state={installationState.status === "claimed" ? "done" : installationState.status === "claiming" ? "active" : "idle"} />
            <FlowStep label="Sync" meta="Final" state="idle" />
          </section>

          <section className="content-grid">
            <div className="panel setup-panel">
              <div className="panel-head">
                <h2>Setup Progress</h2>
                <span className={`status-badge status-${mapBadgeTone(installationState.status)}`}>
                  {installationState.status === "claimed" ? "LINKED" : installationState.status === "failed" ? "FAILED" : "IN PROGRESS"}
                </span>
              </div>

              <div className="setup-list">
                <SetupRow
                  tone="done"
                  title="Auth Status"
                  detail={`Session active as ${user.username}`}
                />

                <SetupRow
                  tone={installationState.status === "idle" ? "active" : installationState.status === "failed" ? "error" : "done"}
                  title="App Installation"
                  detail={installationState.status === "idle" ? "Install the GitHub App on the repositories or organization you want CodeAtlas to read." : installStatus.detail}
                  action={
                    installationState.status === "idle" ? (
                      <button type="button" className="button button-primary button-small" onClick={handleInstallGitHubApp}>
                        Start Install
                      </button>
                    ) : null
                  }
                />

                <SetupRow
                  tone={installationState.installationId ? "done" : "pending"}
                  title="Installation Link"
                  detail={
                    installationState.installationId
                      ? `Installation ${installationState.installationId} returned from GitHub and is being attached to your account.`
                      : "Waiting for GitHub callback to finalize the mapping process."
                  }
                />
              </div>
            </div>

            <div className="panel detail-panel">
              <div className="panel-head">
                <h2>Installation Details</h2>
              </div>

              <dl className="detail-grid">
                <div>
                  <dt>Target</dt>
                  <dd>Select organization or repositories on GitHub</dd>
                </div>
                <div>
                  <dt>Permissions</dt>
                  <dd>Read-only code and metadata</dd>
                </div>
                <div>
                  <dt>Installation ID</dt>
                  <dd>{installationState.installationId || "Not returned yet"}</dd>
                </div>
                <div>
                  <dt>Setup action</dt>
                  <dd>{installationState.setupAction || "Waiting for callback"}</dd>
                </div>
                <div>
                  <dt>Linked user</dt>
                  <dd>
                    {installationState.claimedInstallation?.installed_by_user_id
                      ? `User ${installationState.claimedInstallation.installed_by_user_id}`
                      : "Not linked yet"}
                  </dd>
                </div>
                <div>
                  <dt>System status</dt>
                  <dd className={`detail-status tone-${mapBadgeTone(installationState.status)}`}>{installStatus.label}</dd>
                </div>
              </dl>

              {installationState.error ? <p className="inline-error">{installationState.error}</p> : null}
              {authError ? <p className="inline-error">{authError}</p> : null}
            </div>
          </section>

          <section className="preview-section">
            <div className="preview-header">
              <h3>Post-setup preview</h3>
              <span className="preview-pill">Coming soon</span>
            </div>

            <div className="preview-grid">
              <article className="panel preview-card">
                <div className="preview-card-head">
                  <span>Repository Health</span>
                </div>
                <div className="spark-stack">
                  <SparkRow label="Churn rate" values={[40, 60, 30, 50, 45]} tone="primary" tag="Low" />
                  <SparkRow label="Complexity" values={[20, 25, 22, 24, 23]} tone="muted" tag="Stable" />
                </div>
              </article>

              <article className="panel preview-card">
                <div className="preview-card-head">
                  <span>Hotspot Files</span>
                </div>
                <ul className="preview-list">
                  {futureHotspots.map((item) => (
                    <li key={item.path}>
                      <span className="mono">{item.path}</span>
                      <em>{item.level}</em>
                    </li>
                  ))}
                </ul>
              </article>

              <article className="panel preview-card">
                <div className="preview-card-head">
                  <span>Expertise Map</span>
                </div>
                <div className="coverage-list">
                  {futureCoverage.map((item) => (
                    <div className="coverage-row" key={item.label}>
                      <div className="coverage-meta">
                        <span>{item.label}</span>
                        <strong>{item.value}% Coverage</strong>
                      </div>
                      <div className="coverage-track">
                        <span className={`coverage-fill tone-${item.tone}`} style={{ width: `${item.value}%` }} />
                      </div>
                    </div>
                  ))}
                </div>
              </article>
            </div>
          </section>
        </main>
      </div>

      <div className={`toast ${toast ? "is-visible" : ""}`}>{toast}</div>
    </>
  );
}

function FlowStep({ label, meta, state }) {
  return (
    <div className={`flow-step state-${state}`}>
      <div className="flow-dot">
        {state === "done" ? "✓" : label === "Install App" && state === "active" ? "2" : label === "Sign in" ? "1" : label === "Return" ? "3" : label === "Claim" ? "4" : "5"}
      </div>
      <div className="flow-copy">
        <strong>{label}</strong>
        <span>{meta}</span>
      </div>
    </div>
  );
}

function SetupRow({ action, detail, title, tone }) {
  return (
    <div className={`setup-row tone-${tone}`}>
      <div className="setup-icon" />
      <div className="setup-copy">
        <strong>{title}</strong>
        <p>{detail}</p>
      </div>
      {action ? <div className="setup-action">{action}</div> : null}
    </div>
  );
}

function SparkRow({ label, values, tone, tag }) {
  return (
    <div className="spark-row">
      <div className="spark-head">
        <span>{label}</span>
        <strong>{tag}</strong>
      </div>
      <div className="spark-bars">
        {values.map((value, index) => (
          <i
            key={`${label}-${index}`}
            className={`spark-bar tone-${tone}`}
            style={{ height: `${value}%` }}
          />
        ))}
      </div>
    </div>
  );
}

function getInstallationStatusCopy(state) {
  switch (state.status) {
    case "requesting-install-url":
      return {
        label: "Starting install",
        detail: "Repo-service is returning the GitHub App installation URL."
      };
    case "pending-claim":
      return {
        label: "Returned from GitHub",
        detail: `Installation ${state.installationId} came back from GitHub and is waiting to be claimed.`
      };
    case "claiming":
      return {
        label: "Claiming installation",
        detail: `Frontend is linking installation ${state.installationId} to the logged-in user.`
      };
    case "claimed":
      return {
        label: "Linked",
        detail: `Installation ${state.installationId} is now attached to your CodeAtlas account.`
      };
    case "failed":
      return {
        label: "Failed",
        detail: state.error || "The installation flow did not complete successfully."
      };
    default:
      return {
        label: "Ready to link",
        detail: "No GitHub App installation has been started yet."
      };
  }
}

function mapBadgeTone(status) {
  switch (status) {
    case "claimed":
      return "success";
    case "failed":
      return "danger";
    case "idle":
      return "primary";
    default:
      return "warning";
  }
}

createRoot(document.getElementById("app")).render(<App />);
