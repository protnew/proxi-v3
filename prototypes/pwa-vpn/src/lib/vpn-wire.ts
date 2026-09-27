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

export function mapWireStatus(raw: VpnWireStatus): { state: VpnWireState; code: string } {
  const known = (VPN_STATES as readonly string[]).includes(raw.state)
  if (!known) return { state: 'error', code: 'unknown_state' }
  const state = raw.state as VpnWireState
  if (state === 'error') {
    const code = (VPN_ERROR_CODES as readonly string[]).includes(raw.code || '')
      ? (raw.code as string)
      : 'unknown_state'
    return { state, code }
  }
  return { state, code: '' }
}
