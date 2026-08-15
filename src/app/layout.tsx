import type { Metadata } from "next";
import type { ReactNode } from "react";
import "./globals.css";
import { Sidebar } from "@/components/Sidebar";
import { ToastProvider } from "@/components/Toast";
import { KeyboardShortcuts } from "@/components/KeyboardShortcuts";
import { GlobalSearch } from "@/components/GlobalSearch";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { LocaleProvider } from "@/components/LocaleProvider";

export const metadata: Metadata = {
  title: "WinForge Elite — Control Center",
  description:
    "The definitive all-in-one Windows optimization, debloat, privacy hardening, repair, and power-user configuration suite.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" className="dark" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem('wf-theme');if(t==='light'||t==='system'){document.documentElement.style.colorScheme=t==='light'?'light':'light dark';}}catch(e){}})();`,
          }}
        />
      </head>
      <body className="min-h-screen bg-[#05070c] text-slate-100 antialiased">
        <ToastProvider>
          <LocaleProvider>
            <div className="flex min-h-screen flex-col lg:flex-row">
            <Sidebar />
            <ErrorBoundary>
              <main className="min-w-0 flex-1 overflow-x-hidden">{children}</main>
            </ErrorBoundary>
            </div>
            <KeyboardShortcuts />
            <GlobalSearch />
          </LocaleProvider>
        </ToastProvider>
      </body>
    </html>
  );
}
