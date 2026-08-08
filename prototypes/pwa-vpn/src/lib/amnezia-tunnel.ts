/**
 * DPI-001 AmneziaWG tunnel client (desktop path via Go API).
 * PWA calls these endpoints; kernel apply happens on host if driver installed.
 */

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

export async function getTunnelStatus(): Promise<TunnelStatus> {
  const r = await fetch('/api/vpn/amnezia/tunnel')
  if (!r.ok) throw new Error('tunnel status ' + r.status)
  return r.json()
}

export async function startTunnel(opts: {
  peerPublicKey: string
  endpoint: string
  allowedIPs?: string
  address?: string
}): Promise<TunnelStatus> {
  const r = await fetch('/api/vpn/amnezia/tunnel', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
  const j = await r.json()
  if (!r.ok) throw new Error(j?.error || j?.message || 'tunnel start failed')
  return j
}

export async function stopTunnel(): Promise<TunnelStatus> {
  const r = await fetch('/api/vpn/amnezia/tunnel', { method: 'DELETE' })
  return r.json()
}

export async function buildConf(opts: {
  privateKey: string
  peerPublicKey: string
  endpoint: string
  allowedIPs?: string
}): Promise<{ conf: string; phase: string }> {
  const r = await fetch('/api/vpn/amnezia/conf', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
  if (!r.ok) throw new Error('conf ' + r.status)
  return r.json()
}
