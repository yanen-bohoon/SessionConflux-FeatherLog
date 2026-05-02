type LocaleMessages = Record<string, string>;

const locales: Record<string, LocaleMessages> = {};
let current: string = 'zh';

export function registerLocale(lang: string, messages: LocaleMessages) {
  locales[lang] = messages;
}

export function setLocale(lang: string) {
  if (locales[lang]) current = lang;
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
