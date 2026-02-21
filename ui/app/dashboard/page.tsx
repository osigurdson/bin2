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
import { FormEvent, useEffect, useState } from "react";

type Registry = {
  id: string;
  name: string;
};

type ListRegistriesResponse = {
  registries?: Registry[];
};

const apiBaseUrl = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:5000";

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

  const handleCreateRegistry = async (e: FormEvent<HTMLFormElement>) => {
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
      <main className="container" style={{ paddingTop: "3rem" }}>
        <h1>Dashboard</h1>
        <p>Loading session...</p>
      </main>
    );
  }

  return (
    <>
      <SignedOut>
        <main className="container" style={{ paddingTop: "3rem" }}>
          <h1>Dashboard</h1>
          <p>Redirecting to sign in...</p>
          <RedirectToSignIn />
        </main>
      </SignedOut>

      <SignedIn>
        <main className="container" style={{ paddingTop: "3rem" }}>
          <h1>Dashboard</h1>
          <p>
            Signed in as{" "}
            {user?.primaryEmailAddress?.emailAddress ?? "unknown user"}.
          </p>

          {isCheckingRegistries ? (
            <p>Checking your registries...</p>
          ) : registries.length === 0 ? (
            <form
              onSubmit={handleCreateRegistry}
              style={{
                marginTop: "1rem",
                display: "flex",
                flexDirection: "column",
                gap: "0.75rem",
                maxWidth: "420px",
              }}
            >
              <p>You do not have a registry yet. Choose a registry name:</p>
              <input
                type="text"
                value={registryName}
                onChange={(e) => setRegistryName(e.target.value)}
                placeholder="my-registry"
                autoComplete="off"
              />
              <button type="submit" disabled={isCreatingRegistry}>
                {isCreatingRegistry ? "Creating..." : "Create registry"}
              </button>
            </form>
          ) : (
            <div style={{ marginTop: "1rem" }}>
              <p>Registry: {registries[0]?.name}</p>
            </div>
          )}

          {error ? <p style={{ marginTop: "0.75rem" }}>{error}</p> : null}

          <p style={{ marginTop: "1rem", display: "flex", gap: "0.75rem" }}>
            <Link href="/">Back to landing</Link>
            <SignOutButton>
              <button type="button">Sign out</button>
            </SignOutButton>
          </p>
        </main>
      </SignedIn>
    </>
  );
}
