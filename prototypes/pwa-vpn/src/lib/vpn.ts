/**
 * VPN client — talks to Go backend /api/vpn/rpc (NOT fake browser WebRTC).
 * Modes:
 *  - local: start_local_tunnel (testable on Windows without wg)
 *  - exit:  connect_to_exit_node(publicKey, endpoint)
 *  - share: start_exit_node
 */
import { writable } from 'svelte/store'
import { API_BASE } from './api'

export type VpnUiStatus = 'disconnected' | 'connecting' | 'connected' | 'sharing' | 'error'

export interface VpnBackendStatus {
  state: string
  myIP?: string
  myPublicKey?: string
  peers?: Array<{
    id?: string
    name?: string
    publicKey?: string
    endpoint?: string
    isExitNode?: boolean
    online?: boolean
  }>
  uptime?: number
  bytesUp?: number
  bytesDown?: number
  transport?: string
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
  lastError: '',
  mode: '' as '' | 'local' | 'exit' | 'share',
})

let pollTimer: ReturnType<typeof setInterval> | null = null
let lastMode: '' | 'local' | 'exit' | 'share' = ''

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
    body: JSON.stringify(params ? { method, params } : { method }),
  })
  if (res.status === 401) {
    throw new Error('Нужна авторизация (JWT). Обновите страницу — signup создаст токен.')
  }
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(`VPN API HTTP ${res.status}: ${text.slice(0, 160)}`)
  }
  const body = await res.json()
  if (body?.error) {
    throw new Error(body.error.message || JSON.stringify(body.error))
  }
  return body.result as T
}

function mapState(state: string): VpnUiStatus {
  switch (state) {
    case 'connected': return 'connected'
    case 'sharing': return 'sharing'
    case 'connecting': return 'connecting'
    case 'error': return 'error'
    default: return 'disconnected'
  }
}

export async function refreshVPNStatus(): Promise<VpnBackendStatus | null> {
  try {
    const st = await vpnRpc<VpnBackendStatus>('get_status')
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
      mode: lastMode,
      lastError: '',
    }))
    return st
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    vpnStats.update(s => ({ ...s, lastError: msg }))
    return null
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => { void refreshVPNStatus() }, 2000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

/** Local test tunnel — works on native Windows without WireGuard install. */
export async function connectLocalVPN(): Promise<boolean> {
  vpnStatus.set('connecting')
  lastMode = 'local'
  try {
    await vpnRpc('start_local_tunnel')
    await refreshVPNStatus()
    startPolling()
    return true
  } catch (e) {
    console.error('VPN local connect:', e)
    vpnStatus.set('error')
    vpnStats.update(s => ({
      ...s,
      lastError: e instanceof Error ? e.message : String(e),
      mode: 'local',
    }))
    return false
  }
}

/** Connect to a remote exit node (needs base64/hex pubkey ≥16 chars + host:port). */
export async function connectExitVPN(publicKey: string, endpoint: string): Promise<boolean> {
  vpnStatus.set('connecting')
  lastMode = 'exit'
  try {
    if (!publicKey || publicKey.length < 16) throw new Error('Public key exit-node слишком короткий (мин. 16 символов)')
    if (!endpoint || !endpoint.includes(':')) throw new Error('Endpoint вида host:port обязателен')
    await vpnRpc('connect_to_exit_node', { publicKey, endpoint })
    await refreshVPNStatus()
    startPolling()
    return true
  } catch (e) {
    console.error('VPN exit connect:', e)
    vpnStatus.set('error')
    vpnStats.update(s => ({
      ...s,
      lastError: e instanceof Error ? e.message : String(e),
      mode: 'exit',
    }))
    return false
  }
}

/** Share this machine as exit node (userspace/stub on Windows). */
export async function shareExitNode(): Promise<boolean> {
  vpnStatus.set('connecting')
  lastMode = 'share'
  try {
    await vpnRpc('start_exit_node')
    await refreshVPNStatus()
    startPolling()
    return true
  } catch (e) {
    console.error('VPN share:', e)
    vpnStatus.set('error')
    vpnStats.update(s => ({
      ...s,
      lastError: e instanceof Error ? e.message : String(e),
      mode: 'share',
    }))
    return false
  }
}

export async function disconnectVPN(): Promise<void> {
  try {
    await vpnRpc('disconnect')
  } catch (e) {
    console.warn('VPN disconnect:', e)
  }
  stopPolling()
  lastMode = ''
  vpnStatus.set('disconnected')
  vpnStats.update(s => ({
    ...s,
    bytesIn: 0,
    bytesOut: 0,
    peers: 0,
    uptime: 0,
    mode: '',
    lastError: '',
  }))
}

/** Back-compat for old VpnPanel toggle */
export async function connectVPN(exitUrl?: string): Promise<boolean> {
  if (!exitUrl) return connectLocalVPN()
  // exitUrl historically was wss://... — if host:port use as endpoint with demo key slot
  if (exitUrl.includes('://')) {
    return connectLocalVPN()
  }
  // format: pubkey@host:port OR host:port
  if (exitUrl.includes('@')) {
    const [pk, ep] = exitUrl.split('@')
    return connectExitVPN(pk, ep)
  }
  return connectExitVPN('LOCAL_DEMO_PEER_KEY_XXXXXXXX', exitUrl)
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

// Initial soft refresh (no throw)
if (typeof window !== 'undefined') {
  setTimeout(() => { void refreshVPNStatus() }, 800)
}
