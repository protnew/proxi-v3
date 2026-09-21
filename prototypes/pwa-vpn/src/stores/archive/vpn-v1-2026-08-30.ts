/**
 * PWA-004: Nano Store — VPN status
 */
import { atom, computed } from 'nanostores'

export type VPNStatus = 'disconnected' | 'connecting' | 'connected'

export interface VPNState {
  status: VPNStatus
  role: 'none' | 'host' | 'joiner'
  tunnelIp: string
  bytesIn: number
  bytesOut: number
  peers: number
}

export const vpnStore = atom<VPNState>({
  status: 'disconnected',
  role: 'none',
  tunnelIp: '',
  bytesIn: 0,
  bytesOut: 0,
  peers: 0,
})

export function setVPNStatus(s: Partial<VPNState>) {
  vpnStore.set({ ...vpnStore.get(), ...s })
}

export const vpnConnected = computed(vpnStore, s => s.status === 'connected')
export const vpnStats = computed(vpnStore, s => ({
  in: s.bytesIn,
  out: s.bytesOut,
  peers: s.peers,
}))
