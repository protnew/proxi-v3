import { phaseFromVpnUi } from './connection-status'
/**
 * VPN client — Go /api/vpn/rpc
 * REAL mode = local SOCKS5 that carries app traffic (testable on Windows).
 */
import { writable } from 'svelte/store'
import { API_BASE } from './api'

export type VpnUiStatus = 'disconnected' | 'connecting' | 'connected' | 'sharing' | 'error' | 'core_down' | 'locked'

export interface VpnBackendStatus {
  state: string
  myIP?: string
  myPublicKey?: string
  peers?: unknown[]
  uptime?: number
  bytesUp?: number
  bytesDown?: number
  transport?: string
  socksAddr?: string
  upstream?: string
  mode?: string
  realTraffic?: boolean
}

export const vpnStatus = writable<VpnUiStatus>('disconnected')
export const vpnStats = writable({
  bytesIn: 0,
  bytesOut: 0,
  peers: 0,
  uptime: 0,
  myIP: '',
  myPublicKey: '',
  transport: '',
  socksAddr: '',
  upstream: '',
  mode: '' as string,
  realTraffic: false,
  lastError: '',
  egressIP: '',
})

let pollTimer: ReturnType<typeof setInterval> | null = null
let lastMode = ''

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('proxi_token') || ''
  const h: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) h['Authorization'] = `Bearer ${token}`
  return h
}

async function vpnRpc<T = unknown>(method: string, params?: Record<string, unknown>): Promise<T> {
  const res = await fetch(`${API_BASE}/api/vpn/rpc`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(params !== undefined ? { method, params } : { method }),
  })
  if (res.status === 401) throw new Error('Нужна авторизация (JWT)')
  if (!res.ok) throw new Error(`VPN API HTTP ${res.status}`)
  const body = await res.json()
  if (body?.error) throw new Error(body.error.message || JSON.stringify(body.error))
  return body.result as T
}

function mapState(state: string): VpnUiStatus {
  if (state === 'sharing' || state === 'core_down' || state === 'locked') return state
  const phase = phaseFromVpnUi(state)
  if (phase === 'connected') return 'connected'
  if (phase === 'connecting') return 'connecting'
  if (phase === 'failed') return 'error'
  return 'disconnected'
}

let pollFails = 0

export async function refreshVPNStatus(): Promise<VpnBackendStatus | null> {
  try {
    const st = await vpnRpc<VpnBackendStatus>('get_status')
    pollFails = 0
    vpnStatus.set(mapState(st?.state || 'disconnected'))
    vpnStats.update(s => ({
      ...s,
      bytesIn: st?.bytesDown || 0,
      bytesOut: st?.bytesUp || 0,
      peers: st?.peers?.length || 0,
      uptime: st?.uptime || 0,
      myIP: st?.myIP || '',
      myPublicKey: st?.myPublicKey || '',
      transport: st?.transport || '',
      socksAddr: st?.socksAddr || '',
      upstream: st?.upstream || '',
      mode: st?.mode || lastMode,
      realTraffic: !!st?.realTraffic,
      lastError: '',
    }))
    return st
  } catch (e) {
    pollFails++
    const msg = pollFails >= 3 ? 'Ядро не отвечает' : (e instanceof Error ? e.message : String(e))
    if (pollFails >= 3) vpnStatus.set('core_down')
    vpnStats.update(s => ({ ...s, lastError: msg }))
    return null
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => { void refreshVPNStatus() }, 2000)
}
function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

/** REAL tunnel: SOCKS5 on 127.0.0.1:10808 — app traffic goes through process */
export async function connectRealVPN(upstream = ''): Promise<boolean> {
  vpnStatus.set('connecting')
  lastMode = upstream ? 'exit' : 'real'
  try {
    await vpnRpc('start_real_tunnel', {
      listen: '127.0.0.1:10808',
      upstream: upstream || undefined,
    })
    await refreshVPNStatus()
    startPolling()
    return true
  } catch (e) {
    vpnStatus.set('error')
    vpnStats.update(s => ({ ...s, lastError: e instanceof Error ? e.message : String(e), mode: lastMode }))
    return false
  }
}

export async function connectLocalVPN(): Promise<boolean> {
  vpnStatus.set('connecting')
  lastMode = 'local'
  try {
    await vpnRpc('start_local_tunnel')
    await refreshVPNStatus()
    startPolling()
    return true
  } catch (e) {
    vpnStatus.set('error')
    vpnStats.update(s => ({ ...s, lastError: e instanceof Error ? e.message : String(e), mode: 'local' }))
    return false
  }
}

export async function connectExitVPN(publicKey: string, endpoint: string): Promise<boolean> {
  // endpoint as SOCKS upstream is the practical real path
  if (!endpoint.includes(':')) {
    vpnStats.update(s => ({ ...s, lastError: 'Endpoint host:port обязателен' }))
    vpnStatus.set('error')
    return false
  }
  // If pubkey empty/short → treat endpoint as upstream SOCKS
  if (!publicKey || publicKey.length < 32) {
    return connectRealVPN(endpoint)
  }
  vpnStatus.set('connecting')
  lastMode = 'exit'
  try {
    await vpnRpc('connect_to_exit_node', { publicKey, endpoint, upstream: '' })
    await refreshVPNStatus()
    startPolling()
    return true
  } catch (e) {
    // fallback: still raise local SOCKS so user can test
    const ok = await connectRealVPN('')
    if (!ok) {
      vpnStatus.set('error')
      vpnStats.update(s => ({ ...s, lastError: e instanceof Error ? e.message : String(e) }))
    }
    return ok
  }
}

export async function shareExitNode(): Promise<boolean> {
  vpnStatus.set('connecting')
  lastMode = 'share'
  try {
    await vpnRpc('start_exit_node')
    // also open local SOCKS for apps on this machine
    try { await vpnRpc('start_real_tunnel', { listen: '127.0.0.1:10808' }) } catch { /* optional */ }
    await refreshVPNStatus()
    startPolling()
    return true
  } catch (e) {
    vpnStatus.set('error')
    vpnStats.update(s => ({ ...s, lastError: e instanceof Error ? e.message : String(e), mode: 'share' }))
    return false
  }
}

export async function checkEgressIP(): Promise<string> {
  const r = await vpnRpc<{ ip: string }>('check_egress_ip')
  const ip = r?.ip || ''
  vpnStats.update(s => ({ ...s, egressIP: ip }))
  return ip
}

export async function disconnectVPN(): Promise<void> {
  let clean = false
  try {
    await vpnRpc('disconnect')
    const st = await vpnRpc<VpnBackendStatus>('get_status')
    clean = !st?.state || st.state === 'disconnected' || st.state === 'off'
  } catch (e) {
    console.warn(e)
  }
  stopPolling()
  lastMode = ''
  if (!clean) {
    vpnStatus.set('error')
    vpnStats.update(s => ({ ...s, lastError: 'Повторить отключение' }))
    return
  }
  vpnStatus.set('disconnected')
  vpnStats.update(s => ({
    ...s,
    bytesIn: 0, bytesOut: 0, peers: 0, uptime: 0,
    mode: '', realTraffic: false, socksAddr: '', upstream: '', egressIP: '', lastError: '',
  }))
}

/** default connect = REAL socks */
export async function connectVPN(exitUrl?: string): Promise<boolean> {
  if (!exitUrl) return connectRealVPN('')
  if (exitUrl.includes('://')) return connectRealVPN('')
  if (exitUrl.includes('@')) {
    const [pk, ep] = exitUrl.split('@')
    return connectExitVPN(pk, ep)
  }
  return connectRealVPN(exitUrl)
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB'
  return (bytes / 1073741824).toFixed(1) + ' GB'
}

export function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}

if (typeof window !== 'undefined') {
  setTimeout(() => { void refreshVPNStatus() }, 800)
}
