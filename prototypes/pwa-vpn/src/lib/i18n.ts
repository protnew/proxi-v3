/** INF-002: minimal RU/EN i18n (SSR-safe) */
export type Locale = 'ru' | 'en'

const dict = {
  ru: {
    'app.title': 'Proxi',
    'vpn.connect': 'Подключить VPN',
    'vpn.disconnect': 'Отключить',
    'chat.new': 'Новый чат',
    'settings.title': 'Настройки',
    'settings.theme': 'Тема',
    'settings.language': 'Язык',
    'identity.backup': 'Сохраните nsec',
  },
  en: {
    'app.title': 'Proxi',
    'vpn.connect': 'Connect VPN',
    'vpn.disconnect': 'Disconnect',
    'chat.new': 'New chat',
    'settings.title': 'Settings',
    'settings.theme': 'Theme',
    'settings.language': 'Language',
    'identity.backup': 'Save your nsec',
  },
} as const

export type MsgKey = keyof typeof dict.ru

const LOCALE_KEY = 'proxi_locale'
let memLocale: Locale = 'ru'

export function getLocale(): Locale {
  try {
    if (typeof localStorage !== 'undefined') {
      const v = localStorage.getItem(LOCALE_KEY)
      if (v === 'en' || v === 'ru') return v
    }
  } catch { /* */ }
  return memLocale
}

export function setLocale(l: Locale): void {
  memLocale = l
  try {
    if (typeof localStorage !== 'undefined') localStorage.setItem(LOCALE_KEY, l)
  } catch { /* */ }
}

export function t(key: MsgKey, locale?: Locale): string {
  const loc = locale ?? getLocale()
  return dict[loc][key] ?? dict.ru[key] ?? key
}
