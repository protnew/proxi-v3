import { describe, expect, it } from 'vitest'
import { wsAuthProtocols } from '../src/lib/ws-auth'

describe('P-E ws subprotocol', () => {
  it('puts jwt in protocol not in a query string', () => {
    const token = 'eyJhbGciOiJIUzI1NiJ9.payload.sig'
    const proto = wsAuthProtocols(token)
    expect(proto[0]).toBe('proxi')
    expect(proto[1]).toBe('proxi-jwt.' + token)
    expect(proto.join(' ')).not.toContain('?token=')
  })

  it('anonymous still offers proxi only', () => {
    expect(wsAuthProtocols(null)).toEqual(['proxi'])
    expect(wsAuthProtocols('')).toEqual(['proxi'])
  })
})
