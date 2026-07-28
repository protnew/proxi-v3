/** Small pure helpers for chat payload / E2E flag (keep api.ts <500 LOC). */
export function makeChatPayload(
  from: string,
  to: string,
  content: string,
  encrypted: boolean,
  ts = Math.floor(Date.now() / 1000),
) {
  return { type: 'chat' as const, from, to, text: content, ts, encrypted: !!encrypted }
}
