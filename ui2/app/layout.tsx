import type { Metadata } from "next";
import { Space_Mono } from "next/font/google";
import { AuthKitProvider } from "@workos-inc/authkit-nextjs/components";
import { withAuth } from "@workos-inc/authkit-nextjs";
import "./globals.css";

const spaceMono = Space_Mono({
  subsets: ["latin"],
  weight: ["400", "700"],
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
  const { accessToken: _accessToken, ...auth } = await withAuth();

  return (
    <html lang="en" className={spaceMono.className} data-theme="light">
      <body>
        <AuthKitProvider initialAuth={auth}>
          {children}
        </AuthKitProvider>
      </body>
    </html>
  );
}
