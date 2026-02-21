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
import { SyntheticEvent, useEffect, useState } from "react";

type Registry = {
  id: string;
  name: string;
};

type ListRegistriesResponse = {
  registries?: Registry[];
};

const apiBaseUrl = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:5000";

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
  const [isCheckingRegistries, setIsCheckingRegistries] = useState(true);
  const [isCreatingRegistry, setIsCreatingRegistry] = useState(false);
  const [registries, setRegistries] = useState<Registry[]>([]);
  const [registryName, setRegistryName] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isLoaded || !user) {
      return;
    }

    let active = true;
    const loadRegistries = async () => {
      setIsCheckingRegistries(true);
      setError(null);

      try {
        const token = await getToken();
        if (!token) {
          throw new Error("missing clerk token");
        }

        const res = await fetch(`${apiBaseUrl}/api/v1/registries`, {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        });

        if (!res.ok) {
          throw new Error(`registry lookup failed (${res.status})`);
        }

        const body = (await res.json()) as ListRegistriesResponse;
        if (!active) {
          return;
        }
        setRegistries(body.registries ?? []);
      } catch (err) {
        if (!active) {
          return;
        }
        console.error(err);
        setError("Could not load registries.");
      } finally {
        if (active) {
          setIsCheckingRegistries(false);
        }
      }
    };

    void loadRegistries();
    return () => {
      active = false;
    };
  }, [getToken, isLoaded, user?.id]);

  const handleCreateRegistry = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!user) {
      return;
    }

    const name = registryName.trim();
    if (name.length === 0) {
      setError("Please provide a registry name.");
      return;
    }

    setIsCreatingRegistry(true);
    setError(null);

    try {
      const token = await getToken();
      if (!token) {
        throw new Error("missing clerk token");
      }

      const res = await fetch(`${apiBaseUrl}/api/v1/registries`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name }),
      });

      if (res.status === 409) {
        setError("That registry name is already taken.");
        return;
      }
      if (!res.ok) {
        throw new Error(`registry create failed (${res.status})`);
      }

      const created = (await res.json()) as Registry;
      setRegistries([created]);
      setRegistryName("");
    } catch (err) {
      console.error(err);
      setError("Could not create registry.");
    } finally {
      setIsCreatingRegistry(false);
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
            Signed in as{" "}
            {user?.primaryEmailAddress?.emailAddress ?? "unknown user"}.
          </p>

          {isCheckingRegistries ? (
            <p>Checking your registries...</p>
          ) : registries.length === 0 ? (
            <form onSubmit={handleCreateRegistry} className="dashboard-onboarding-card">
              <p className="dashboard-onboarding-copy">
                You do not have a registry yet. Choose a registry name:
              </p>
              <label htmlFor="registry-name" className="dashboard-field-label">
                New Registry Name
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
              <button type="submit" disabled={isCreatingRegistry} className="dashboard-primary-btn">
                {isCreatingRegistry ? "Creating..." : "Create registry"}
              </button>
            </form>
          ) : (
            <div className="dashboard-section">
              <p>Registry: {registries[0]?.name}</p>
              <RegistryCommandPreview name={registries[0]?.name ?? ""} />
            </div>
          )}

          {error ? <p className="dashboard-error">{error}</p> : null}

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
