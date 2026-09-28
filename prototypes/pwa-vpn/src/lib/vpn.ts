import { phaseFromVpnUi } from './connection-status'
import { mapWireStatus, type VpnWireState } from './vpn-wire'
/**
 * VPN client — Go /api/vpn/rpc
 * REAL mode = local SOCKS5 that carries app traffic (testable on Windows).
 */
import { writable } from 'svelte/store'
import * as secp from '@noble/secp256k1'
import { API_BASE, getSeckey } from './api'

export type VpnUiStatus = VpnWireState

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
  peerLost?: boolean
  activeLeg?: string
  code?: string
  phase?: string
  attempt?: number
  max?: number
}

export const vpnStatus = writable<VpnUiStatus>('off')
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
  lastErrorCode: '',
  phase: '',
  attempt: 0,
  max: 0,
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

class VpnRpcError extends Error {
  httpStatus?: number
  constructor(msg: string, httpStatus?: number) {
    super(msg)
    this.httpStatus = httpStatus
  }
}

async function vpnRpc<T = unknown>(method: string, params?: Record<string, unknown>): Promise<T> {
  const res = await fetch(`${API_BASE}/api/vpn/rpc`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(params !== undefined ? { method, params } : { method }),
  })
  if (res.status === 401) throw new VpnRpcError('Нужна авторизация (JWT)', 401)
  if (res.status === 403) throw new VpnRpcError('Нет прав (admin-only)', 403)
  if (!res.ok) throw new VpnRpcError(`VPN API HTTP ${res.status}`, res.status)
  const body = await res.json()
  if (body?.error) throw new Error(body.error.message || JSON.stringify(body.error))
  return body.result as T
}

function mapState(state: string, code?: string): { state: VpnUiStatus; code: string } {
  return mapWireStatus({ state, code })
}

let pollFails = 0

export async function refreshVPNStatus(): Promise<VpnBackendStatus | null> {
  try {
    const st = await vpnRpc<VpnBackendStatus>('get_status')
    pollFails = 0
    const mapped = mapState(st?.state || 'disconnected', st?.code)
    vpnStatus.set(mapped.state)
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
      lastErrorCode: mapped.code,
      phase: st?.phase || '',
      attempt: st?.attempt || 0,
      max: st?.max || 0,
    }))
    return st
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    const status = e instanceof VpnRpcError ? e.httpStatus : undefined
    // 401/403 are auth states, NOT core failures (BAG-56).
    if (status === 401 || status === 403) {
      const code = status === 401 ? 'auth_required' : 'forbidden'
      vpnStatus.set('error')
      vpnStats.update(s => ({ ...s, lastError: msg, lastErrorCode: code }))
      return null
    }
    // 4xx business errors are not core failures either.
    if (status !== undefined && status >= 400 && status < 500) {
      vpnStats.update(s => ({ ...s, lastError: msg }))
      return null
    }
    pollFails++
    const shown = pollFails >= 3 ? 'Ядро не отвечает' : msg
    if (pollFails >= 3) vpnStatus.set('core_down')
    vpnStats.update(s => ({ ...s, lastError: shown }))
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
    // fallback: still raise local SOCKS — but warn that exit-node failed
    const orig = e instanceof Error ? e.message : String(e)
    const ok = await connectRealVPN('')
    vpnStats.update(s => ({
      ...s,
      lastError: `Exit-узел недоступен — работает локальный SOCKS (трафик НЕ через донора): ${orig}`,
      lastErrorCode: 'donor_offline',
    }))
    return ok
  }
}

/**
 * Принять инвайт донора: schnorr-подпись sha256(token+"|"+npubHex) →
 * core RPC connect_invite → first-frame auth на onion/WT ноге.
 */
export async function connectInviteVPN(inv: {
  onion?: string
  wtAddr?: string
  certHash?: string
  token: string
  exp?: number
}): Promise<boolean> {
  const seckey = getSeckey()
  if (!seckey || seckey.length < 64) {
    vpnStats.update(s => ({ ...s, lastError: 'Нет секретного ключа — войдите снова' }))
    return false
  }
  const sk = seckey.slice(0, 64)
  const hb = (h: string) => { const x = h.length % 2 ? '0' + h : h; const o = new Uint8Array(x.length / 2); for (let i = 0; i < o.length; i++) o[i] = parseInt(x.slice(i * 2, i * 2 + 2), 16); return o }
  const npub = Array.from(secp.schnorr.getPublicKey(hb(sk))).map(b => b.toString(16).padStart(2, '0')).join('')
  const msgHash = new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(inv.token + '|' + npub)))
  const sigBytes = await secp.schnorr.signAsync(msgHash, hb(sk))
  const sig = Array.from(sigBytes).map(b => b.toString(16).padStart(2, '0')).join('')

  vpnStatus.set('connecting')
  lastMode = 'invite'
  try {
    await vpnRpc('connect_invite', {
      onion: inv.onion || undefined,
      wtAddr: inv.wtAddr || undefined,
      certHash: inv.certHash || undefined,
      token: inv.token,
      npub,
      sig,
      exp: inv.exp || 0,
    })
    await refreshVPNStatus()
    startPolling()
    return true
  } catch (e) {
    vpnStatus.set('error')
    vpnStats.update(s => ({ ...s, lastError: e instanceof Error ? e.message : String(e), mode: 'invite' }))
    return false
  }
}

/** Снять удержание туннеля (locked-состояние helper'а). */
export async function unlockVPN(): Promise<boolean> {
  try {
    await vpnRpc('unlock')
    await refreshVPNStatus()
    return true
  } catch (e) {
    vpnStats.update(s => ({ ...s, lastError: e instanceof Error ? e.message : String(e), lastErrorCode: 'unlock_failed' }))
    return false
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
  // Poll keeps running after manual disconnect: locked/reconcile states
  // must stay visible (BAG-56). stopPolling only fires on module unload.
  lastMode = ''
  if (!clean) {
    vpnStatus.set('error')
    vpnStats.update(s => ({ ...s, lastError: 'Повторить отключение' }))
    return
  }
  vpnStatus.set('off')
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
  startPolling()
}
