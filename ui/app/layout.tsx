import type { Metadata } from "next";
import { Space_Mono } from "next/font/google";
import { AuthKitProvider } from "@workos-inc/authkit-nextjs/components";
import { withAuth } from "@workos-inc/authkit-nextjs";
import "./globals.css";
import { PostHogProvider } from "@/providers/PostHogProvider";

const spaceMono = Space_Mono({
  subsets: ["latin"],
  weight: ["400", "700"],
  variable: "--font-space-mono",
});


export const metadata: Metadata = {
  title: 'bin2',
  description: 'Cheap / fast container registry',
  icons: {
    icon: '/favicon.svg',
  },
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const { accessToken, ...auth } = await withAuth();
  void accessToken;

  return (
    <html lang="en" className={`${spaceMono.className} ${spaceMono.variable}`}>
      <body>
        <AuthKitProvider initialAuth={auth}>
          <PostHogProvider>
            {children}
          </PostHogProvider>
        </AuthKitProvider>
      </body>
    </html>
  );
}
