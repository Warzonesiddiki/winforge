"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { t as translate, type Locale, getLocales } from "@/lib/i18n";

interface LocaleContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string) => string;
  locales: { id: Locale; name: string }[];
  ready: boolean;
}

const LocaleContext = createContext<LocaleContextValue | null>(null);

/**
 * Client-side locale provider. Fetches the persisted language from the
 * /api/status endpoint on mount and exposes a t() function for components.
 * Falls back to en-US if the API is unreachable.
 */
export function LocaleProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>("en-US");
  const [ready, setReady] = useState(false);

  useEffect(() => {
    async function load() {
      try {
        // Check localStorage first for instant load
        const stored = typeof window !== "undefined" ? localStorage.getItem("wf-lang") : null;
        if (stored) {
          setLocaleState(stored as Locale);
        }
        const res = await fetch("/api/status", { cache: "no-store" });
        if (res.ok) {
          const data = await res.json();
          const serverLang = data.settings?.language ?? data.language;
          if (serverLang) {
            setLocaleState(serverLang as Locale);
          }
        }
      } catch {
        // Use localStorage or default
      } finally {
        setReady(true);
      }
    }
    load();
  }, []);

  function setLocale(newLocale: Locale) {
    setLocaleState(newLocale);
    try {
      localStorage.setItem("wf-lang", newLocale);
    } catch {
      // localStorage unavailable
    }
  }

  const value: LocaleContextValue = {
    locale,
    setLocale,
    t: (key: string) => translate(key, locale),
    locales: getLocales(),
    ready,
  };

  return <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>;
}

export function useLocale(): LocaleContextValue {
  const ctx = useContext(LocaleContext);
  if (!ctx) {
    // Safe fallback if provider isn't mounted — never throw
    return {
      locale: "en-US",
      setLocale: () => {},
      t: (key: string) => translate(key, "en-US"),
      locales: getLocales(),
      ready: true,
    };
  }
  return ctx;
}
