"use client";

import {
  RedirectToSignIn,
  SignedIn,
  SignedOut,
  SignOutButton,
  useAuth,
  useUser,
} from "@clerk/nextjs";
import Link from "next/link";
import { SyntheticEvent, useEffect, useMemo, useState } from "react";

type Registry = {
  id: string;
  name: string;
};

type APIKey = {
  id: string;
  keyName: string;
  prefix: string;
  createdAt: string;
  lastUsedAt: string | null;
  scopes: APIKeyScope[];
};

type APIKeyScope = {
  registryId: string;
  repository: string | null;
  permission: "read" | "write" | "admin";
  createdAt: string;
};

type CreateAPIKeyResponse = {
  apiKey: APIKey;
  secretKey: string;
};

type AddRegistryResponse = {
  id: string;
  name: string;
  apiKey: APIKey;
  secretKey: string;
};

type ListRegistriesResponse = {
  registries?: Registry[];
};

type ListAPIKeysResponse = {
  keys?: APIKey[];
};

type OnboardingSetup = {
  registryName: string;
  apiKeyName: string;
  secretKey: string;
};

const keyNameRe = /^[A-Za-z0-9._-]{2,8}$/;
const localAPIBaseURL = "http://localhost:5000";
const productionAPIBaseURL = "https://bin2.nthesis.ai";

function resolveAPIBaseURL(): string {
  const explicit = process.env.NEXT_PUBLIC_API_BASE_URL;
  if (explicit && explicit.trim()) {
    return explicit.trim();
  }

  if (typeof window !== "undefined") {
    const host = window.location.hostname;
    if (host === "localhost" || host === "127.0.0.1") {
      return localAPIBaseURL;
    }
  }

  return productionAPIBaseURL;
}

function formatDate(ts: string | null): string {
  if (!ts) {
    return "Never used";
  }

  const date = new Date(ts);
  if (Number.isNaN(date.getTime())) {
    return "Unknown";
  }

  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function describeScope(scope: APIKeyScope): string {
  const target = scope.repository ? scope.repository : "entire registry";
  return `${scope.permission} on ${target}`;
}

function RegistryCommandPreview({ name }: { name: string }) {
  const trimmed = name.trim();
  const hasName = trimmed !== "";
  const displayName = hasName ? trimmed : "{their-name}";

  return (
    <div className={`dashboard-preview${hasName ? " is-live" : ""}`}>
      <p className="dashboard-preview-title">
        {hasName ? "Live command preview:" : "Use these commands:"}
      </p>
      <code className="dashboard-command">
        docker push push.bin2.io/{displayName}
      </code>
      <code className="dashboard-command">
        docker pull bin2.io/{displayName}
      </code>
    </div>
  );
}

export default function DashboardPage() {
  const { isLoaded, user } = useUser();
  const { getToken } = useAuth();
  const apiBaseUrl = useMemo(() => resolveAPIBaseURL(), []);

  const [isCheckingRegistries, setIsCheckingRegistries] = useState(true);
  const [isCompletingOnboarding, setIsCompletingOnboarding] = useState(false);
  const [registries, setRegistries] = useState<Registry[]>([]);
  const [registryName, setRegistryName] = useState("");
  const [registryError, setRegistryError] = useState<string | null>(null);

  const [isLoadingAPIKeys, setIsLoadingAPIKeys] = useState(true);
  const [isCreatingAPIKey, setIsCreatingAPIKey] = useState(false);
  const [deletingKeyIDs, setDeletingKeyIDs] = useState<Set<string>>(new Set());
  const [apiKeys, setAPIKeys] = useState<APIKey[]>([]);
  const [apiKeyName, setAPIKeyName] = useState("");
  const [apiKeyError, setAPIKeyError] = useState<string | null>(null);
  const [newSecretKey, setNewSecretKey] = useState<string | null>(null);
  const [isSecretVisible, setIsSecretVisible] = useState(false);
  const [isSecretCopied, setIsSecretCopied] = useState(false);

  const [onboardingSetup, setOnboardingSetup] = useState<OnboardingSetup | null>(null);
  const [copiedCommandKey, setCopiedCommandKey] = useState<string | null>(null);

  const activeRegistry = registries[0] ?? null;
  const registryNamespace = activeRegistry?.name ?? "";
  const registryForCommands = registryNamespace || "{your-registry}";

  const maskedSecretKey = useMemo(() => {
    if (!newSecretKey) {
      return "";
    }
    if (newSecretKey.length <= 8) {
      return "••••••••";
    }
    return `${newSecretKey.slice(0, 8)}••••••••`;
  }, [newSecretKey]);

  const isOnboardingStep1 = !isCheckingRegistries && registries.length === 0 && !onboardingSetup;
  const isOnboardingStep2 = onboardingSetup !== null;

  const onboardingDockerLoginCommand = onboardingSetup
    ? `docker login push.bin2.io -u ${onboardingSetup.registryName} -p '${onboardingSetup.secretKey}'`
    : "";
  const onboardingPodmanLoginCommand = onboardingSetup
    ? `podman login push.bin2.io -u ${onboardingSetup.registryName} -p '${onboardingSetup.secretKey}'`
    : "";

  useEffect(() => {
    if (!isLoaded || !user) {
      return;
    }

    let active = true;
    const loadDashboardData = async () => {
      setIsCheckingRegistries(true);
      setIsLoadingAPIKeys(true);
      setRegistryError(null);
      setAPIKeyError(null);

      try {
        const token = await getToken();
        if (!token) {
          throw new Error("missing token");
        }

        const registryRequest = fetch(`${apiBaseUrl}/api/v1/registries`, {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        });

        const keyRequest = fetch(`${apiBaseUrl}/api/v1/api-keys`, {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        });

        const [registryResult, keyResult] = await Promise.allSettled([
          registryRequest,
          keyRequest,
        ]);

        if (!active) {
          return;
        }

        if (registryResult.status === "fulfilled") {
          if (!registryResult.value.ok) {
            console.error(new Error(`registry lookup failed (${registryResult.value.status})`));
            setRegistryError("Could not load registries.");
          } else {
            const body = (await registryResult.value.json()) as ListRegistriesResponse;
            setRegistries(body.registries ?? []);
          }
        } else {
          console.error(registryResult.reason);
          setRegistryError("Could not load registries.");
        }

        if (keyResult.status === "fulfilled") {
          if (!keyResult.value.ok) {
            console.error(new Error(`api key lookup failed (${keyResult.value.status})`));
            setAPIKeyError("Could not load API keys.");
          } else {
            const body = (await keyResult.value.json()) as ListAPIKeysResponse;
            setAPIKeys(body.keys ?? []);
          }
        } else {
          console.error(keyResult.reason);
          setAPIKeyError("Could not load API keys.");
        }
      } catch (err) {
        if (!active) {
          return;
        }
        console.error(err);
        setRegistryError("Could not load registries.");
        setAPIKeyError("Could not load API keys.");
      } finally {
        if (active) {
          setIsCheckingRegistries(false);
          setIsLoadingAPIKeys(false);
        }
      }
    };

    void loadDashboardData();
    return () => {
      active = false;
    };
  }, [apiBaseUrl, getToken, isLoaded, user?.id]);

  const validateAPIKeyName = (name: string): string | null => {
    if (!keyNameRe.test(name)) {
      return "API key name must be 2-8 chars of letters, numbers, '.', '_' or '-'.";
    }

    if (apiKeys.some((key) => key.keyName === name)) {
      return "An API key with that name already exists.";
    }

    return null;
  };

  const createScopedAdminKey = async (
    token: string,
    registry: Registry,
    keyName: string,
  ): Promise<Response> => {
    return fetch(`${apiBaseUrl}/api/v1/api-keys`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        keyName,
        scopes: [
          {
            registryId: registry.id,
            permission: "admin",
          },
        ],
      }),
    });
  };

  const handleCreateRegistry = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!user) {
      return;
    }

    const name = registryName.trim();
    if (name.length === 0) {
      setRegistryError("Please provide a registry name.");
      return;
    }

    setIsCompletingOnboarding(true);
    setRegistryError(null);
    setAPIKeyError(null);

    try {
      const token = await getToken();
      if (!token) {
        throw new Error("missing token");
      }

      const registryRes = await fetch(`${apiBaseUrl}/api/v1/registries`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name }),
      });

      if (registryRes.status === 409) {
        setRegistryError("That registry name is already taken.");
        return;
      }
      if (!registryRes.ok) {
        throw new Error(`registry create failed (${registryRes.status})`);
      }

      const result = (await registryRes.json()) as AddRegistryResponse;
      setRegistries([{ id: result.id, name: result.name }]);
      setRegistryName("");
      setAPIKeys((previous) => [result.apiKey, ...previous]);
      setNewSecretKey(result.secretKey);
      setIsSecretVisible(false);
      setIsSecretCopied(false);
      setOnboardingSetup({
        registryName: result.name,
        apiKeyName: result.apiKey.keyName,
        secretKey: result.secretKey,
      });
    } catch (err) {
      console.error(err);
      setRegistryError("Could not create registry.");
    } finally {
      setIsCompletingOnboarding(false);
    }
  };

  const handleCreateAPIKey = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();

    const name = apiKeyName.trim();
    const validationError = validateAPIKeyName(name);
    if (validationError) {
      setAPIKeyError(validationError);
      return;
    }

    setIsCreatingAPIKey(true);
    setAPIKeyError(null);

    try {
      if (!activeRegistry) {
        setAPIKeyError("Create a registry before creating an API key.");
        return;
      }

      const token = await getToken();
      if (!token) {
        throw new Error("missing token");
      }

      const res = await createScopedAdminKey(token, activeRegistry, name);

      if (res.status === 409) {
        setAPIKeyError("An API key with that name already exists.");
        return;
      }
      if (!res.ok) {
        throw new Error(`api key create failed (${res.status})`);
      }

      const created = (await res.json()) as CreateAPIKeyResponse;
      setAPIKeys((previous) => [created.apiKey, ...previous]);
      setNewSecretKey(created.secretKey);
      setIsSecretVisible(false);
      setIsSecretCopied(false);
      setAPIKeyName("");
    } catch (err) {
      console.error(err);
      setAPIKeyError("Could not create API key.");
    } finally {
      setIsCreatingAPIKey(false);
    }
  };

  const handleDeleteAPIKey = async (id: string) => {
    const shouldDelete = window.confirm("Delete this API key? This cannot be undone.");
    if (!shouldDelete) {
      return;
    }

    setDeletingKeyIDs((previous) => {
      const next = new Set(previous);
      next.add(id);
      return next;
    });
    setAPIKeyError(null);

    try {
      const token = await getToken();
      if (!token) {
        throw new Error("missing token");
      }

      const res = await fetch(`${apiBaseUrl}/api/v1/api-keys/${id}`, {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (res.status !== 204 && res.status !== 404) {
        throw new Error(`api key delete failed (${res.status})`);
      }

      setAPIKeys((previous) => previous.filter((item) => item.id !== id));
    } catch (err) {
      console.error(err);
      setAPIKeyError("Could not delete API key.");
    } finally {
      setDeletingKeyIDs((previous) => {
        const next = new Set(previous);
        next.delete(id);
        return next;
      });
    }
  };

  const copyText = async (text: string, feedbackKey: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedCommandKey(feedbackKey);
      window.setTimeout(() => setCopiedCommandKey((previous) => (
        previous === feedbackKey ? null : previous
      )), 1800);
    } catch (err) {
      console.error(err);
      setRegistryError("Could not copy to clipboard.");
    }
  };

  const handleCopySecret = async () => {
    if (!newSecretKey) {
      return;
    }

    try {
      await navigator.clipboard.writeText(newSecretKey);
      setIsSecretCopied(true);
      window.setTimeout(() => setIsSecretCopied(false), 1800);
    } catch (err) {
      console.error(err);
      setAPIKeyError("Could not copy API key to clipboard.");
    }
  };

  if (!isLoaded) {
    return (
      <main className="container dashboard-main">
        <h1>Dashboard</h1>
        <p>Loading session...</p>
      </main>
    );
  }

  return (
    <>
      <SignedOut>
        <main className="container dashboard-main">
          <h1>Dashboard</h1>
          <p>Redirecting to sign in...</p>
          <RedirectToSignIn />
        </main>
      </SignedOut>

      <SignedIn>
        <main className="container dashboard-main">
          <h1>Dashboard</h1>
          <p>
            Signed in as {" "}
            {user?.primaryEmailAddress?.emailAddress ?? "unknown user"}.
          </p>

          {isCheckingRegistries ? (
            <p>Checking your registries...</p>
          ) : isOnboardingStep1 ? (
            <form onSubmit={handleCreateRegistry} className="dashboard-onboarding-card">
              <p className="dashboard-onboarding-step">Step 1 of 2</p>
              <p className="dashboard-onboarding-copy">
                Choose your registry name.
              </p>
              <p className="dashboard-onboarding-note">
                This name cannot be changed later.
              </p>
              <label htmlFor="registry-name" className="dashboard-field-label">
                Registry Name
              </label>
              <input
                id="registry-name"
                type="text"
                value={registryName}
                onChange={(e) => setRegistryName(e.target.value)}
                placeholder="my-registry"
                autoComplete="off"
                className="dashboard-field-input"
              />
              <RegistryCommandPreview name={registryName} />
              <button type="submit" disabled={isCompletingOnboarding} className="dashboard-primary-btn">
                {isCompletingOnboarding ? "Setting up..." : "Continue"}
              </button>
            </form>
          ) : isOnboardingStep2 && onboardingSetup ? (
            <section className="dashboard-onboarding-card dashboard-onboarding-card-wide">
              <p className="dashboard-onboarding-step">Step 2 of 2</p>
              <p className="dashboard-onboarding-copy">
                Your registry <code>{onboardingSetup.registryName}</code> is ready.
              </p>
              <p className="dashboard-onboarding-note">
                We created your default API key <code>{onboardingSetup.apiKeyName}</code>. Copy one of these login commands.
              </p>

              <div className="dashboard-onboarding-commands">
                <div className="dashboard-onboarding-command-row">
                  <code className="dashboard-command">{onboardingDockerLoginCommand}</code>
                  <button
                    type="button"
                    className="dashboard-secondary-btn"
                    onClick={() => copyText(onboardingDockerLoginCommand, "docker-login")}
                  >
                    {copiedCommandKey === "docker-login" ? "Copied" : "Copy"}
                  </button>
                </div>
                <div className="dashboard-onboarding-command-row">
                  <code className="dashboard-command">{onboardingPodmanLoginCommand}</code>
                  <button
                    type="button"
                    className="dashboard-secondary-btn"
                    onClick={() => copyText(onboardingPodmanLoginCommand, "podman-login")}
                  >
                    {copiedCommandKey === "podman-login" ? "Copied" : "Copy"}
                  </button>
                </div>
              </div>

              <div className="dashboard-preview dashboard-api-quickstart">
                <p className="dashboard-preview-title">After login</p>
                <code className="dashboard-command">
                  docker push push.bin2.io/{onboardingSetup.registryName}/my-image:latest
                </code>
                <code className="dashboard-command">
                  docker pull bin2.io/{onboardingSetup.registryName}/my-image:latest
                </code>
              </div>

              <div className="dashboard-onboarding-actions">
                <button
                  type="button"
                  className="dashboard-primary-btn"
                  onClick={() => setOnboardingSetup(null)}
                >
                  Go to dashboard
                </button>
              </div>
            </section>
          ) : (
            <div className="dashboard-section">
              <p>Registry: {registries[0]?.name}</p>
              <RegistryCommandPreview name={registries[0]?.name ?? ""} />
            </div>
          )}

          {registryError ? <p className="dashboard-error">{registryError}</p> : null}

          {!isOnboardingStep1 && !isOnboardingStep2 ? (
            <section className="dashboard-api-card">
              <div className="dashboard-api-heading">
                <h2>API Keys</h2>
                <p>Create and manage keys for docker login and registry pushes/pulls.</p>
                <p>Keys created here get admin access to <code>{registryForCommands}</code>.</p>
              </div>

              <form onSubmit={handleCreateAPIKey} className="dashboard-api-create-form">
                <label htmlFor="api-key-name" className="dashboard-field-label">API Key Name</label>
                <div className="dashboard-api-create-row">
                  <input
                    id="api-key-name"
                    type="text"
                    value={apiKeyName}
                    onChange={(e) => {
                      setAPIKeyName(e.target.value);
                      if (apiKeyError) {
                        setAPIKeyError(null);
                      }
                    }}
                    placeholder="ci-key"
                    autoComplete="off"
                    className="dashboard-field-input"
                    disabled={isCreatingAPIKey}
                  />
                  <button type="submit" disabled={isCreatingAPIKey} className="dashboard-primary-btn">
                    {isCreatingAPIKey ? "Generating..." : "Generate key"}
                  </button>
                </div>
              </form>

              {newSecretKey ? (
                <div className="dashboard-secret-card">
                  <p className="dashboard-secret-title">New API key generated</p>
                  <p className="dashboard-secret-help">Copy this now. It will not be shown again.</p>
                  <code className="dashboard-secret-value">
                    {isSecretVisible ? newSecretKey : maskedSecretKey}
                  </code>
                  <div className="dashboard-secret-actions">
                    <button
                      type="button"
                      className="dashboard-secondary-btn"
                      onClick={() => setIsSecretVisible((value) => !value)}
                    >
                      {isSecretVisible ? "Hide" : "Show"}
                    </button>
                    <button
                      type="button"
                      className="dashboard-secondary-btn"
                      onClick={handleCopySecret}
                    >
                      {isSecretCopied ? "Copied" : "Copy"}
                    </button>
                    <button
                      type="button"
                      className="dashboard-secondary-btn"
                      onClick={() => {
                        setNewSecretKey(null);
                        setIsSecretVisible(false);
                        setIsSecretCopied(false);
                      }}
                    >
                      Dismiss
                    </button>
                  </div>
                </div>
              ) : null}

              <div className="dashboard-preview dashboard-api-quickstart">
                <p className="dashboard-preview-title">Quick start</p>
                <code className="dashboard-command">
                  docker login push.bin2.io -u {registryForCommands} -p {newSecretKey ? "&lt;your-api-key&gt;" : "&lt;api-key&gt;"}
                </code>
                <code className="dashboard-command">
                  docker push push.bin2.io/{registryForCommands}/my-image:latest
                </code>
                <code className="dashboard-command">
                  docker pull bin2.io/{registryForCommands}/my-image:latest
                </code>
              </div>

              <div className="dashboard-api-list-wrap">
                <h3>Active API Keys</h3>
                {isLoadingAPIKeys ? (
                  <p>Loading API keys...</p>
                ) : apiKeys.length === 0 ? (
                  <p>No API keys yet. Generate one to get started.</p>
                ) : (
                  <ul className="dashboard-api-list">
                    {apiKeys.map((apiKey) => {
                      const isDeleting = deletingKeyIDs.has(apiKey.id);
                      return (
                        <li key={apiKey.id} className="dashboard-api-list-item">
                          <div className="dashboard-api-list-meta">
                            <p className="dashboard-api-key-name">{apiKey.keyName}</p>
                            <p className="dashboard-api-key-dates">
                              Created {formatDate(apiKey.createdAt)} · Last used {formatDate(apiKey.lastUsedAt)}
                            </p>
                            <p className="dashboard-api-key-dates">
                              {apiKey.scopes.length > 0
                                ? apiKey.scopes.map(describeScope).join(" · ")
                                : "No scopes"}
                            </p>
                          </div>
                          <button
                            type="button"
                            className="dashboard-danger-btn"
                            onClick={() => handleDeleteAPIKey(apiKey.id)}
                            disabled={isDeleting}
                          >
                            {isDeleting ? "Deleting..." : "Delete"}
                          </button>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </div>

              {apiKeyError ? <p className="dashboard-error">{apiKeyError}</p> : null}
            </section>
          ) : null}

          <p className="dashboard-link-row">
            <Link href="/">Back to landing</Link>
            <SignOutButton>
              <button type="button" className="dashboard-secondary-btn">
                Sign out
              </button>
            </SignOutButton>
          </p>
        </main>
      </SignedIn>
    </>
  );
}
