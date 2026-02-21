import type { Metadata } from "next";
import { ClerkProvider } from "@clerk/nextjs";
import "./tailwind.css";
import "./globals.css";
import "./marketing.css";

export const metadata: Metadata = {
  title: "bin2 - Container Registry",
  description: "the ridiculously cheap, fast and simple container registry",
  icons: {
    icon: "/favicon.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <ClerkProvider
      signInForceRedirectUrl="/dashboard"
      signUpForceRedirectUrl="/dashboard"
      afterSignOutUrl="/"
    >
      <html lang="en">
        <body>{children}</body>
      </html>
    </ClerkProvider>
  );
}
