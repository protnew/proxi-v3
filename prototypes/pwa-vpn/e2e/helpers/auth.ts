/**
 * Shared e2e auth helper — P1 challenge-response flow.
 * Bare-npub signup is gone: server requires a fresh kind:22242 event
 * signed by the account's secp256k1 key.
 */
import { generateSecretKey, getPublicKey, finalizeEvent } from 'nostr-tools/pure'
import type { APIRequestContext } from '@playwright/test'

const API = process.env.API_URL || 'http://127.0.0.1:8090'

export async function apiToken(request: APIRequestContext, username = 'e2e'): Promise<string> {
  const sk = generateSecretKey()
  const npub = getPublicKey(sk) // 64-hex x-only pubkey
  const ch = await request.post(`${API}/api/auth/challenge`, { data: { npub } })
  if (!ch.ok()) throw new Error(`challenge failed: ${ch.status()}`)
  const { challenge } = await ch.json()
  const event = finalizeEvent(
    {
      kind: 22242,
      created_at: Math.floor(Date.now() / 1000),
      tags: [['challenge', challenge]],
      content: challenge,
    },
    sk
  )
  const r = await request.post(`${API}/api/auth/signup`, {
    data: { npub, username: `${username}_${Date.now().toString().slice(-6)}`, challenge, event },
  })
  const j = await r.json().catch(() => ({}))
  if (!r.ok()) throw new Error(`signup failed: ${r.status()} ${JSON.stringify(j)}`)
  return j.access_token as string
}
