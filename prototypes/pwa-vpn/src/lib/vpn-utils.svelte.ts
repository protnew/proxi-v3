/**
 * Pure VPN utility functions extracted from VPNProductPanel.svelte.
 */

import { getPubkey, getSeckey } from "../lib/api";
import * as secp from "@noble/secp256k1";

  function hexToBytes(hex: string): Uint8Array {
    const h = hex.length % 2 ? '0' + hex : hex
    const out = new Uint8Array(h.length / 2)
    for (let i = 0; i < out.length; i++) out[i] = parseInt(h.slice(i * 2, i * 2 + 2), 16)
    return out
  }
  function bytesToHex(b: Uint8Array): string {
    return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')
  }
  function mySigningPubkey(): string {
    const sk = (getSeckey() || '').slice(0, 64)
    if (sk.length === 64) return bytesToHex(secp.schnorr.getPublicKey(hexToBytes(sk)))
    return getPubkey()
  }
  /** Accept hex pubkey or demo 1111.../2222... seckey-as-id */
  function normalizePeerId(raw: string): string {
    const s = raw.trim().toLowerCase().replace(/^0x/, '')
    if (/^[0-9a-f]{64}$/.test(s)) {
      // If it's a demo seckey (111.. or 222..), derive pubkey
      try {
        return bytesToHex(secp.schnorr.getPublicKey(hexToBytes(s)))
      } catch {
        return s
      }
    }
    return s
  }



  async function authHeaders(): Promise<Record<string, string>> {
    const token = localStorage.getItem('proxi_token') || ''
    return { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token }
  }

  async function startWTServer(): Promise<{ wtAddr: string; certHash: string }> {
    const h = await authHeaders()
    const start = await fetch('/api/vpn/wt/start', { method: 'POST', headers: h, body: '{}' })
    if (!start.ok) throw new Error('WT start HTTP ' + start.status)
    const stats = await start.json()
    const certHash = stats.certHash || ''
    // Prefer LAN host + actual WT port from addr
        // Prefer page hostname (LAN/localhost). Never publish 0.0.0.0 or bare [::].
    // Use window.location.hostname so friends on same LAN can connect.
    let host = window.location.hostname || '127.0.0.1'
    if (host === '[::]' || host === '::' || host === '0.0.0.0') host = '127.0.0.1'
    let port = '4433'
    if (typeof stats.addr === 'string' && stats.addr) {
      const m = String(stats.addr).match(/:(\d+)$/)
      if (m) port = m[1]
    }
    const wtAddr = `${host}:${port}`
    return { wtAddr, certHash }
  }
export { hexToBytes, bytesToHex, mySigningPubkey, normalizePeerId, authHeaders, startWTServer };
