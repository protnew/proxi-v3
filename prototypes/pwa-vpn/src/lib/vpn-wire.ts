export const VPN_STATES = [
  'off',
  'helper_missing',
  'core_down',
  'connecting',
  'connected',
  'sharing',
  'reconnecting',
  'locked',
  'disconnecting',
  'error',
] as const

export type VpnWireState = (typeof VPN_STATES)[number]

export const VPN_ERROR_CODES = [
  'helper_not_installed',
  'access_denied',
  'wintun_missing',
  'route_add_failed',
  'no_exit_peers',
  'auth_required',
  'forbidden',
  'donor_offline',
  'unlock_failed',
  'unknown_state',
  'connect_timeout',
  'reconnect_exhausted',
] as const

export type VpnErrorCode = (typeof VPN_ERROR_CODES)[number]

export const VPN_PHASES = ['service', 'tun', 'routes', 'transport', 'exit-probe'] as const

export interface VpnWireStatus {
  state: string
  phase?: string
  attempt?: number
  max?: number
  code?: string
  leg?: string
  selfExit?: boolean
}

/** Backend synonyms → wire enum (BAG-56): Go core emits 'disconnected'. */
const STATE_ALIASES: Record<string, VpnWireState> = {
  disconnected: 'off',
}

export function mapWireStatus(raw: VpnWireStatus): { state: VpnWireState; code: string } {
  const alias = STATE_ALIASES[raw.state]
  const rawState = alias ?? raw.state
  const known = (VPN_STATES as readonly string[]).includes(rawState)
  if (!known) return { state: 'error', code: 'unknown_state' }
  const state = rawState as VpnWireState
  if (state === 'error') {
    const code = (VPN_ERROR_CODES as readonly string[]).includes(raw.code || '')
      ? (raw.code as string)
      : 'unknown_state'
    return { state, code }
  }
  return { state, code: '' }
}

/** Localized labels for all 12 wire error codes (RU / EN). */
export const ERROR_LABELS: Record<VpnErrorCode, { ru: string; en: string }> = {
  helper_not_installed: { ru: 'Служба-помощник не установлена', en: 'Helper service not installed' },
  access_denied:        { ru: 'Нет прав (запуск не от администратора)', en: 'Access denied (elevation required)' },
  wintun_missing:       { ru: 'Драйвер Wintun не найден', en: 'Wintun driver missing' },
  route_add_failed:     { ru: 'Не удалось установить маршруты', en: 'Failed to install routes' },
  no_exit_peers:        { ru: 'Нет доступных выходных узлов', en: 'No exit peers available' },
  auth_required:        { ru: 'Требуется вход (JWT)', en: 'Sign-in required (JWT)' },
  forbidden:            { ru: 'Недостаточно прав (admin-only)', en: 'Forbidden (admin-only)' },
  donor_offline:        { ru: 'Донор недоступен', en: 'Donor offline' },
  unlock_failed:        { ru: 'Не удалось снять блокировку', en: 'Unlock failed' },
  unknown_state:        { ru: 'Неизвестное состояние ядра', en: 'Unknown core state' },
  connect_timeout:      { ru: 'Таймаут подключения', en: 'Connection timeout' },
  reconnect_exhausted:  { ru: 'Повторные попытки исчерпаны', en: 'Reconnect attempts exhausted' },
}
