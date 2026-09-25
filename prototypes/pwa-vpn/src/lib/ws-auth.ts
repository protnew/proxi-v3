/** WS auth rides in Sec-WebSocket-Protocol. Query ?token= is file routes only. */
export function wsAuthProtocols(token: string | null | undefined): string[] {
  if (!token) return ['proxi']
  return ['proxi', 'proxi-jwt.' + token]
}
