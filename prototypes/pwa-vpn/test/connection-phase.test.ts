import { describe, expect, it } from 'vitest'
import { phaseFromDesktop, phaseFromVpnUi } from '../src/lib/connection-status'

describe('one connection phase', () => {
  it('maps vpn and desktop strings onto the same enum', () => {
    expect(phaseFromVpnUi('sharing')).toBe('connected')
    expect(phaseFromVpnUi('core_down')).toBe('failed')
    expect(phaseFromVpnUi('locked')).toBe('peer_offline')
    expect(phaseFromDesktop('unavailable')).toBe('idle')
    expect(phaseFromDesktop('connecting')).toBe(phaseFromVpnUi('connecting'))
  })
})
