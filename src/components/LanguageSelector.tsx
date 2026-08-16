"use client";

import { useLocale } from "@/components/LocaleProvider";
import type { Locale } from "@/lib/i18n";

export function LanguageSelector() {
  const { locale, setLocale, locales, t } = useLocale();

  return (
    <div className="flex items-center justify-between">
      <label htmlFor="language-select" className="text-sm text-slate-300">
        {t("settings.language")}
      </label>
      <select
        id="language-select"
        value={locale}
        onChange={(e) => setLocale(e.target.value as Locale)}
        className="rounded-lg border border-white/10 bg-white/5 px-2 py-1 text-sm text-white"
        aria-label={t("settings.language")}
      >
        {locales.map((l) => (
          <option key={l.id} value={l.id}>
            {l.name}
          </option>
        ))}
      </select>
    </div>
  );
}

export default LanguageSelector;
