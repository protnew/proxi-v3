import { describe, it, expect, beforeEach } from 'vitest'
import { vpnStore, setVPNStatus, vpnConnected, vpnStats } from '../src/stores/vpn'

describe('stores/vpn', () => {
  beforeEach(() => {
    vpnStore.set({ status: 'disconnected', role: 'none', tunnelIp: '', bytesIn: 0, bytesOut: 0, peers: 0 })
  })
  it('default disconnected', () => {
    expect(vpnConnected.get()).toBe(false)
    expect(vpnStats.get()).toEqual({ in: 0, out: 0, peers: 0 })
  })
  it('setVPNStatus connects', () => {
    setVPNStatus({ status: 'connected', role: 'host', tunnelIp: '10.0.0.1', bytesIn: 10, bytesOut: 20, peers: 1 })
    expect(vpnConnected.get()).toBe(true)
    expect(vpnStore.get().tunnelIp).toBe('10.0.0.1')
    expect(vpnStats.get()).toEqual({ in: 10, out: 20, peers: 1 })
  })
})
