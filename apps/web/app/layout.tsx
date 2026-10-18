import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { headers } from "next/headers";
import { Toaster } from "sonner";
import { Toaster as ShadcnToaster } from "@/components/ui/toaster";
import { ThemeProvider } from "@/components/theme-provider";
import { CSP_HEADER } from "@/lib/content-security-policy";
import { pageApiConfig, runtimeConfigScript } from "@/lib/runtime-config";
import "./globals.css";

// Inter is the fallback for platforms without SF Pro; see --font-sans in globals.css.
const inter = Inter({ subsets: ["latin"], variable: "--font-inter", display: "swap" });

export const metadata: Metadata = {
  title: "Agent Identity Management | OpenA2A",
  description:
    "AIM for a better security. Identity verification and security platform for AI agents and MCP servers.",
  icons: {
    icon: "/icon.svg",
    shortcut: "/icon.svg",
    apple: "/icon.svg",
  },
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Reading the request's headers makes Next.js render each page when it is requested,
  // so the environment read below is the running server's, not the build's.
  const host = (await headers()).get("host");
  // The API URL the server's environment names, if any, and the page's connect-src
  // policy (lib/runtime-config.ts).
  const { apiUrl, connectPolicy } = pageApiConfig(process.env, host);
  return (
    // suppressHydrationWarning: next-themes sets the theme class on <html> before hydration.
    <html lang="en" suppressHydrationWarning>
      <head>
        <meta httpEquiv={CSP_HEADER} content={connectPolicy} />
        {apiUrl ? <script dangerouslySetInnerHTML={{ __html: runtimeConfigScript(apiUrl) }} /> : null}
      </head>
      <body className={`${inter.variable} font-sans`}>
        <ThemeProvider>
          {children}
          <Toaster
            position="top-right"
            richColors
            closeButton
            expand={false}
            duration={4000}
          />
          <ShadcnToaster />
        </ThemeProvider>
      </body>
    </html>
  );
}
