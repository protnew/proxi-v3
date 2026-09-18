/** Small pure helpers for chat payload / E2E flag (keep api.ts <500 LOC). */
import { isEncryptedPayload } from './nip-e2e'

/**
 * Build a WS chat frame.
 * P5: `encrypted` is true ONLY when `content` is real ciphertext (nip44:/v1.).
 * Passing encrypted=true with plaintext is ignored — never lie on the wire.
 */
export function makeChatPayload(
  from: string,
  to: string,
  content: string,
  encrypted: boolean,
  ts = Math.floor(Date.now() / 1000),
) {
  const actuallyEncrypted = isEncryptedPayload(content)
  return {
    type: 'chat' as const,
    from,
    to,
    text: content,
    ts,
    encrypted: !!encrypted && actuallyEncrypted,
  }
}
