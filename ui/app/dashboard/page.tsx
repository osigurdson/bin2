"use client";

import {
  RedirectToSignIn,
  SignedIn,
  SignedOut,
  SignOutButton,
  useUser,
} from "@clerk/nextjs";
import Link from "next/link";

export default function DashboardPage() {
  const { isLoaded, user } = useUser();

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
          <p>This is a stub dashboard page for the logged-in state.</p>
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
