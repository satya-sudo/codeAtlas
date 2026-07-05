import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "../styles.css";

const CONFIG = {
  apiBaseUrl: "http://localhost:8060",
  publicGatewayBaseUrl: "https://pregnancy-fence-childcare.ngrok-free.dev",
  tokenStorageKey: "codeatlas.auth.token"
};

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
  const [installations, setInstallations] = useState([]);
  const [installationsStatus, setInstallationsStatus] = useState("idle");
  const [installationsError, setInstallationsError] = useState("");
  const [selectedInstallationId, setSelectedInstallationId] = useState("");
  const [repositories, setRepositories] = useState([]);
  const [repositoriesStatus, setRepositoriesStatus] = useState("idle");
  const [repositoriesError, setRepositoriesError] = useState("");
  const [repositorySearch, setRepositorySearch] = useState("");
  const [selectedRepositoryId, setSelectedRepositoryId] = useState("");
  const [connectedRepositories, setConnectedRepositories] = useState([]);
  const [connectedReposStatus, setConnectedReposStatus] = useState("idle");
  const [connectedReposError, setConnectedReposError] = useState("");
  const [connectedReposVersion, setConnectedReposVersion] = useState(0);
  const [latestSyncRunsByRepo, setLatestSyncRunsByRepo] = useState({});
  const [syncActionStatusByRepo, setSyncActionStatusByRepo] = useState({});
  const [syncActionErrorByRepo, setSyncActionErrorByRepo] = useState({});
  const [connectStatus, setConnectStatus] = useState("idle");
  const [connectError, setConnectError] = useState("");

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
      setToast("GitHub access granted. Linking it to your workspace.");
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
        const response = await fetch(`${CONFIG.apiBaseUrl}/auth/me`, {
          headers: {
            Authorization: `Bearer ${token}`
          }
        });

        if (response.status === 401) {
          localStorage.removeItem(CONFIG.tokenStorageKey);
          if (!cancelled) {
            resetSession("Session expired. Sign in again.");
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
        const response = await fetch(`${CONFIG.apiBaseUrl}/integrations/github/installations/claim`, {
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
            resetSession("Session expired before access could be linked. Sign in again.");
            setInstallationState((current) => ({
              ...current,
              status: "failed",
              error: "GitHub returned successfully, but your session was no longer valid."
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
          setSelectedInstallationId(String(payload.installation.installation_id));
          setToast("GitHub access linked.");
        }
      } catch (error) {
        console.error(error);
        if (!cancelled) {
          setInstallationState((current) => ({
            ...current,
            status: "failed",
            error: "Could not link the returned GitHub access to your account."
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
    if (!token || !user) {
      setInstallations([]);
      setConnectedRepositories([]);
      return;
    }

    let cancelled = false;

    async function loadInstallations() {
      setInstallationsStatus("loading");
      setInstallationsError("");

      try {
        const response = await fetch(`${CONFIG.apiBaseUrl}/integrations/github/installations`, {
          headers: {
            Authorization: `Bearer ${token}`
          }
        });

        if (response.status === 401) {
          localStorage.removeItem(CONFIG.tokenStorageKey);
          if (!cancelled) {
            resetSession("Session expired. Sign in again.");
          }
          return;
        }

        if (!response.ok) {
          throw new Error(`list installations failed with status ${response.status}`);
        }

        const payload = await response.json();
        if (!cancelled) {
          const items = payload.installations || [];
          setInstallations(items);
          setInstallationsStatus("ready");
          setInstallationsError("");

          setSelectedInstallationId((current) => {
            if (current && items.some((item) => String(item.installation_id) === current)) {
              return current;
            }
            if (installationState.installationId && items.some((item) => String(item.installation_id) === installationState.installationId)) {
              return installationState.installationId;
            }
            return items.length > 0 ? String(items[0].installation_id) : "";
          });
        }
      } catch (error) {
        console.error(error);
        if (!cancelled) {
          setInstallations([]);
          setInstallationsStatus("failed");
          setInstallationsError("Could not load GitHub access yet.");
        }
      }
    }

    loadInstallations();

    return () => {
      cancelled = true;
    };
  }, [token, user, installationState.installationId]);

  useEffect(() => {
    if (!token || !user) {
      setConnectedRepositories([]);
      setConnectedReposStatus("idle");
      setConnectedReposError("");
      return;
    }

    let cancelled = false;

    async function loadConnectedRepositories() {
      setConnectedReposStatus("loading");
      setConnectedReposError("");

      try {
        const response = await fetch(`${CONFIG.apiBaseUrl}/repos`, {
          headers: {
            Authorization: `Bearer ${token}`
          }
        });

        if (response.status === 401) {
          localStorage.removeItem(CONFIG.tokenStorageKey);
          if (!cancelled) {
            resetSession("Session expired. Sign in again.");
          }
          return;
        }

        if (!response.ok) {
          throw new Error(`list connected repositories failed with status ${response.status}`);
        }

        const payload = await response.json();
        if (!cancelled) {
          setConnectedRepositories(payload.repositories || []);
          setConnectedReposStatus("ready");
          setConnectedReposError("");
        }
      } catch (error) {
        console.error(error);
        if (!cancelled) {
          setConnectedRepositories([]);
          setConnectedReposStatus("failed");
          setConnectedReposError("Could not load connected repositories.");
        }
      }
    }

    loadConnectedRepositories();

    return () => {
      cancelled = true;
    };
  }, [token, user, connectedReposVersion]);

  useEffect(() => {
    if (!token || !selectedInstallationId) {
      setRepositories([]);
      setRepositoriesStatus("idle");
      setRepositoriesError("");
      setSelectedRepositoryId("");
      return;
    }

    let cancelled = false;

    async function loadRepositories() {
      setRepositoriesStatus("loading");
      setRepositoriesError("");
      setSelectedRepositoryId("");

      try {
        const response = await fetch(
          `${CONFIG.apiBaseUrl}/integrations/github/installations/${selectedInstallationId}/repositories`,
          {
            headers: {
              Authorization: `Bearer ${token}`
            }
          }
        );

        if (response.status === 401) {
          localStorage.removeItem(CONFIG.tokenStorageKey);
          if (!cancelled) {
            resetSession("Session expired. Sign in again.");
          }
          return;
        }

        if (!response.ok) {
          throw new Error(`list repositories failed with status ${response.status}`);
        }

        const payload = await response.json();
        if (!cancelled) {
          setRepositories(payload.repositories || []);
          setRepositoriesStatus("ready");
          setRepositoriesError("");
        }
      } catch (error) {
        console.error(error);
        if (!cancelled) {
          setRepositories([]);
          setRepositoriesStatus("failed");
          setRepositoriesError("Could not load repositories for this GitHub access.");
        }
      }
    }

    loadRepositories();

    return () => {
      cancelled = true;
    };
  }, [token, selectedInstallationId]);

  useEffect(() => {
    if (!toast) {
      return undefined;
    }

    const timer = window.setTimeout(() => {
      setToast("");
    }, 2800);

    return () => window.clearTimeout(timer);
  }, [toast]);

  const selectedInstallation = useMemo(
    () => installations.find((item) => String(item.installation_id) === selectedInstallationId) || null,
    [installations, selectedInstallationId]
  );

  const filteredRepositories = useMemo(() => {
    const query = repositorySearch.trim().toLowerCase();
    if (!query) {
      return repositories;
    }

    return repositories.filter((item) => {
      const fullName = item.full_name?.toLowerCase() || "";
      const name = item.name?.toLowerCase() || "";
      const owner = item.owner?.login?.toLowerCase() || "";
      return fullName.includes(query) || name.includes(query) || owner.includes(query);
    });
  }, [repositories, repositorySearch]);

  const deduplicatedConnectedRepositories = useMemo(() => {
    const seen = new Set();
    return connectedRepositories.filter((repo) => {
      const key = String(repo.github_repo_id || repo.full_name || repo.id);
      if (seen.has(key)) {
        return false;
      }
      seen.add(key);
      return true;
    });
  }, [connectedRepositories]);

  const selectedRepository = useMemo(
    () => repositories.find((item) => String(item.id) === selectedRepositoryId) || null,
    [repositories, selectedRepositoryId]
  );

  const selectedRepositoryAlreadyConnected = useMemo(() => {
    if (!selectedRepository) {
      return false;
    }

    return deduplicatedConnectedRepositories.some((repo) => repo.github_repo_id === selectedRepository.id);
  }, [selectedRepository, deduplicatedConnectedRepositories]);

  useEffect(() => {
    if (!token || !user || deduplicatedConnectedRepositories.length === 0) {
      setLatestSyncRunsByRepo({});
      return;
    }

    let cancelled = false;

    async function loadLatestSyncRuns() {
      try {
        const results = await Promise.all(
          deduplicatedConnectedRepositories.map(async (repo) => {
            const response = await fetch(`${CONFIG.apiBaseUrl}/repos/${repo.id}/sync-runs`, {
              headers: {
                Authorization: `Bearer ${token}`
              }
            });

            if (response.status === 401) {
              throw new Error("unauthorized");
            }

            if (!response.ok) {
              throw new Error(`sync runs request failed for repository ${repo.id}`);
            }

            const payload = await response.json();
            return [repo.id, payload.sync_runs?.[0] || null];
          })
        );

        if (!cancelled) {
          setLatestSyncRunsByRepo(Object.fromEntries(results));
        }
      } catch (error) {
        console.error(error);
        if (!cancelled && error instanceof Error && error.message === "unauthorized") {
          localStorage.removeItem(CONFIG.tokenStorageKey);
          resetSession("Session expired. Sign in again.");
        }
      }
    }

    loadLatestSyncRuns();

    return () => {
      cancelled = true;
    };
  }, [token, user, deduplicatedConnectedRepositories, connectedReposVersion]);

  const setupSummary = useMemo(
    () => getSetupSummary(user, installations, selectedInstallation, selectedRepository, deduplicatedConnectedRepositories),
    [user, installations, selectedInstallation, selectedRepository, deduplicatedConnectedRepositories]
  );

  function resetSession(message) {
    setToken("");
    setUser(null);
    setAuthError(message);
    setInstallations([]);
    setRepositories([]);
    setConnectedRepositories([]);
    setSelectedInstallationId("");
    setSelectedRepositoryId("");
    setConnectedReposVersion(0);
  }

  const handleLogout = () => {
    localStorage.removeItem(CONFIG.tokenStorageKey);
    setToken("");
    setUser(null);
    setAuthError("");
    setToast("Signed out.");
    setInstallationState({
      installationId: "",
      setupAction: "",
      status: "idle",
      error: "",
      claimedInstallation: null
    });
    setInstallations([]);
    setRepositories([]);
    setConnectedRepositories([]);
    setSelectedInstallationId("");
    setSelectedRepositoryId("");
    setConnectedReposVersion(0);
  };

  const handleInstallGitHubApp = async () => {
    if (!token) {
      setAuthError("Sign in before granting GitHub access.");
      return;
    }

    setInstallationState((current) => ({
      ...current,
      status: "requesting-install-url",
      error: ""
    }));

    try {
      const response = await fetch(`${CONFIG.apiBaseUrl}/integrations/github/install`, {
        headers: {
          Authorization: `Bearer ${token}`
        }
      });

      if (response.status === 401) {
        localStorage.removeItem(CONFIG.tokenStorageKey);
        resetSession("Session expired. Sign in again before granting GitHub access.");
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
        error: "Could not start the GitHub App access flow."
      }));
    }
  };

  const handleRepositoryConnect = async () => {
    if (!selectedRepository || !selectedInstallation || selectedRepositoryAlreadyConnected) {
      return;
    }

    setConnectStatus("loading");
    setConnectError("");

    try {
      const response = await fetch(
        `${CONFIG.apiBaseUrl}/integrations/github/installations/${selectedInstallation.installation_id}/repositories/connect`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`
          },
          body: JSON.stringify({
            github_repo_id: selectedRepository.id
          })
        }
      );

      if (response.status === 401) {
        localStorage.removeItem(CONFIG.tokenStorageKey);
        resetSession("Session expired. Sign in again.");
        return;
      }

      if (!response.ok) {
        throw new Error(`connect repository failed with status ${response.status}`);
      }

      const payload = await response.json();
      setConnectStatus("success");
      setConnectedReposVersion((current) => current + 1);

      if (payload.connection_status === "already_connected") {
        setToast(`${payload.repository.full_name} is already connected.`);
      } else if (payload.connection_status === "updated") {
        setToast(`Updated connection for ${payload.repository.full_name}.`);
      } else {
        setToast(`Connected ${payload.repository.full_name}.`);
      }
    } catch (error) {
      console.error(error);
      setConnectStatus("failed");
      setConnectError("Could not connect the selected repository.");
    }
  };

  const handleQueueSync = async (repo) => {
    setSyncActionStatusByRepo((current) => ({
      ...current,
      [repo.id]: "loading"
    }));
    setSyncActionErrorByRepo((current) => ({
      ...current,
      [repo.id]: ""
    }));

    try {
      const response = await fetch(`${CONFIG.apiBaseUrl}/repos/${repo.id}/sync`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          sync_type: "initial"
        })
      });

      if (response.status === 401) {
        localStorage.removeItem(CONFIG.tokenStorageKey);
        resetSession("Session expired. Sign in again.");
        return;
      }

      if (!response.ok) {
        throw new Error(`queue sync failed with status ${response.status}`);
      }

      const payload = await response.json();
      setLatestSyncRunsByRepo((current) => ({
        ...current,
        [repo.id]: payload.sync_run
      }));
      setSyncActionStatusByRepo((current) => ({
        ...current,
        [repo.id]: "success"
      }));
      setToast(`Queued sync for ${repo.full_name}.`);
    } catch (error) {
      console.error(error);
      setSyncActionStatusByRepo((current) => ({
        ...current,
        [repo.id]: "failed"
      }));
      setSyncActionErrorByRepo((current) => ({
        ...current,
        [repo.id]: "Could not queue sync."
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
                <a className="button button-primary button-full" href={`${CONFIG.publicGatewayBaseUrl}/auth/github/login`}>
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
      <div className="app-shell">
        <header className="topbar">
          <div className="topbar-brand">
            <strong>CodeAtlas</strong>
            <span>Engineering Intelligence</span>
          </div>

          <div className="topbar-user">
            <div className="avatar-shell">
              {user.avatar_url ? <img src={user.avatar_url} alt={user.username} /> : <span>{user.username.slice(0, 1).toUpperCase()}</span>}
            </div>
            <div className="topbar-user-copy">
              <strong>{user.username}</strong>
              <button type="button" className="link-button" onClick={handleLogout}>
                Sign out
              </button>
            </div>
          </div>
        </header>

        <main className="page-body">
          <section className="page-intro">
            <span className="eyebrow">Repository onboarding</span>
            <h1>Connect your first repository</h1>
            <p>Choose the GitHub account or organization you gave CodeAtlas access to, then pick one repository to start syncing.</p>
          </section>

          <section className="summary-strip">
            {setupSummary.map((item) => (
              <div className={`summary-step state-${item.state}`} key={item.label}>
                <span className="summary-dot" />
                <div>
                  <strong>{item.label}</strong>
                  <span>{item.meta}</span>
                </div>
              </div>
            ))}
          </section>

          <section className="workspace-grid">
            <div className="workspace-main">
              <section className="panel">
                <div className="panel-head panel-head-stack">
                  <div>
                    <h2>GitHub access</h2>
                    <p className="panel-subtitle">Each card represents a GitHub account or organization where the CodeAtlas app is installed.</p>
                  </div>
                  <button type="button" className="link-action" onClick={handleInstallGitHubApp}>
                    Grant access to another account
                  </button>
                </div>

                {installationsStatus === "loading" ? <p className="panel-message">Loading GitHub access…</p> : null}
                {installationsError ? <p className="inline-error">{installationsError}</p> : null}

                {installationsStatus === "ready" && installations.length === 0 ? (
                  <div className="empty-state">
                    <strong>No GitHub access connected yet</strong>
                    <p>Install the CodeAtlas GitHub App on a personal account or organization, then come back here to choose a repository.</p>
                    <button type="button" className="button button-primary button-small" onClick={handleInstallGitHubApp}>
                      Install GitHub App
                    </button>
                  </div>
                ) : null}

                {installations.length > 0 ? (
                  <div className="installation-grid">
                    {installations.map((item) => {
                      const isSelected = String(item.installation_id) === selectedInstallationId;
                      return (
                        <button
                          type="button"
                          key={item.installation_id}
                          className={`installation-card ${isSelected ? "is-selected" : ""}`}
                          onClick={() => setSelectedInstallationId(String(item.installation_id))}
                        >
                          <div className="installation-icon">{item.account_type === "Organization" ? "ORG" : "GH"}</div>
                          <div className="installation-copy">
                            <strong>{getInstallationLabel(item)}</strong>
                            <span>{getInstallationDescription(item)}</span>
                            <em>{isSelected ? "Selected" : "Available"}</em>
                          </div>
                        </button>
                      );
                    })}
                  </div>
                ) : null}
              </section>

              <section className="panel">
                <div className="panel-head panel-head-search panel-head-stack-mobile">
                  <div>
                    <h2>Choose repository</h2>
                    <p className="panel-subtitle">We only show repositories that belong to the selected GitHub access.</p>
                  </div>
                  <div className="search-shell">
                    <input
                      type="text"
                      value={repositorySearch}
                      onChange={(event) => setRepositorySearch(event.target.value)}
                      placeholder="Search repositories..."
                    />
                  </div>
                </div>

                {!selectedInstallation ? <p className="panel-message">Choose a GitHub access card first.</p> : null}
                {repositoriesStatus === "loading" ? <p className="panel-message">Loading repositories…</p> : null}
                {repositoriesError ? <p className="inline-error">{repositoriesError}</p> : null}

                {selectedInstallation && repositoriesStatus === "ready" && filteredRepositories.length === 0 ? (
                  <p className="panel-message">No repositories match the current search.</p>
                ) : null}

                {filteredRepositories.length > 0 ? (
                  <div className="repository-list">
                    {filteredRepositories.map((item) => {
                      const isSelected = String(item.id) === selectedRepositoryId;
                      const isAlreadyConnected = connectedRepositories.some((repo) => repo.github_repo_id === item.id);

                      return (
                        <div className={`repository-row ${isSelected ? "is-selected" : ""}`} key={item.id}>
                          <div className="repository-meta">
                            <strong>{item.name}</strong>
                            <p>{item.full_name}</p>
                            <div className="repository-tags">
                              <span>{item.private ? "Private" : "Public"}</span>
                              <span>{item.default_branch || "Unknown branch"}</span>
                              {isAlreadyConnected ? <span className="tag-success">Connected</span> : null}
                            </div>
                          </div>
                          <button
                            type="button"
                            className={`button ${isSelected ? "button-primary" : "button-secondary"} button-small ${isAlreadyConnected ? "is-disabled" : ""}`}
                            onClick={() => setSelectedRepositoryId(String(item.id))}
                          >
                            {isAlreadyConnected ? "Connected" : isSelected ? "Selected" : "Select"}
                          </button>
                        </div>
                      );
                    })}
                  </div>
                ) : null}
              </section>
            </div>

            <aside className="workspace-side">
              <section className="panel">
                <div className="panel-head">
                  <h2>Selection summary</h2>
                </div>

                <dl className="selection-grid">
                  <div>
                    <dt>GitHub access</dt>
                    <dd>{selectedInstallation ? getInstallationLabel(selectedInstallation) : "Not selected"}</dd>
                  </div>
                  <div>
                    <dt>Repository</dt>
                    <dd>{selectedRepository ? selectedRepository.full_name : "Not selected"}</dd>
                  </div>
                  <div>
                    <dt>Already connected</dt>
                    <dd>{deduplicatedConnectedRepositories.length}</dd>
                  </div>
                </dl>

                <button
                  type="button"
                  className={`button button-primary button-full ${selectedRepository && connectStatus !== "loading" && !selectedRepositoryAlreadyConnected ? "" : "is-disabled"}`}
                  disabled={!selectedRepository || connectStatus === "loading" || selectedRepositoryAlreadyConnected}
                  onClick={handleRepositoryConnect}
                >
                  {selectedRepositoryAlreadyConnected ? "Repository already connected" : connectStatus === "loading" ? "Connecting..." : "Connect repository"}
                </button>
                {connectError ? <p className="inline-error connect-error">{connectError}</p> : null}
              </section>

              <section className="panel">
                <div className="panel-head">
                  <h3>Connected repositories</h3>
                  <button type="button" className="link-action" onClick={() => setConnectedReposVersion((current) => current + 1)}>
                    Refresh
                  </button>
                </div>

                {connectedReposStatus === "loading" ? <p className="panel-message">Loading connected repositories…</p> : null}
                {connectedReposError ? <p className="inline-error">{connectedReposError}</p> : null}

                {connectedReposStatus === "ready" && deduplicatedConnectedRepositories.length === 0 ? (
                  <div className="empty-state compact-empty-state">
                    <strong>No repositories connected yet</strong>
                    <p>When you connect one from the left side, it will appear here immediately.</p>
                  </div>
                ) : null}

                {deduplicatedConnectedRepositories.length > 0 ? (
                  <div className="connected-repo-list">
                    {deduplicatedConnectedRepositories.map((repo) => (
                      <div className="connected-repo-card" key={repo.id}>
                        <div className="connected-repo-head">
                          <div>
                            <strong>{repo.full_name}</strong>
                            <p>{repo.sync_status || "connected"}</p>
                          </div>
                          <button
                            type="button"
                            className={`button button-secondary button-small ${syncActionStatusByRepo[repo.id] === "loading" ? "is-disabled" : ""}`}
                            disabled={syncActionStatusByRepo[repo.id] === "loading"}
                            onClick={() => handleQueueSync(repo)}
                          >
                            {syncActionStatusByRepo[repo.id] === "loading" ? "Queueing..." : "Queue sync"}
                          </button>
                        </div>
                        <div className="sync-run-summary">
                          <span className={`sync-run-pill sync-run-${latestSyncRunsByRepo[repo.id]?.status || "none"}`}>
                            {latestSyncRunsByRepo[repo.id]?.status || "no sync run yet"}
                          </span>
                          {latestSyncRunsByRepo[repo.id]?.sync_type ? (
                            <span className="sync-run-meta">Latest run: {latestSyncRunsByRepo[repo.id].sync_type}</span>
                          ) : null}
                        </div>
                        <div className="repository-tags tight-tags">
                          <span>{repo.is_private ? "Private" : "Public"}</span>
                          <span>{repo.default_branch}</span>
                        </div>
                        {syncActionErrorByRepo[repo.id] ? <p className="inline-error repo-inline-error">{syncActionErrorByRepo[repo.id]}</p> : null}
                      </div>
                    ))}
                  </div>
                ) : null}
              </section>

              <section className="panel panel-note">
                <h3>What happens next</h3>
                <ol className="next-steps-list">
                  <li>CodeAtlas saves the repository connection in Postgres.</li>
                  <li>The sync service will later import commit history and contributor data.</li>
                  <li>Analytics and graph workers will use that data for ownership, hotspots, and dependency insights.</li>
                </ol>
              </section>
            </aside>
          </section>

          {installationState.error || authError ? <section className="inline-alert">{installationState.error || authError}</section> : null}
        </main>
      </div>

      <div className={`toast ${toast ? "is-visible" : ""}`}>{toast}</div>
    </>
  );
}

function getInstallationLabel(installation) {
  return installation.account_login || `GitHub access ${installation.installation_id}`;
}

function getInstallationDescription(installation) {
  if (installation.account_type === "Organization") {
    return "Organization access";
  }
  if (installation.account_type === "User") {
    return "Personal account access";
  }
  return "GitHub App access";
}

function getSetupSummary(user, installations, selectedInstallation, selectedRepository, connectedRepositories) {
  return [
    {
      label: "Signed in",
      meta: user ? "Ready" : "Pending",
      state: user ? "done" : "idle"
    },
    {
      label: "GitHub access",
      meta: installations.length > 0 ? `${installations.length} available` : "Add one",
      state: installations.length > 0 ? "done" : "active"
    },
    {
      label: "Repository chosen",
      meta: selectedRepository ? "Ready" : "Choose one",
      state: selectedRepository ? "done" : "idle"
    },
    {
      label: "Saved in CodeAtlas",
      meta: connectedRepositories.length > 0 ? `${connectedRepositories.length} connected` : "Connect one",
      state: connectedRepositories.length > 0 ? "done" : "idle"
    }
  ];
}

createRoot(document.getElementById("app")).render(<App />);
