import { describe, expect, it } from 'vitest'
import { mapWireStatus } from '../src/lib/vpn-wire'

describe('mapWireStatus', () => {
  it('keeps known states', () => {
    expect(mapWireStatus({ state: 'connecting', phase: 'routes' }).state).toBe('connecting')
    expect(mapWireStatus({ state: 'locked' }).state).toBe('locked')
    expect(mapWireStatus({ state: 'sharing' }).state).toBe('sharing')
  })
  it('maps unknown to error(unknown_state)', () => {
    const out = mapWireStatus({ state: 'mystery' })
    expect(out.state).toBe('error')
    expect(out.code).toBe('unknown_state')
  })
  it('rejects an unknown error code', () => {
    expect(mapWireStatus({ state: 'error', code: 'made_up' }).code).toBe('unknown_state')
  })
  it('does not paint locked as off', () => {
    expect(mapWireStatus({ state: 'locked' }).state).not.toBe('off')
    expect(mapWireStatus({ state: '' }).state).toBe('error')
  })
  it('keeps a listed error code', () => {
    expect(mapWireStatus({ state: 'error', code: 'no_exit_peers' }).code).toBe('no_exit_peers')
  })
})
