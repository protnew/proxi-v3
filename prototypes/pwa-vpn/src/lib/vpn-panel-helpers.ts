/**
 * VPN helper functions extracted from VPNProductPanel.svelte
 * to stay under 500 LOC limit.
 */

import { rtcVPN } from '../lib/webrtc-vpn';
import { getLocalTabP2P, type TabP2PStatus } from '../lib/local-tab-p2p';
import { getPubkey, getSeckey } from '../lib/api';

// INF-010: Nostr signaling wired — connects WebRTC VPN through Nostr relays
async function startVPNSignaling(targetPubkey: string) {
  try {
    await (rtcVPN as any).startNostrSignaling(targetPubkey)
    console.log('[VPN] Nostr signaling started for', targetPubkey.slice(0, 8))
  } catch (e) {
    console.warn('[VPN] Nostr signaling failed:', e)
  }
}


  let showInviteModal = $state(false)
  let showRequestModal = $state(false)
  let incomingEvent = $state<VPNEvent | null>(null)
  let friendId = $state('')
  let vpnStatus = $state<'off' | 'sharing' | 'connecting' | 'connected'>('off')
  let statusText = $state('VPN выключен')
  let log = $state<string[]>([])
  // VPN-101: same-PC two-tab WebRTC proof
  let tabP2P = $state<TabP2PStatus>({
    phase: 'idle', role: 'none', lastError: '', peerReady: false, rttMs: null, bytesIn: 0, bytesOut: 0, tunnelIp: ''
  })
  const localP2P = getLocalTabP2P()
  localP2P.onChange((s) => {
    tabP2P = s
    if (s.phase === 'connected') {
      vpnStatus = 'connected'
      statusText = s.role === 'host'
        ? 'VPN: 2 вкладки · вы exit node (DataChannel open)'
        : `VPN: 2 вкладки · DataChannel open${s.tunnelIp ? ' · IP ' + s.tunnelIp : ''}`
      addLog('VPN-101 P2P connected role=' + s.role + (s.tunnelIp ? ' ip=' + s.tunnelIp : ''))
    } else if (s.phase === 'negotiating' || s.phase === 'waiting_peer') {
      vpnStatus = 'connecting'
      statusText = s.phase === 'waiting_peer' ? 'VPN-101: жду вторую вкладку…' : 'VPN-101: WebRTC negotiating…'
    } else if (s.phase === 'error') {
      vpnStatus = 'off'
      statusText = 'VPN-101 error: ' + s.lastError
      addLog('VPN-101 error: ' + s.lastError)
    }
  })
  async function startTabP2PHost() {
    try {
      await localP2P.startAsHost()
      addLog('VPN-101: host (exit) — откройте 2-ю вкладку и нажмите «Войти peer»')
    } catch (e) { addLog('VPN-101 host fail: ' + e) }
  }
  async function startTabP2PJoiner() {
    try {
      await localP2P.startAsJoiner()
      addLog('VPN-101: joiner — ищу host-вкладку…')
    } catch (e) { addLog('VPN-101 joiner fail: ' + e) }
  }
  function stopTabP2P() {
    localP2P.stop()
    vpnStatus = 'off'
    statusText = 'VPN выключен'
    addLog('VPN-101 stopped')
  }
  let lastTunnelIp = $state('')
  let handlingIncoming = $state(false)
  let turnStatus = $state<string>('checking…')
  let amneziaStatus = $state<string>('')
  let tunnelInfo = $state<TunnelStatus | null>(null)
  let pushInfo = $state<string>('')
  let pushBusy = $state(false)
  let tunnelBusy = $state(false)
  let transportMode = $state<string>('')  // 'P2P' | 'Nostr Relay' | ''
  let dismissedFrom = $state<Record<string, number>>({})

export {
  startVPNSignaling,
  startTabP2PHost,
  startTabP2PJoiner,
  stopTabP2P,
};
