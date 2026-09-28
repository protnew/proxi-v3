/**
 * DPI-001 AmneziaWG tunnel client (desktop path via Go API).
 * Optional Advanced path. Primary in-app VPN is WebRTC DataChannel.
 */
import { authHeaders } from './vpn-utils.svelte'

export type TunnelStatus = {
  state: string
  mode: string
  driver?: string
  confPath?: string
  localPublicKey?: string
  peerPublicKey?: string
  endpoint?: string
  address?: string
  lastError?: string
  note?: string
  jc?: number
}

async function readJson(r: Response): Promise<any> {
  const text = await r.text()
  if (!text) throw new Error('empty response HTTP ' + r.status)
  try { return JSON.parse(text) } catch {
    throw new Error('bad json HTTP ' + r.status)
  }
}

function errMsg(j: any, fallback: string): string {
  if (typeof j?.error === 'string') return j.error
  if (typeof j?.error?.message === 'string') return j.error.message
  if (typeof j?.message === 'string') return j.message
  return fallback
}

export async function getTunnelStatus(): Promise<TunnelStatus> {
  const r = await fetch('/api/vpn/amnezia/tunnel', { headers: await authHeaders() })
  if (!r.ok) throw new Error('tunnel status ' + r.status)
  return readJson(r)
}

export async function startTunnel(opts: {
  peerPublicKey: string
  endpoint: string
  allowedIPs?: string
  address?: string
}): Promise<TunnelStatus> {
  const r = await fetch('/api/vpn/amnezia/tunnel', {
    method: 'POST',
    headers: await authHeaders(),
    body: JSON.stringify(opts),
  })
  const j = await readJson(r)
  if (!r.ok) throw new Error(errMsg(j, 'tunnel start failed'))
  return j
}

export async function stopTunnel(): Promise<TunnelStatus> {
  const r = await fetch('/api/vpn/amnezia/tunnel', {
    method: 'DELETE',
    headers: await authHeaders(),
  })
  return readJson(r)
}

export async function buildConf(opts: {
  privateKey: string
  peerPublicKey: string
  endpoint: string
  allowedIPs?: string
}): Promise<{ conf: string; phase: string }> {
  const r = await fetch('/api/vpn/amnezia/conf', {
    method: 'POST',
    headers: await authHeaders(),
    body: JSON.stringify(opts),
  })
  if (!r.ok) throw new Error('conf ' + r.status)
  return readJson(r)
}
