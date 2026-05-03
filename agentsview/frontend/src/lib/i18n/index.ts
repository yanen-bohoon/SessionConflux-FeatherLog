type LocaleMessages = Record<string, string>;

const locales: Record<string, LocaleMessages> = {};
let current: string = 'en';

export function registerLocale(lang: string, messages: LocaleMessages) {
  locales[lang] = messages;
}

export function initLocale(lang: string) {
  if (!locales[lang]) return;
  current = lang;
}

export function setLocale(lang: string) {
  if (!locales[lang]) return;
  if (current === lang) return;
  current = lang;
  try { localStorage.setItem("agentsview-locale", lang); } catch { /* noop */ }
  window.location.reload();
}

export function getLocale(): string {
  return current;
}

export function t(key: string, params?: Record<string, string | number>): string {
  const msg = locales[current]?.[key] ?? locales['en']?.[key] ?? key;
  if (!params) return msg;
  return msg.replace(/\{(\w+)\}/g, (_, k) => String(params[k] ?? `{${k}}`));
}

export const _ = t;
