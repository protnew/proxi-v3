/**
 * VPN-107: Invite link — QR code + deep link for endpoint/keys
 */

export interface InviteData {
  type: 'vpn-invite'
  role: 'host' | 'joiner'
  pubkey: string
  endpoints: string[]
  createdAt: number
}

export function encodeInvite(data: InviteData): string {
  const json = JSON.stringify(data)
  // Base64url encode
  const b64 = typeof btoa !== 'undefined'
    ? btoa(unescape(encodeURIComponent(json)))
    : Buffer.from(json, 'utf-8').toString('base64')
  return `proxi://invite/${b64.replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_')}`
}

export function decodeInvite(link: string): InviteData | null {
  try {
    const match = link.match(/proxi:\/\/invite\/(.+)$/)
    if (!match) return null
    const b64 = match[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = b64 + '='.repeat((4 - b64.length % 4) % 4)
    const json = typeof atob !== 'undefined'
      ? decodeURIComponent(escape(atob(padded)))
      : Buffer.from(padded, 'base64').toString('utf-8')
    return JSON.parse(json) as InviteData
  } catch {
    return null
  }
}

export function createInvite(pubkey: string, endpoints: string[], role: 'host' | 'joiner' = 'host'): InviteData {
  return {
    type: 'vpn-invite',
    role,
    pubkey,
    endpoints,
    createdAt: Date.now(),
  }
}
