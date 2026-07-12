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
  const [route, setRoute] = useState(() => getCurrentRoute());
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
  const [dashboardStatus, setDashboardStatus] = useState("idle");
  const [dashboardError, setDashboardError] = useState("");
  const [dashboardData, setDashboardData] = useState(null);

  useEffect(() => {
    const onPopState = () => {
      setRoute(getCurrentRoute());
    };

    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

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
        const response = await fetch(`${CONFIG.apiBaseUrl}/repos/sync-status`, {
          headers: {
            Authorization: `Bearer ${token}`
          }
        });

        if (response.status === 401) {
          throw new Error("unauthorized");
        }

        if (!response.ok) {
          throw new Error(`sync status request failed with status ${response.status}`);
        }

        const payload = await response.json();
        const results = (payload.repositories || []).map((item) => [item.repository.id, item.latest_sync_run || null]);

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
  }, [token, user, deduplicatedConnectedRepositories.length, connectedReposVersion]);

  useEffect(() => {
    if (!token || !user || deduplicatedConnectedRepositories.length === 0) {
      return undefined;
    }

    const timer = window.setInterval(() => {
      setConnectedReposVersion((current) => current + 1);
    }, 5000);

    return () => window.clearInterval(timer);
  }, [token, user, deduplicatedConnectedRepositories.length]);

  useEffect(() => {
    if (!token || !user || (route.view !== "dashboard" && route.view !== "modules" && route.view !== "hotspots" && route.view !== "cochange") || !route.repositoryId) {
      if (route.view !== "dashboard" && route.view !== "modules" && route.view !== "hotspots" && route.view !== "cochange") {
        setDashboardData(null);
        setDashboardStatus("idle");
        setDashboardError("");
      }
      return;
    }

    let cancelled = false;

    async function loadDashboard() {
      setDashboardStatus("loading");
      setDashboardError("");

      try {
        const headers = {
          Authorization: `Bearer ${token}`
        };

        if (route.view === "hotspots") {
          const [dashboardResponse, hotspotsResponse] = await Promise.all([
            fetch(`${CONFIG.apiBaseUrl}/repos/${route.repositoryId}/dashboard`, { headers }),
            fetch(`${CONFIG.apiBaseUrl}/repos/${route.repositoryId}/hotspots?limit=100`, { headers })
          ]);

          if (dashboardResponse.status === 401 || hotspotsResponse.status === 401) {
            localStorage.removeItem(CONFIG.tokenStorageKey);
            if (!cancelled) {
              resetSession("Session expired. Sign in again.");
            }
            return;
          }

          if (dashboardResponse.status === 404) {
            if (!cancelled) {
              setDashboardData(null);
              setDashboardStatus("failed");
              setDashboardError("This repository is not available in your workspace yet.");
            }
            return;
          }

          if (!dashboardResponse.ok) {
            throw new Error(`dashboard request failed with status ${dashboardResponse.status}`);
          }

          if (!hotspotsResponse.ok) {
            throw new Error(`hotspots request failed with status ${hotspotsResponse.status}`);
          }

          const dashboardPayload = await dashboardResponse.json();
          const hotspotsPayload = await hotspotsResponse.json();

          if (!cancelled) {
            setDashboardData({
              ...dashboardPayload.dashboard,
              hotspots: hotspotsPayload.hotspots || []
            });
            setDashboardStatus("ready");
            setDashboardError("");
          }
          return;
        }

        if (route.view === "cochange") {
          const [dashboardResponse, coChangeResponse, moduleCoChangeResponse] = await Promise.all([
            fetch(`${CONFIG.apiBaseUrl}/repos/${route.repositoryId}/dashboard`, { headers }),
            fetch(`${CONFIG.apiBaseUrl}/repos/${route.repositoryId}/co-change?limit=100`, { headers }),
            fetch(`${CONFIG.apiBaseUrl}/repos/${route.repositoryId}/modules/co-change?limit=50`, { headers })
          ]);

          if (dashboardResponse.status === 401 || coChangeResponse.status === 401 || moduleCoChangeResponse.status === 401) {
            localStorage.removeItem(CONFIG.tokenStorageKey);
            if (!cancelled) {
              resetSession("Session expired. Sign in again.");
            }
            return;
          }

          if (dashboardResponse.status === 404) {
            if (!cancelled) {
              setDashboardData(null);
              setDashboardStatus("failed");
              setDashboardError("This repository is not available in your workspace yet.");
            }
            return;
          }

          if (!dashboardResponse.ok) {
            throw new Error(`dashboard request failed with status ${dashboardResponse.status}`);
          }

          if (!coChangeResponse.ok) {
            throw new Error(`co-change request failed with status ${coChangeResponse.status}`);
          }

          if (!moduleCoChangeResponse.ok) {
            throw new Error(`module co-change request failed with status ${moduleCoChangeResponse.status}`);
          }

          const dashboardPayload = await dashboardResponse.json();
          const coChangePayload = await coChangeResponse.json();
          const moduleCoChangePayload = await moduleCoChangeResponse.json();

          if (!cancelled) {
            setDashboardData({
              ...dashboardPayload.dashboard,
              co_changes: coChangePayload.co_changes || [],
              module_co_changes: moduleCoChangePayload.module_co_changes || []
            });
            setDashboardStatus("ready");
            setDashboardError("");
          }
          return;
        }

        const [dashboardResponse, ownershipResponse, expertiseResponse, busFactorResponse] = await Promise.all([
          fetch(`${CONFIG.apiBaseUrl}/repos/${route.repositoryId}/dashboard`, { headers }),
          fetch(`${CONFIG.apiBaseUrl}/repos/${route.repositoryId}/modules/ownership`, { headers }),
          fetch(`${CONFIG.apiBaseUrl}/repos/${route.repositoryId}/modules/expertise`, { headers }),
          fetch(`${CONFIG.apiBaseUrl}/repos/${route.repositoryId}/modules/bus-factor`, { headers })
        ]);

        if (
          dashboardResponse.status === 401 ||
          ownershipResponse.status === 401 ||
          expertiseResponse.status === 401 ||
          busFactorResponse.status === 401
        ) {
          localStorage.removeItem(CONFIG.tokenStorageKey);
          if (!cancelled) {
            resetSession("Session expired. Sign in again.");
          }
          return;
        }

        if (dashboardResponse.status === 404) {
          if (!cancelled) {
            setDashboardData(null);
            setDashboardStatus("failed");
            setDashboardError("This repository is not available in your workspace yet.");
          }
          return;
        }

        if (!dashboardResponse.ok) {
          throw new Error(`dashboard request failed with status ${dashboardResponse.status}`);
        }

        const dashboardPayload = await dashboardResponse.json();
        const moduleOwnershipPayload = ownershipResponse.ok ? await ownershipResponse.json() : { modules: [] };
        const moduleExpertisePayload = expertiseResponse.ok ? await expertiseResponse.json() : { modules: [] };
        const moduleBusFactorPayload = busFactorResponse.ok ? await busFactorResponse.json() : { modules: [] };
        const coChangeResponse = await fetch(`${CONFIG.apiBaseUrl}/repos/${route.repositoryId}/co-change?limit=6`, { headers });

        if (coChangeResponse.status === 401) {
          localStorage.removeItem(CONFIG.tokenStorageKey);
          if (!cancelled) {
            resetSession("Session expired. Sign in again.");
          }
          return;
        }

        const coChangePayload = coChangeResponse.ok ? await coChangeResponse.json() : { co_changes: [] };

        if (!cancelled) {
          setDashboardData({
            ...dashboardPayload.dashboard,
            module_ownership: moduleOwnershipPayload.modules || [],
            module_expertise: moduleExpertisePayload.modules || [],
            module_bus_factor: moduleBusFactorPayload.modules || [],
            co_changes: coChangePayload.co_changes || []
          });
          setDashboardStatus("ready");
          setDashboardError("");
        }
      } catch (error) {
        console.error(error);
        if (!cancelled) {
          setDashboardData(null);
          setDashboardStatus("failed");
          setDashboardError("Could not load the repository dashboard.");
        }
      }
    }

    loadDashboard();

    return () => {
      cancelled = true;
    };
  }, [token, user, route]);

  const setupSummary = useMemo(
    () => getSetupSummary(user, installations, selectedInstallation, selectedRepository, deduplicatedConnectedRepositories),
    [user, installations, selectedInstallation, selectedRepository, deduplicatedConnectedRepositories]
  );

  function resetSession(message) {
    setToken("");
    setRoute(getCurrentRoute());
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
    navigateToRoute("/");
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
      setConnectedReposVersion((current) => current + 1);
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

  const navigateToRoute = (path) => {
    window.history.pushState({}, "", path);
    setRoute(getCurrentRoute());
  };

  const openRepositoryDashboard = (repoId) => {
    navigateToRoute(`/repos/${repoId}/dashboard`);
  };

  const openRepositoryModules = (repoId) => {
    navigateToRoute(`/repos/${repoId}/modules`);
  };

  const openRepositoryHotspots = (repoId) => {
    navigateToRoute(`/repos/${repoId}/hotspots`);
  };

  const openRepositoryCoChange = (repoId) => {
    navigateToRoute(`/repos/${repoId}/co-change`);
  };

  const returnToRepositories = () => {
    navigateToRoute("/");
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

          <div className="topbar-nav">
            {route.view === "dashboard" || route.view === "modules" || route.view === "hotspots" || route.view === "cochange" ? (
              <button type="button" className="link-button topbar-back" onClick={returnToRepositories}>
                ← Repositories
              </button>
            ) : null}
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
          {route.view === "dashboard" ? (
            <RepositoryDashboardPage
              dashboard={dashboardData}
              status={dashboardStatus}
              error={dashboardError}
              onBack={returnToRepositories}
              onOpenModules={openRepositoryModules}
              onOpenHotspots={openRepositoryHotspots}
              onOpenCoChange={openRepositoryCoChange}
            />
          ) : route.view === "modules" ? (
            <RepositoryModulesPage
              dashboard={dashboardData}
              status={dashboardStatus}
              error={dashboardError}
              onBack={() => openRepositoryDashboard(route.repositoryId)}
            />
          ) : route.view === "hotspots" ? (
            <RepositoryHotspotsPage
              dashboard={dashboardData}
              status={dashboardStatus}
              error={dashboardError}
              onBack={() => openRepositoryDashboard(route.repositoryId)}
            />
          ) : route.view === "cochange" ? (
            <RepositoryCoChangePage
              dashboard={dashboardData}
              status={dashboardStatus}
              error={dashboardError}
              token={token}
              onBack={() => openRepositoryDashboard(route.repositoryId)}
            />
          ) : (
            <>
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
                        {latestSyncRunsByRepo[repo.id]?.status === "failed" ? (
                          <div className="sync-run-alert sync-run-alert-failed">
                            <strong>Latest sync failed</strong>
                            <p>{latestSyncRunsByRepo[repo.id]?.error_message || "The last sync ended with an error."}</p>
                            <span>{`Last attempt ${formatRelativeDate(latestSyncRunsByRepo[repo.id]?.completed_at || latestSyncRunsByRepo[repo.id]?.started_at || latestSyncRunsByRepo[repo.id]?.created_at)}`}</span>
                          </div>
                        ) : null}

                        <div className="connected-repo-head">
                          <div>
                            <button type="button" className="repo-link-button" onClick={() => openRepositoryDashboard(repo.id)}>
                              {repo.full_name}
                            </button>
                            <p>{getRepositorySyncHeadline(repo, latestSyncRunsByRepo[repo.id])}</p>
                          </div>
                          <div className="connected-repo-actions">
                            <button type="button" className="button button-secondary button-small" onClick={() => openRepositoryDashboard(repo.id)}>
                              Open dashboard
                            </button>
                            <button
                              type="button"
                              className={`button button-secondary button-small ${syncActionStatusByRepo[repo.id] === "loading" || isSyncActive(latestSyncRunsByRepo[repo.id]) ? "is-disabled" : ""}`}
                              disabled={syncActionStatusByRepo[repo.id] === "loading" || isSyncActive(latestSyncRunsByRepo[repo.id])}
                              onClick={() => handleQueueSync(repo)}
                            >
                              {syncActionStatusByRepo[repo.id] === "loading" ? "Queueing..." : isSyncActive(latestSyncRunsByRepo[repo.id]) ? "Sync active" : "Queue sync"}
                            </button>
                          </div>
                        </div>
                        <div className="sync-run-summary">
                          <span className={`sync-run-pill sync-run-${latestSyncRunsByRepo[repo.id]?.status || "none"}`}>
                            {getSyncRunPillLabel(latestSyncRunsByRepo[repo.id])}
                          </span>
                          <span className="sync-run-meta">{getSyncRunMeta(repo, latestSyncRunsByRepo[repo.id])}</span>
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
            </>
          )}
        </main>
      </div>

      <div className={`toast ${toast ? "is-visible" : ""}`}>{toast}</div>
    </>
  );
}

function RepositoryDashboardPage({ dashboard, status, error, onBack, onOpenModules, onOpenHotspots, onOpenCoChange }) {
  if (status === "loading") {
    return (
      <section className="dashboard-shell">
        <div className="dashboard-empty-state">
          <strong>Loading repository dashboard…</strong>
          <p>Fetching repo metadata, sync history, and contributor details.</p>
        </div>
      </section>
    );
  }

  if (status === "failed") {
    return (
      <section className="dashboard-shell">
        <div className="dashboard-empty-state">
          <strong>Dashboard unavailable</strong>
          <p>{error || "We could not load this repository right now."}</p>
          <button type="button" className="button button-secondary button-small" onClick={onBack}>
            Back to repositories
          </button>
        </div>
      </section>
    );
  }

  if (!dashboard) {
    return null;
  }

  const repo = dashboard.repository;
  const overview = dashboard.overview || {};
  const hotspots = dashboard.hotspots || [];
  const coChanges = dashboard.co_changes || [];
  const moduleOwnership = dashboard.module_ownership || [];
  const moduleExpertise = dashboard.module_expertise || [];
  const moduleBusFactor = dashboard.module_bus_factor || [];
  const latestSyncRun = dashboard.latest_sync_run;
  const recentSyncRuns = dashboard.recent_sync_runs || [];
  const topContributors = dashboard.top_contributors || [];
  const highlights = getDashboardHighlights(repo, overview, latestSyncRun, topContributors, moduleBusFactor);
  const metrics = getDashboardMetrics(repo, overview, latestSyncRun, recentSyncRuns, topContributors);
  const moduleSummary = getModuleSummary(moduleOwnership, moduleExpertise, moduleBusFactor);

  return (
    <section className="dashboard-shell">
      <section className="dashboard-hero">
        <div className="dashboard-hero-copy">
          <div className="dashboard-breadcrumb">
            <button type="button" className="link-button" onClick={onBack}>
              Repositories
            </button>
            <span>/</span>
            <span>{repo.name}</span>
          </div>
          <div className="dashboard-title-row">
            <h1>{repo.name}</h1>
            <div className="dashboard-badges">
              <span className="dashboard-badge dashboard-badge-primary">{formatSyncStatusForBadge(repo.sync_status)}</span>
              <span className="dashboard-badge">{repo.is_private ? "Private repo" : "Public repo"}</span>
              <span className="dashboard-badge dashboard-badge-muted">Env not connected</span>
              <span className="dashboard-badge dashboard-badge-muted">Version unavailable</span>
            </div>
          </div>
          <p className="dashboard-subtitle">{repo.full_name}</p>
          <p className="dashboard-description">
            This is the first repository dashboard view for <strong>{repo.name}</strong>. It uses only the repository metadata,
            sync history, and contributor data that CodeAtlas already stores safely today.
          </p>
        </div>

        <div className="dashboard-hero-meta">
          <div className="hero-meta-card">
            <span>Default branch</span>
            <strong>{repo.default_branch}</strong>
          </div>
          <div className="hero-meta-card">
            <span>Last updated</span>
            <strong>{formatRelativeDate(overview.last_synced_at || repo.last_synced_at || repo.updated_at)}</strong>
          </div>
          <div className="hero-meta-card">
            <span>Latest sync</span>
            <strong>{latestSyncRun ? formatSyncStatusForBadge(latestSyncRun.status) : "Not synced yet"}</strong>
          </div>
        </div>
      </section>

      <section className="dashboard-welcome">
        <div>
          <span className="eyebrow">Welcome summary</span>
          <h2>{repo.name} at a glance</h2>
          <p>
            A safe first dashboard focused on what CodeAtlas knows for sure right now: sync health, mapped codebase size, and
            contributor ownership signals.
          </p>
        </div>
        <div className="dashboard-highlight-list">
          {highlights.map((highlight) => (
            <div className="dashboard-highlight-card" key={highlight.title}>
              <strong>{highlight.title}</strong>
              <p>{highlight.body}</p>
            </div>
          ))}
        </div>
      </section>

      {latestSyncRun ? (
        <section className={`dashboard-sync-health dashboard-sync-health-${latestSyncRun.status}`}>
          <div>
            <span className="eyebrow">Sync health</span>
            <h2>{getSyncHealthTitle(latestSyncRun)}</h2>
            <p>{getSyncHealthBody(latestSyncRun)}</p>
          </div>
          <div className="dashboard-sync-health-meta">
            <div className="hero-meta-card">
              <span>Latest state</span>
              <strong>{formatSyncStatusForBadge(latestSyncRun.status)}</strong>
            </div>
            <div className="hero-meta-card">
              <span>Last attempt</span>
              <strong>{formatRelativeDate(latestSyncRun.completed_at || latestSyncRun.started_at || latestSyncRun.created_at)}</strong>
            </div>
            <div className="hero-meta-card">
              <span>Duration</span>
              <strong>{formatDuration(latestSyncRun.summary?.duration_ms)}</strong>
            </div>
          </div>
        </section>
      ) : null}

      <section className="dashboard-kpi-grid">
        {metrics.map((metric) => (
          <div className="dashboard-kpi-card" key={metric.label}>
            <span>{metric.label}</span>
            <strong>{metric.value}</strong>
            <p>{metric.meta}</p>
          </div>
        ))}
      </section>

      <section className="dashboard-main-grid">
        <div className="dashboard-main-column">
          <section className="dashboard-panel">
            <div className="dashboard-panel-head">
              <div>
                <h3>Repository overview</h3>
                <p>Real metadata and sync signals that are safely available today.</p>
              </div>
            </div>
            <div className="dashboard-overview-grid">
              <div className="overview-item">
                <span>Total commits</span>
                <strong>{formatCount(overview.total_commits)}</strong>
              </div>
              <div className="overview-item">
                <span>Total contributors</span>
                <strong>{formatCount(overview.total_contributors)}</strong>
              </div>
              <div className="overview-item">
                <span>Total files</span>
                <strong>{formatCount(overview.total_files)}</strong>
              </div>
              <div className="overview-item">
                <span>Total modules</span>
                <strong>{formatCount(overview.total_modules)}</strong>
              </div>
              <div className="overview-item">
                <span>Last sync duration</span>
                <strong>{formatDuration(overview.latest_sync_duration_ms)}</strong>
              </div>
              <div className="overview-item">
                <span>Last synced</span>
                <strong>{formatRelativeDate(overview.last_synced_at)}</strong>
              </div>
            </div>
          </section>

          <section className="dashboard-panel">
            <div className="dashboard-panel-head">
              <div>
                <h3>Hotspot files</h3>
                <p>A compact summary of where file churn is currently concentrated.</p>
              </div>
              <button type="button" className="button button-secondary button-small" onClick={() => onOpenHotspots(repo.id)}>
                View all hotspots
              </button>
            </div>

            {hotspots.length === 0 ? (
              <div className="dashboard-empty-panel">
                <strong>No hotspot data yet</strong>
                <p>Hotspot analysis will appear here after commit history has been imported.</p>
              </div>
            ) : (
              <div className="dashboard-module-summary-grid">
                {getHotspotSummary(hotspots).map((item) => (
                  <div className="dashboard-module-summary-card" key={item.title}>
                    <span className="dashboard-module-summary-label">{item.title}</span>
                    <strong>{item.primary}</strong>
                    <p>{item.secondary}</p>
                    {item.meta ? <span className="dashboard-inline-chip">{item.meta}</span> : null}
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="dashboard-panel">
            <div className="dashboard-panel-head">
              <div>
                <h3>Files that move together</h3>
                <p>Early co-change signals from commit history, useful for spotting coupling and hidden dependencies.</p>
              </div>
              <button type="button" className="button button-secondary button-small" onClick={() => onOpenCoChange(repo.id)}>
                View all pairs
              </button>
            </div>

            {coChanges.length === 0 ? (
              <div className="dashboard-empty-panel">
                <strong>No co-change data yet</strong>
                <p>Once synced commit history is available, CodeAtlas will surface files that frequently change together.</p>
              </div>
            ) : (
              <div className="dashboard-module-summary-grid">
                {getCoChangeSummary(coChanges).map((item) => (
                  <div className="dashboard-module-summary-card" key={item.title}>
                    <span className="dashboard-module-summary-label">{item.title}</span>
                    <strong>{item.primary}</strong>
                    <p>{item.secondary}</p>
                    {item.meta ? <span className="dashboard-inline-chip">{item.meta}</span> : null}
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="dashboard-panel">
            <div className="dashboard-panel-head">
              <div>
                <h3>Recent sync activity</h3>
                <p>Latest import attempts recorded by CodeAtlas.</p>
              </div>
            </div>

            {recentSyncRuns.length === 0 ? (
              <div className="dashboard-empty-panel">
                <strong>No sync history yet</strong>
                <p>Queue the first sync from the repository list to populate operational history here.</p>
              </div>
            ) : (
              <div className="dashboard-sync-list">
                {recentSyncRuns.map((run) => (
                  <div className="dashboard-sync-row" key={run.id}>
                    <div>
                      <strong>{formatSyncStatusForBadge(run.status)}</strong>
                      <p>{formatSyncRunDetail(run)}</p>
                    </div>
                    <div className="dashboard-sync-meta">
                      <span>{formatRelativeDate(run.completed_at || run.started_at || run.created_at)}</span>
                      <code>{run.sync_type}</code>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="dashboard-panel">
            <div className="dashboard-panel-head">
              <div>
                <h3>Module intelligence</h3>
                <p>A compact summary of ownership, expertise, and risk across the synced module map.</p>
              </div>
              <button type="button" className="button button-secondary button-small" onClick={() => onOpenModules(repo.id)}>
                View all modules
              </button>
            </div>

            {moduleSummary.length === 0 ? (
              <div className="dashboard-empty-panel">
                <strong>No module analytics yet</strong>
                <p>Run a completed sync to compute ownership, expertise, and bus-factor summaries for modules.</p>
              </div>
            ) : (
              <div className="dashboard-module-summary-grid">
                {moduleSummary.map((item) => (
                  <div className="dashboard-module-summary-card" key={item.title}>
                    <span className="dashboard-module-summary-label">{item.title}</span>
                    <strong>{item.primary}</strong>
                    <p>{item.secondary}</p>
                    {item.meta ? <span className={`dashboard-inline-chip ${item.metaClass || ""}`}>{item.meta}</span> : null}
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="dashboard-panel">
            <div className="dashboard-panel-head">
              <div>
                <h3>Top module owners</h3>
                <p>The strongest ownership signals across the highest-priority modules.</p>
              </div>
            </div>

            {moduleOwnership.length === 0 ? (
              <div className="dashboard-empty-panel">
                <strong>No module ownership data yet</strong>
                <p>Ownership distribution appears here after module analytics have been rebuilt.</p>
              </div>
            ) : (
              <div className="dashboard-module-list">
                {moduleOwnership.slice(0, 3).map((module) => (
                  <div className="dashboard-module-card" key={module.module_id}>
                    <div className="dashboard-module-head">
                      <div>
                        <strong>{module.module_name}</strong>
                        <p>{formatModulePath(module.path_prefix)}</p>
                      </div>
                      <div className="dashboard-module-badges">
                        <span className={`dashboard-badge ${getRiskBadgeClass(module.risk)}`}>{formatRiskLabel(module.risk)}</span>
                        <span className="dashboard-badge">{`Bus factor ${module.bus_factor || 0}`}</span>
                      </div>
                    </div>
                    {module.owners.length === 0 ? (
                      <p className="dashboard-module-empty-copy">No ownership entries computed yet for this module.</p>
                    ) : (
                      <div className="dashboard-module-owner-list">
                        {module.owners.slice(0, 3).map((owner) => (
                          <div className="dashboard-module-owner-row" key={`${module.module_id}-${owner.rank}-${owner.username}`}>
                            <div>
                              <strong>{owner.username}</strong>
                              <p>
                                {formatCount(owner.commit_count)} commits, {formatCount(owner.files_touched_count)} files touched
                              </p>
                            </div>
                            <span className="dashboard-module-owner-percent">{formatPercent(owner.ownership_percent)}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="dashboard-panel">
            <div className="dashboard-panel-head">
              <div>
                <h3>Contributors</h3>
                <p>Top contributors from the most recent synced contributor data.</p>
              </div>
            </div>

            {topContributors.length === 0 ? (
              <div className="dashboard-empty-panel">
                <strong>No contributor data yet</strong>
                <p>Contributor ownership appears here after the repository completes its first sync.</p>
              </div>
            ) : (
              <div className="dashboard-contributor-list">
                {topContributors.map((contributor) => (
                  <div className="dashboard-contributor-row" key={contributor.id}>
                    <div className="dashboard-contributor-id">
                      <div className="dashboard-contributor-avatar">
                        {contributor.avatar_url ? <img src={contributor.avatar_url} alt={contributor.username} /> : contributor.username.slice(0, 1).toUpperCase()}
                      </div>
                      <div>
                        <strong>{contributor.username}</strong>
                        <p>{formatContributionLabel(contributor.contributions_count)}</p>
                      </div>
                    </div>
                    <span className="dashboard-contributor-count">{contributor.contributions_count}</span>
                  </div>
                ))}
              </div>
            )}
          </section>
        </div>

        <aside className="dashboard-side-column">
          <section className="dashboard-panel">
            <div className="dashboard-panel-head">
              <div>
                <h3>Operational highlights</h3>
                <p>High-value facts we can source safely right now.</p>
              </div>
            </div>
            <div className="dashboard-aside-list">
              <div className="aside-item">
                <span>Last sync duration</span>
                <strong>{formatDuration(overview.latest_sync_duration_ms || latestSyncRun?.summary?.duration_ms)}</strong>
              </div>
              <div className="aside-item">
                <span>Files mapped</span>
                <strong>{formatCount(overview.total_files || latestSyncRun?.summary?.files_count)}</strong>
              </div>
              <div className="aside-item">
                <span>Modules mapped</span>
                <strong>{formatCount(overview.total_modules || latestSyncRun?.summary?.modules_count)}</strong>
              </div>
              <div className="aside-item">
                <span>Highest risk module</span>
                <strong>{moduleSummary[0]?.title === "Bus factor risk" ? moduleSummary[0].primary : "Not available"}</strong>
                <p>{moduleSummary[0]?.secondary || "Risk concentration appears here after module analytics are available."}</p>
              </div>
            </div>
          </section>

          <section className="dashboard-panel">
            <div className="dashboard-panel-head">
              <div>
                <h3>Waiting on integrations</h3>
                <p>Reserved space for approved future connections.</p>
              </div>
            </div>
            <div className="dashboard-placeholder-list">
              <div className="placeholder-item">
                <strong>Environment</strong>
                <p>Not connected yet</p>
              </div>
              <div className="placeholder-item">
                <strong>Version / release</strong>
                <p>Not tracked yet</p>
              </div>
              <div className="placeholder-item">
                <strong>Open PRs</strong>
                <p>Coming soon through safe GitHub sync</p>
              </div>
              <div className="placeholder-item">
                <strong>Issues</strong>
                <p>Coming soon through safe GitHub sync</p>
              </div>
              <div className="placeholder-item">
                <strong>Deployments</strong>
                <p>Unavailable until a safe deployment integration is approved</p>
              </div>
            </div>
          </section>
        </aside>
      </section>
    </section>
  );
}

function RepositoryModulesPage({ dashboard, status, error, onBack }) {
  const [searchQuery, setSearchQuery] = useState("");
  const [sortBy, setSortBy] = useState("risk");
  const repo = dashboard?.repository;
  const moduleOwnership = dashboard?.module_ownership || [];
  const moduleExpertise = dashboard?.module_expertise || [];
  const moduleBusFactor = dashboard?.module_bus_factor || [];
  const modules = useMemo(() => {
    const mergedModules = mergeModuleAnalytics(moduleOwnership, moduleExpertise, moduleBusFactor);
    const query = searchQuery.trim().toLowerCase();

    const filteredModules = !query
      ? mergedModules
      : mergedModules.filter((module) => {
          const ownerNames = (module.owners || []).map((owner) => owner.username).join(" ").toLowerCase();
          const expertNames = (module.experts || []).map((expert) => expert.username).join(" ").toLowerCase();
          return (
            module.module_name.toLowerCase().includes(query) ||
            (module.path_prefix || "").toLowerCase().includes(query) ||
            ownerNames.includes(query) ||
            expertNames.includes(query)
          );
        });

    return [...filteredModules].sort((left, right) => {
      if (sortBy === "owners") {
        if ((right.top_owner_percent || 0) !== (left.top_owner_percent || 0)) {
          return (right.top_owner_percent || 0) - (left.top_owner_percent || 0);
        }
      } else if (sortBy === "bus_factor") {
        if ((left.bus_factor || 0) !== (right.bus_factor || 0)) {
          return (left.bus_factor || 0) - (right.bus_factor || 0);
        }
      } else if (sortBy === "contributors") {
        if ((right.active_contributors || 0) !== (left.active_contributors || 0)) {
          return (right.active_contributors || 0) - (left.active_contributors || 0);
        }
      } else {
        const leftRisk = riskOrder(left.risk);
        const rightRisk = riskOrder(right.risk);
        if (leftRisk !== rightRisk) {
          return leftRisk - rightRisk;
        }
      }

      return left.module_name.localeCompare(right.module_name);
    });
  }, [moduleOwnership, moduleExpertise, moduleBusFactor, searchQuery, sortBy]);

  if (status === "loading") {
    return (
      <section className="dashboard-shell">
        <div className="dashboard-empty-state">
          <strong>Loading module analytics…</strong>
          <p>Fetching ownership, expertise, and bus-factor details.</p>
        </div>
      </section>
    );
  }

  if (status === "failed") {
    return (
      <section className="dashboard-shell">
        <div className="dashboard-empty-state">
          <strong>Module analytics unavailable</strong>
          <p>{error || "We could not load module analytics right now."}</p>
          <button type="button" className="button button-secondary button-small" onClick={onBack}>
            Back to dashboard
          </button>
        </div>
      </section>
    );
  }

  if (!dashboard) {
    return null;
  }

  return (
    <section className="dashboard-shell">
      <section className="dashboard-subpage-hero">
        <div>
          <div className="dashboard-breadcrumb">
            <button type="button" className="link-button" onClick={onBack}>
              {repo.name}
            </button>
            <span>/</span>
            <span>Modules</span>
          </div>
          <h1>Module intelligence</h1>
          <p>
            A deeper view of ownership concentration, reviewer expertise, and bus-factor risk for <strong>{repo.full_name}</strong>.
          </p>
        </div>
      </section>

      <section className="dashboard-toolbar">
        <label className="dashboard-search-field">
          <span>Search modules</span>
          <input
            type="text"
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder="Search by module, path, owner, or expert"
          />
        </label>

        <label className="dashboard-select-field">
          <span>Sort by</span>
          <select value={sortBy} onChange={(event) => setSortBy(event.target.value)}>
            <option value="risk">Highest risk</option>
            <option value="owners">Ownership concentration</option>
            <option value="bus_factor">Lowest bus factor</option>
            <option value="contributors">Most contributors</option>
          </select>
        </label>
      </section>

      {modules.length === 0 ? (
        <section className="dashboard-panel">
          <div className="dashboard-empty-panel">
            <strong>{searchQuery ? "No modules match this search" : "No module analytics yet"}</strong>
            <p>{searchQuery ? "Try a different module name, path, owner, or expert." : "Queue and complete a repository sync to populate ownership, expertise, and bus-factor details."}</p>
          </div>
        </section>
      ) : (
        <section className="dashboard-module-detail-list">
          {modules.map((module) => (
            <section className="dashboard-panel" key={module.module_id}>
              <div className="dashboard-panel-head">
                <div>
                  <h3>{module.module_name}</h3>
                  <p>{formatModulePath(module.path_prefix)}</p>
                </div>
                <div className="dashboard-module-badges">
                  <span className={`dashboard-badge ${getRiskBadgeClass(module.risk)}`}>{formatRiskLabel(module.risk)}</span>
                  <span className="dashboard-badge">{`Bus factor ${module.bus_factor || 0}`}</span>
                  <span className="dashboard-badge">{`${formatCount(module.active_contributors || 0)} contributors`}</span>
                </div>
              </div>

              <div className="dashboard-module-detail-grid">
                <div className="dashboard-module-detail-column">
                  <h4>Ownership</h4>
                  {module.owners.length === 0 ? (
                    <p className="dashboard-module-empty-copy">No ownership entries computed yet.</p>
                  ) : (
                    <div className="dashboard-module-owner-list">
                      {module.owners.map((owner) => (
                        <div className="dashboard-module-owner-row" key={`${module.module_id}-owner-${owner.rank}-${owner.username}`}>
                          <div>
                            <strong>{owner.username}</strong>
                            <p>
                              {formatCount(owner.commit_count)} commits, {formatCount(owner.files_touched_count)} files touched, {formatCount(owner.changes_count)} changes
                            </p>
                          </div>
                          <span className="dashboard-module-owner-percent">{formatPercent(owner.ownership_percent)}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                <div className="dashboard-module-detail-column">
                  <h4>Expertise</h4>
                  {module.experts.length === 0 ? (
                    <p className="dashboard-module-empty-copy">No expertise entries computed yet.</p>
                  ) : (
                    <div className="dashboard-module-owner-list">
                      {module.experts.map((expert) => (
                        <div className="dashboard-module-owner-row" key={`${module.module_id}-expert-${expert.rank}-${expert.username}`}>
                          <div>
                            <strong>{expert.username}</strong>
                            <p>
                              Score {expert.score}, {formatCount(expert.commit_count)} commits, {formatCount(expert.recent_commit_count)} recent commits
                            </p>
                          </div>
                          <span className="dashboard-module-owner-percent">{expert.score}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </section>
          ))}
        </section>
      )}
    </section>
  );
}

function RepositoryHotspotsPage({ dashboard, status, error, onBack }) {
  const [searchQuery, setSearchQuery] = useState("");
  const [sortBy, setSortBy] = useState("churn");
  const repo = dashboard?.repository;
  const hotspots = useMemo(() => {
    const items = dashboard?.hotspots || [];
    const query = searchQuery.trim().toLowerCase();
    const filteredItems = !query ? items : items.filter((hotspot) => hotspot.path.toLowerCase().includes(query));

    return [...filteredItems].sort((left, right) => {
      if (sortBy === "commits") {
        if (right.commit_count !== left.commit_count) {
          return right.commit_count - left.commit_count;
        }
      } else if (sortBy === "additions") {
        if (right.lines_added !== left.lines_added) {
          return right.lines_added - left.lines_added;
        }
      } else {
        if (right.churn !== left.churn) {
          return right.churn - left.churn;
        }
      }

      return left.path.localeCompare(right.path);
    });
  }, [dashboard, searchQuery, sortBy]);

  if (status === "loading") {
    return (
      <section className="dashboard-shell">
        <div className="dashboard-empty-state">
          <strong>Loading hotspot files…</strong>
          <p>Fetching ranked file churn details from synced commit history.</p>
        </div>
      </section>
    );
  }

  if (status === "failed") {
    return (
      <section className="dashboard-shell">
        <div className="dashboard-empty-state">
          <strong>Hotspot detail unavailable</strong>
          <p>{error || "We could not load hotspot details right now."}</p>
          <button type="button" className="button button-secondary button-small" onClick={onBack}>
            Back to dashboard
          </button>
        </div>
      </section>
    );
  }

  if (!dashboard) {
    return null;
  }

  return (
    <section className="dashboard-shell">
      <section className="dashboard-subpage-hero">
        <div>
          <div className="dashboard-breadcrumb">
            <button type="button" className="link-button" onClick={onBack}>
              {repo.name}
            </button>
            <span>/</span>
            <span>Hotspots</span>
          </div>
          <h1>Hotspot files</h1>
          <p>
            Ranked file churn detail for <strong>{repo.full_name}</strong>, based on synced commit history only.
          </p>
        </div>
      </section>

      <section className="dashboard-toolbar">
        <label className="dashboard-search-field">
          <span>Search files</span>
          <input
            type="text"
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder="Search by file path"
          />
        </label>

        <label className="dashboard-select-field">
          <span>Sort by</span>
          <select value={sortBy} onChange={(event) => setSortBy(event.target.value)}>
            <option value="churn">Highest churn</option>
            <option value="commits">Most commits</option>
            <option value="additions">Most additions</option>
          </select>
        </label>
      </section>

      {hotspots.length === 0 ? (
        <section className="dashboard-panel">
          <div className="dashboard-empty-panel">
            <strong>{searchQuery ? "No files match this search" : "No hotspot data yet"}</strong>
            <p>{searchQuery ? "Try a different file path." : "Complete a repository sync to populate file-level churn and hotspot rankings."}</p>
          </div>
        </section>
      ) : (
        <section className="dashboard-module-detail-list">
          {hotspots.map((hotspot, index) => (
            <section className="dashboard-panel" key={hotspot.path}>
              <div className="dashboard-panel-head">
                <div>
                  <h3 title={hotspot.path}>{hotspot.path}</h3>
                  <p>{`Rank #${index + 1} by total churn`}</p>
                </div>
                <div className="dashboard-module-badges">
                  <span className="dashboard-badge">{`${formatCount(hotspot.commit_count)} commits`}</span>
                  <span className="dashboard-badge dashboard-badge-primary">{`${formatCount(hotspot.churn)} churn`}</span>
                </div>
              </div>

              <div className="dashboard-overview-grid">
                <div className="overview-item">
                  <span>Lines added</span>
                  <strong>{formatCount(hotspot.lines_added)}</strong>
                </div>
                <div className="overview-item">
                  <span>Lines deleted</span>
                  <strong>{formatCount(hotspot.lines_deleted)}</strong>
                </div>
                <div className="overview-item">
                  <span>Commit touches</span>
                  <strong>{formatCount(hotspot.commit_count)}</strong>
                </div>
                <div className="overview-item">
                  <span>Total churn</span>
                  <strong>{formatCount(hotspot.churn)}</strong>
                </div>
              </div>
            </section>
          ))}
        </section>
      )}
    </section>
  );
}

function RepositoryCoChangePage({ dashboard, status, error, token, onBack }) {
  const [searchQuery, setSearchQuery] = useState("");
  const [sortBy, setSortBy] = useState("frequency");
  const [focusedPath, setFocusedPath] = useState("");
  const [focusedPairs, setFocusedPairs] = useState([]);
  const [focusedStatus, setFocusedStatus] = useState("idle");
  const [focusedError, setFocusedError] = useState("");
  const [moduleSearchQuery, setModuleSearchQuery] = useState("");
  const [moduleSortBy, setModuleSortBy] = useState("frequency");
  const repo = dashboard?.repository;

  useEffect(() => {
    if (!focusedPath || !repo?.id || !token) {
      setFocusedPairs([]);
      setFocusedStatus("idle");
      setFocusedError("");
      return;
    }

    let cancelled = false;

    async function loadFocusedPairs() {
      setFocusedStatus("loading");
      setFocusedError("");

      try {
        const response = await fetch(
          `${CONFIG.apiBaseUrl}/repos/${repo.id}/co-change?limit=100&path=${encodeURIComponent(focusedPath)}`,
          {
            headers: {
              Authorization: `Bearer ${token}`
            }
          }
        );

        if (!response.ok) {
          throw new Error(`focused co-change request failed with status ${response.status}`);
        }

        const payload = await response.json();
        if (!cancelled) {
          setFocusedPairs(payload.co_changes || []);
          setFocusedStatus("ready");
        }
      } catch (fetchError) {
        console.error(fetchError);
        if (!cancelled) {
          setFocusedPairs([]);
          setFocusedStatus("failed");
          setFocusedError("Could not load focused file pairs.");
        }
      }
    }

    loadFocusedPairs();

    return () => {
      cancelled = true;
    };
  }, [focusedPath, repo?.id, token]);

  const pairs = useMemo(() => {
    const items = focusedPath ? focusedPairs : dashboard?.co_changes || [];
    const query = searchQuery.trim().toLowerCase();
    const filteredItems = !query
      ? items
      : items.filter((pair) => pair.left_path.toLowerCase().includes(query) || pair.right_path.toLowerCase().includes(query));

    return [...filteredItems].sort((left, right) => {
      if (sortBy === "recent") {
        const leftTime = left.last_cochanged_at ? new Date(left.last_cochanged_at).getTime() : 0;
        const rightTime = right.last_cochanged_at ? new Date(right.last_cochanged_at).getTime() : 0;
        if (rightTime !== leftTime) {
          return rightTime - leftTime;
        }
      } else {
        if (right.cochange_count !== left.cochange_count) {
          return right.cochange_count - left.cochange_count;
        }
      }

      return `${left.left_path} ${left.right_path}`.localeCompare(`${right.left_path} ${right.right_path}`);
    });
  }, [dashboard, focusedPairs, focusedPath, searchQuery, sortBy]);

  const modulePairs = useMemo(() => {
    const items = dashboard?.module_co_changes || [];
    const query = moduleSearchQuery.trim().toLowerCase();
    const filteredItems = !query
      ? items
      : items.filter(
          (pair) =>
            pair.left_module_name.toLowerCase().includes(query) ||
            pair.right_module_name.toLowerCase().includes(query) ||
            (pair.left_path_prefix || "").toLowerCase().includes(query) ||
            (pair.right_path_prefix || "").toLowerCase().includes(query)
        );

    return [...filteredItems].sort((left, right) => {
      if (moduleSortBy === "recent") {
        const leftTime = left.last_cochanged_at ? new Date(left.last_cochanged_at).getTime() : 0;
        const rightTime = right.last_cochanged_at ? new Date(right.last_cochanged_at).getTime() : 0;
        if (rightTime !== leftTime) {
          return rightTime - leftTime;
        }
      } else {
        if (right.cochange_count !== left.cochange_count) {
          return right.cochange_count - left.cochange_count;
        }
      }

      return `${left.left_module_name} ${left.right_module_name}`.localeCompare(`${right.left_module_name} ${right.right_module_name}`);
    });
  }, [dashboard, moduleSearchQuery, moduleSortBy]);

  if (status === "loading") {
    return (
      <section className="dashboard-shell">
        <div className="dashboard-empty-state">
          <strong>Loading co-change pairs…</strong>
          <p>Fetching file pairs that frequently move together from synced commit history.</p>
        </div>
      </section>
    );
  }

  if (status === "failed") {
    return (
      <section className="dashboard-shell">
        <div className="dashboard-empty-state">
          <strong>Co-change detail unavailable</strong>
          <p>{error || "We could not load co-change details right now."}</p>
          <button type="button" className="button button-secondary button-small" onClick={onBack}>
            Back to dashboard
          </button>
        </div>
      </section>
    );
  }

  if (!dashboard) {
    return null;
  }

  return (
    <section className="dashboard-shell">
      <section className="dashboard-subpage-hero">
        <div>
          <div className="dashboard-breadcrumb">
            <button type="button" className="link-button" onClick={onBack}>
              {repo.name}
            </button>
            <span>/</span>
            <span>Co-change</span>
          </div>
          <h1>Co-change pairs</h1>
          <p>
            File pairs for <strong>{repo.full_name}</strong> that frequently changed in the same commit.
          </p>
        </div>
      </section>

      <section className="dashboard-toolbar">
        <label className="dashboard-search-field">
          <span>Search file pairs</span>
          <input
            type="text"
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder="Search by either file path"
          />
        </label>

        <label className="dashboard-select-field">
          <span>Sort by</span>
          <select value={sortBy} onChange={(event) => setSortBy(event.target.value)}>
            <option value="frequency">Most shared commits</option>
            <option value="recent">Most recent pair</option>
          </select>
        </label>
      </section>

      <section className="dashboard-panel">
        <div className="dashboard-panel-head">
          <div>
            <h3>Focused file investigation</h3>
            <p>
              {focusedPath
                ? `Showing the strongest co-change neighbors for ${focusedPath}.`
                : "Click a file path below to pivot the page around one file and inspect its neighbors."}
            </p>
          </div>
          {focusedPath ? (
            <button type="button" className="button button-secondary button-small" onClick={() => setFocusedPath("")}>
              Clear focus
            </button>
          ) : null}
        </div>

        {focusedPath ? (
          <div className="dashboard-inline-state">
            <strong>{focusedPath}</strong>
            <span>
              {focusedStatus === "loading"
                ? "Loading focused file pairs…"
                : focusedStatus === "failed"
                  ? focusedError
                  : `${formatCount(focusedPairs.length)} related pair${focusedPairs.length === 1 ? "" : "s"} loaded`}
            </span>
          </div>
        ) : (
          <div className="dashboard-empty-panel">
            <strong>No focused file selected</strong>
            <p>Choose a file from any co-change pair below and CodeAtlas will reload the list centered on that file.</p>
          </div>
        )}
      </section>

      {pairs.length === 0 ? (
        <section className="dashboard-panel">
          <div className="dashboard-empty-panel">
            <strong>{searchQuery ? "No file pairs match this search" : "No co-change data yet"}</strong>
            <p>{searchQuery ? "Try a different file path." : "Complete a repository sync to compute file pairs that commonly move together."}</p>
          </div>
        </section>
      ) : (
        <section className="dashboard-module-detail-list">
          {pairs.map((pair, index) => (
            <section className="dashboard-panel" key={`${pair.left_path}-${pair.right_path}`}>
              <div className="dashboard-panel-head">
                <div>
                  <h3>{`${pair.left_path} ↔ ${pair.right_path}`}</h3>
                  <p>{`Rank #${index + 1} by shared commits`}</p>
                </div>
                <div className="dashboard-module-badges">
                  <span className="dashboard-badge dashboard-badge-primary">{`${formatCount(pair.cochange_count)} shared commits`}</span>
                  <span className="dashboard-badge">{formatRelativeDate(pair.last_cochanged_at)}</span>
                </div>
              </div>

              <div className="dashboard-overview-grid">
                <div className="overview-item">
                  <span>Left file</span>
                  <button type="button" className="dashboard-path-button" title={pair.left_path} onClick={() => setFocusedPath(pair.left_path)}>
                    {pair.left_path}
                  </button>
                </div>
                <div className="overview-item">
                  <span>Right file</span>
                  <button type="button" className="dashboard-path-button" title={pair.right_path} onClick={() => setFocusedPath(pair.right_path)}>
                    {pair.right_path}
                  </button>
                </div>
                <div className="overview-item">
                  <span>Shared commits</span>
                  <strong>{formatCount(pair.cochange_count)}</strong>
                </div>
                <div className="overview-item">
                  <span>Last seen together</span>
                  <strong>{formatRelativeDate(pair.last_cochanged_at)}</strong>
                </div>
              </div>
            </section>
          ))}
        </section>
      )}

      <section className="dashboard-subpage-hero">
        <div>
          <h1>Module-to-module co-change</h1>
          <p>Higher-level module coupling signals based on modules that frequently show up in the same commits.</p>
        </div>
      </section>

      <section className="dashboard-toolbar">
        <label className="dashboard-search-field">
          <span>Search module pairs</span>
          <input
            type="text"
            value={moduleSearchQuery}
            onChange={(event) => setModuleSearchQuery(event.target.value)}
            placeholder="Search by module name or path prefix"
          />
        </label>

        <label className="dashboard-select-field">
          <span>Sort by</span>
          <select value={moduleSortBy} onChange={(event) => setModuleSortBy(event.target.value)}>
            <option value="frequency">Most shared commits</option>
            <option value="recent">Most recent pair</option>
          </select>
        </label>
      </section>

      {modulePairs.length === 0 ? (
        <section className="dashboard-panel">
          <div className="dashboard-empty-panel">
            <strong>{moduleSearchQuery ? "No module pairs match this search" : "No module co-change data yet"}</strong>
            <p>{moduleSearchQuery ? "Try a different module name or path." : "Complete a repository sync to compute module-level co-change pairs."}</p>
          </div>
        </section>
      ) : (
        <section className="dashboard-module-detail-list">
          {modulePairs.map((pair, index) => (
            <section className="dashboard-panel" key={`${pair.left_path_prefix}-${pair.right_path_prefix}`}>
              <div className="dashboard-panel-head">
                <div>
                  <h3>{`${pair.left_module_name} ↔ ${pair.right_module_name}`}</h3>
                  <p>{`Rank #${index + 1} by shared commits`}</p>
                </div>
                <div className="dashboard-module-badges">
                  <span className="dashboard-badge dashboard-badge-primary">{`${formatCount(pair.cochange_count)} shared commits`}</span>
                  <span className="dashboard-badge">{formatRelativeDate(pair.last_cochanged_at)}</span>
                </div>
              </div>

              <div className="dashboard-overview-grid">
                <div className="overview-item">
                  <span>Left module</span>
                  <strong>{pair.left_module_name}</strong>
                  <p>{formatModulePath(pair.left_path_prefix)}</p>
                </div>
                <div className="overview-item">
                  <span>Right module</span>
                  <strong>{pair.right_module_name}</strong>
                  <p>{formatModulePath(pair.right_path_prefix)}</p>
                </div>
                <div className="overview-item">
                  <span>Shared commits</span>
                  <strong>{formatCount(pair.cochange_count)}</strong>
                </div>
                <div className="overview-item">
                  <span>Last seen together</span>
                  <strong>{formatRelativeDate(pair.last_cochanged_at)}</strong>
                </div>
              </div>
            </section>
          ))}
        </section>
      )}
    </section>
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

function isSyncActive(syncRun) {
  return syncRun?.status === "queued" || syncRun?.status === "running";
}

function getRepositorySyncHeadline(repo, latestSyncRun) {
  if (latestSyncRun?.status === "queued") {
    return "Sync queued";
  }
  if (latestSyncRun?.status === "running") {
    return "Sync in progress";
  }
  if (latestSyncRun?.status === "succeeded") {
    return "Ready";
  }
  if (latestSyncRun?.status === "failed") {
    return "Last sync failed";
  }

  switch ((repo.sync_status || "").toLowerCase()) {
    case "ready":
      return "Ready";
    case "importing":
      return "Importing";
    case "failed":
      return "Failed";
    case "pending":
      return "Connected, not synced yet";
    default:
      return "Connected";
  }
}

function getSyncHealthTitle(syncRun) {
  switch ((syncRun?.status || "").toLowerCase()) {
    case "failed":
      return "Latest sync needs attention";
    case "running":
      return "Repository sync is in progress";
    case "queued":
      return "Repository sync is queued";
    case "succeeded":
      return "Latest sync completed successfully";
    default:
      return "Repository sync status";
  }
}

function getSyncHealthBody(syncRun) {
  if (!syncRun) {
    return "No sync has been recorded yet.";
  }

  if (syncRun.status === "failed") {
    return syncRun.error_message || "The latest sync failed before the repository snapshot could finish importing.";
  }

  if (syncRun.status === "running") {
    return "CodeAtlas is currently importing contributors, commits, files, and analytics for this repository.";
  }

  if (syncRun.status === "queued") {
    return "The sync worker has not picked up this request yet. The repository should start importing shortly.";
  }

  return formatSyncRunDetail(syncRun);
}

function getCurrentRoute() {
  const pathname = window.location.pathname || "/";
  const match = pathname.match(/^\/repos\/(\d+)(?:\/(dashboard|modules|hotspots|co-change))?\/?$/);
  if (match) {
    return {
      view:
        match[2] === "modules"
          ? "modules"
          : match[2] === "hotspots"
            ? "hotspots"
            : match[2] === "co-change"
              ? "cochange"
              : "dashboard",
      repositoryId: match[1]
    };
  }

  return {
    view: "onboarding",
    repositoryId: ""
  };
}

function getDashboardHighlights(repo, overview, latestSyncRun, topContributors, moduleBusFactor) {
  const topContributor = topContributors[0];
  const highestRiskModule = moduleBusFactor.find((module) => module.risk === "high") || moduleBusFactor[0];

  return [
    {
      title: latestSyncRun ? `Latest sync is ${formatSyncStatusForBadge(latestSyncRun.status).toLowerCase()}` : "No sync completed yet",
      body: latestSyncRun
        ? `The most recent ${latestSyncRun.sync_type} sync ${getSyncTimingSummary(latestSyncRun)}.`
        : "Queue the first sync to populate commit, file, module, and contributor insights."
    },
    {
      title: overview.total_files ? `${formatCount(overview.total_files)} files mapped` : "Codebase mapping pending",
      body: overview.total_modules
        ? `${formatCount(overview.total_modules)} modules are currently represented from the latest synced repository snapshot.`
        : "Module and file structure appears here after the first completed import."
    },
    {
      title: highestRiskModule ? `${highestRiskModule.module_name} needs the most ownership attention` : topContributor ? `${topContributor.username} is currently the top visible contributor` : "Contributor ownership will appear after sync",
      body: highestRiskModule
        ? `${formatRiskLabel(highestRiskModule.risk)} risk with bus factor ${highestRiskModule.bus_factor || 0} and top owner concentration at ${formatPercent(highestRiskModule.top_owner_percent)}.`
        : topContributor
          ? `${formatContributionLabel(topContributor.contributions_count)} are currently associated with ${topContributor.username} in synced contributor data.`
          : "Top contributors and ownership hints will populate once contributor import completes."
    }
  ];
}

function getDashboardMetrics(repo, overview, latestSyncRun, recentSyncRuns, topContributors) {
  return [
    {
      label: "Sync status",
      value: formatSyncStatusForBadge(latestSyncRun?.status || repo.sync_status),
      meta: latestSyncRun ? `Latest ${latestSyncRun.sync_type} run` : "No sync run yet"
    },
    {
      label: "Contributors",
      value: formatCount(overview.total_contributors || latestSyncRun?.summary?.contributors_count || topContributors.length),
      meta: "Stored contributor records"
    },
    {
      label: "Commits synced",
      value: formatCount(overview.total_commits || latestSyncRun?.summary?.commits_count),
      meta: "Repository commit records stored"
    },
    {
      label: "Files mapped",
      value: formatCount(overview.total_files || latestSyncRun?.summary?.files_count),
      meta: "Repository files currently indexed"
    },
    {
      label: "Modules mapped",
      value: formatCount(overview.total_modules || latestSyncRun?.summary?.modules_count),
      meta: "Top-level modules derived from files"
    },
    {
      label: "Recent sync runs",
      value: formatCount(recentSyncRuns.length),
      meta: "Most recent attempts retained on this page"
    }
  ];
}

function getModuleSummary(moduleOwnership, moduleExpertise, moduleBusFactor) {
  const highestRiskModule = moduleBusFactor.find((module) => module.risk === "high") || moduleBusFactor[0];
  const strongestOwnerModule = moduleOwnership.find((module) => module.owners && module.owners.length > 0);
  const strongestExpertModule = moduleExpertise.find((module) => module.experts && module.experts.length > 0);
  const cards = [];

  if (highestRiskModule) {
    cards.push({
      title: "Bus factor risk",
      primary: highestRiskModule.module_name,
      secondary: `Bus factor ${highestRiskModule.bus_factor || 0} with top owner concentration at ${formatPercent(highestRiskModule.top_owner_percent)}.`,
      meta: formatRiskLabel(highestRiskModule.risk),
      metaClass: getRiskBadgeClass(highestRiskModule.risk)
    });
  }

  if (strongestOwnerModule) {
    const owner = strongestOwnerModule.owners[0];
    cards.push({
      title: "Strongest owner signal",
      primary: owner ? owner.username : strongestOwnerModule.module_name,
      secondary: owner
        ? `${strongestOwnerModule.module_name} at ${formatPercent(owner.ownership_percent)} ownership.`
        : `Ownership is tracked for ${strongestOwnerModule.module_name}.`,
      meta: strongestOwnerModule.module_name
    });
  }

  if (strongestExpertModule) {
    const expert = strongestExpertModule.experts[0];
    cards.push({
      title: "Best review candidate",
      primary: expert ? expert.username : strongestExpertModule.module_name,
      secondary: expert
        ? `${strongestExpertModule.module_name} with expertise score ${expert.score}.`
        : `Expertise is tracked for ${strongestExpertModule.module_name}.`,
      meta: strongestExpertModule.module_name
    });
  }

  return cards.slice(0, 3);
}

function getHotspotSummary(hotspots) {
  const highestChurn = hotspots[0];
  const mostTouched = [...hotspots].sort((left, right) => right.commit_count - left.commit_count)[0];
  const largestAddition = [...hotspots].sort((left, right) => right.lines_added - left.lines_added)[0];
  const cards = [];

  if (highestChurn) {
    cards.push({
      title: "Highest churn file",
      primary: highestChurn.path,
      secondary: `${formatCount(highestChurn.churn)} total churn across ${formatCount(highestChurn.commit_count)} commits.`,
      meta: "Most volatile"
    });
  }

  if (mostTouched) {
    cards.push({
      title: "Most touched file",
      primary: mostTouched.path,
      secondary: `${formatCount(mostTouched.commit_count)} commit touches with ${formatCount(mostTouched.lines_deleted)} deleted lines.`,
      meta: "Most revisited"
    });
  }

  if (largestAddition) {
    cards.push({
      title: "Largest additions",
      primary: largestAddition.path,
      secondary: `${formatCount(largestAddition.lines_added)} lines added and ${formatCount(largestAddition.lines_deleted)} lines deleted.`,
      meta: "Highest growth"
    });
  }

  return cards.slice(0, 3);
}

function getCoChangeSummary(coChanges) {
  const strongestPair = coChanges[0];
  const mostRecentPair = [...coChanges]
    .filter((pair) => Boolean(pair.last_cochanged_at))
    .sort((left, right) => new Date(right.last_cochanged_at).getTime() - new Date(left.last_cochanged_at).getTime())[0];
  const broadestPair = [...coChanges].sort((left, right) => {
    const leftLength = `${left.left_path} ${left.right_path}`.length;
    const rightLength = `${right.left_path} ${right.right_path}`.length;
    return rightLength - leftLength;
  })[0];
  const cards = [];

  if (strongestPair) {
    cards.push({
      title: "Strongest pair",
      primary: `${strongestPair.left_path} ↔ ${strongestPair.right_path}`,
      secondary: `${formatCount(strongestPair.cochange_count)} shared commits in the current sync dataset.`,
      meta: "Highest overlap"
    });
  }

  if (mostRecentPair) {
    cards.push({
      title: "Most recently linked",
      primary: `${mostRecentPair.left_path} ↔ ${mostRecentPair.right_path}`,
      secondary: `Seen together ${formatRelativeDate(mostRecentPair.last_cochanged_at)}.`,
      meta: "Fresh signal"
    });
  }

  if (broadestPair) {
    cards.push({
      title: "Another likely dependency",
      primary: `${broadestPair.left_path} ↔ ${broadestPair.right_path}`,
      secondary: `${formatCount(broadestPair.cochange_count)} shared commits recorded so far.`,
      meta: "Investigate"
    });
  }

  return cards.slice(0, 3);
}

function mergeModuleAnalytics(moduleOwnership, moduleExpertise, moduleBusFactor) {
  const moduleMap = new Map();

  moduleOwnership.forEach((module) => {
    moduleMap.set(module.module_id, {
      module_id: module.module_id,
      module_name: module.module_name,
      path_prefix: module.path_prefix,
      bus_factor: module.bus_factor,
      active_contributors: module.active_contributors,
      top_owner_percent: module.top_owner_percent,
      risk: module.risk,
      owners: module.owners || [],
      experts: []
    });
  });

  moduleExpertise.forEach((module) => {
    const current = moduleMap.get(module.module_id) || {
      module_id: module.module_id,
      module_name: module.module_name,
      path_prefix: module.path_prefix,
      bus_factor: 0,
      active_contributors: 0,
      top_owner_percent: 0,
      risk: "unknown",
      owners: [],
      experts: []
    };
    current.experts = module.experts || [];
    moduleMap.set(module.module_id, current);
  });

  moduleBusFactor.forEach((module) => {
    const current = moduleMap.get(module.module_id) || {
      module_id: module.module_id,
      module_name: module.module_name,
      path_prefix: module.path_prefix,
      bus_factor: 0,
      active_contributors: 0,
      top_owner_percent: 0,
      risk: "unknown",
      owners: [],
      experts: []
    };
    current.bus_factor = module.bus_factor;
    current.active_contributors = module.active_contributors;
    current.top_owner_percent = module.top_owner_percent;
    current.risk = module.risk;
    moduleMap.set(module.module_id, current);
  });

  return Array.from(moduleMap.values()).sort((left, right) => {
    const leftRisk = riskOrder(left.risk);
    const rightRisk = riskOrder(right.risk);
    if (leftRisk !== rightRisk) {
      return leftRisk - rightRisk;
    }
    return left.module_name.localeCompare(right.module_name);
  });
}

function riskOrder(risk) {
  switch ((risk || "").toLowerCase()) {
    case "high":
      return 0;
    case "medium":
      return 1;
    case "low":
      return 2;
    default:
      return 3;
  }
}

function formatSyncStatusForBadge(status) {
  switch ((status || "").toLowerCase()) {
    case "queued":
      return "Queued";
    case "running":
    case "importing":
      return "Importing";
    case "succeeded":
    case "ready":
      return "Ready";
    case "failed":
      return "Failed";
    case "pending":
      return "Pending";
    default:
      return "Not synced";
  }
}

function formatCount(value) {
  if (!value) {
    return "0";
  }
  return new Intl.NumberFormat().format(value);
}

function formatDuration(durationMs) {
  if (!durationMs || durationMs <= 0) {
    return "Not available";
  }

  if (durationMs < 1000) {
    return `${durationMs} ms`;
  }

  const seconds = Math.round(durationMs / 1000);
  if (seconds < 60) {
    return `${seconds}s`;
  }

  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
}

function formatPercent(value) {
  if (value == null || Number.isNaN(Number(value))) {
    return "0%";
  }
  return `${Number(value).toFixed(1)}%`;
}

function formatModulePath(pathPrefix) {
  return pathPrefix && pathPrefix !== "." ? pathPrefix : "Repository root";
}

function formatRiskLabel(risk) {
  switch ((risk || "").toLowerCase()) {
    case "high":
      return "High";
    case "medium":
      return "Medium";
    case "low":
      return "Low";
    default:
      return "Unknown";
  }
}

function getRiskBadgeClass(risk) {
  switch ((risk || "").toLowerCase()) {
    case "high":
      return "dashboard-badge-danger";
    case "medium":
      return "dashboard-badge-warning";
    case "low":
      return "dashboard-badge-success";
    default:
      return "dashboard-badge-muted";
  }
}

function formatRelativeDate(value) {
  if (!value) {
    return "Not available";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "Not available";
  }

  const diffMs = Date.now() - date.getTime();
  const diffMinutes = Math.round(diffMs / 60000);

  if (diffMinutes < 1) {
    return "Just now";
  }
  if (diffMinutes < 60) {
    return `${diffMinutes}m ago`;
  }

  const diffHours = Math.round(diffMinutes / 60);
  if (diffHours < 24) {
    return `${diffHours}h ago`;
  }

  const diffDays = Math.round(diffHours / 24);
  if (diffDays < 7) {
    return `${diffDays}d ago`;
  }

  return date.toLocaleString();
}

function formatContributionLabel(count) {
  return `${formatCount(count)} contribution${count === 1 ? "" : "s"}`;
}

function getSyncTimingSummary(syncRun) {
  if (syncRun.completed_at) {
    return `completed ${formatRelativeDate(syncRun.completed_at)}`;
  }
  if (syncRun.started_at) {
    return `started ${formatRelativeDate(syncRun.started_at)}`;
  }
  return `was queued ${formatRelativeDate(syncRun.created_at)}`;
}

function formatSyncRunDetail(syncRun) {
  if (syncRun.status === "failed" && syncRun.error_message) {
    return syncRun.error_message;
  }

  const summary = syncRun.summary || {};
  const parts = [];

  if (summary.commits_count > 0) {
    parts.push(`${summary.commits_count} commits`);
  }
  if (summary.files_count > 0) {
    parts.push(`${summary.files_count} files`);
  }
  if (summary.contributors_count > 0) {
    parts.push(`${summary.contributors_count} contributors`);
  }

  if (parts.length > 0) {
    return `${parts.join(", ")} imported`;
  }

  return getSyncTimingSummary(syncRun);
}

function getSyncRunPillLabel(syncRun) {
  if (!syncRun?.status) {
    return "not synced";
  }

  switch (syncRun.status) {
    case "queued":
      return "queued";
    case "running":
      return "running";
    case "succeeded":
      return "succeeded";
    case "failed":
      return "failed";
    default:
      return syncRun.status;
  }
}

function getSyncRunMeta(repo, syncRun) {
  if (!syncRun) {
    return "No sync has been queued for this repository yet.";
  }

  if (syncRun.status === "queued") {
    return "Waiting for the sync worker to pick this up.";
  }

  if (syncRun.status === "running") {
    return "Importing contributors, commits, files, and modules.";
  }

  if (syncRun.status === "failed") {
    return syncRun.error_message || "The last sync ended with an error.";
  }

  if (syncRun.status === "succeeded") {
    const summary = syncRun.summary || {};
    const parts = [];

    if (summary.commits_count > 0) {
      parts.push(`${summary.commits_count} commits`);
    }
    if (summary.files_count > 0) {
      parts.push(`${summary.files_count} files`);
    }
    if (summary.contributors_count > 0) {
      parts.push(`${summary.contributors_count} contributors`);
    }

    if (parts.length > 0) {
      return `Last sync imported ${parts.join(", ")}.`;
    }

    return "Last sync completed successfully.";
  }

  switch ((repo.sync_status || "").toLowerCase()) {
    case "ready":
      return "Repository is connected and ready.";
    case "importing":
      return "Repository data is currently being imported.";
    case "failed":
      return "The previous sync failed. Queue another one to retry.";
    default:
      return "Repository is connected and waiting for its first sync.";
  }
}

createRoot(document.getElementById("app")).render(<App />);
