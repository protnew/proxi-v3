/**
 * VPN Module — integrates P2P WebRTC tunnel into messenger
 * Status tracking, connect/disconnect, traffic stats
 */

import { writable, derived } from 'svelte/store'

export const vpnStatus = writable<'disconnected' | 'connecting' | 'connected' | 'error'>('disconnected')
export const vpnStats = writable<{ bytesIn: number; bytesOut: number; peers: number; uptime: number }>({
  bytesIn: 0, bytesOut: 0, peers: 0, uptime: 0
})

let tunnelConnection: RTCPeerConnection | null = null
let tunnelChannel: RTCDataChannel | null = null
let exitNodeUrl = ''
let startTime = 0
let statsInterval: ReturnType<typeof setInterval> | null = null

const VPN_RELAY = 'wss://relay.damus.io'

/**
 * Connect to VPN exit node via WebRTC tunnel
 */
export async function connectVPN(exitUrl?: string): Promise<boolean> {
  if (exitUrl) exitNodeUrl = exitUrl
  if (!exitNodeUrl) {
    // Default: use built-in proxy approach
    exitNodeUrl = 'wss://relay.damus.io' // signaling relay for exit node discovery
  }

  vpnStatus.set('connecting')

  try {
    // 1. Find exit node via Nostr signaling (kind 21000)
    // 2. Establish WebRTC DataChannel
    // 3. Route traffic through tunnel

    const pc = new RTCPeerConnection({
      iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun:stun1.l.google.com:19302' },
      ],
    })

    const dc = pc.createDataChannel('vpn-tunnel', {
      ordered: false,
      maxRetransmits: 0,
    })

    dc.binaryType = 'arraybuffer'

    // Handle incoming data from tunnel
    dc.onmessage = (e) => {
      vpnStats.update(s => ({ ...s, bytesIn: s.bytesIn + (e.data as ArrayBuffer).byteLength }))
    }

    dc.onopen = () => {
      tunnelConnection = pc
      tunnelChannel = dc
      vpnStatus.set('connected')
      startTime = Date.now()
      startStatsUpdate()
    }

    dc.onerror = () => {
      vpnStatus.set('error')
    }

    // ICE
    pc.onicecandidate = (e) => {
      if (e.candidate) {
        // Send ICE to exit node via Nostr
        sendVPNSignal('ice', { candidate: e.candidate.toJSON() })
      }
    }

    // Create offer
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)

    // Wait for ICE gathering
    await new Promise<void>(resolve => {
      if (pc.iceGatheringState === 'complete') return resolve()
      pc.onicegatheringstatechange = () => { if (pc.iceGatheringState === 'complete') resolve() }
      setTimeout(resolve, 5000)
    })

    // Send offer to exit node
    sendVPNSignal('offer', { sdp: pc.localDescription!.sdp })

    return true
  } catch (e) {
    console.error('VPN connect error:', e)
    vpnStatus.set('error')
    return false
  }
}

/**
 * Disconnect VPN
 */
export function disconnectVPN() {
  tunnelChannel?.close()
  tunnelConnection?.close()
  tunnelChannel = null
  tunnelConnection = null
  vpnStatus.set('disconnected')
  if (statsInterval) clearInterval(statsInterval)
}

/**
 * Send HTTP request through VPN tunnel
 */
export async function fetchThroughVPN(url: string, options?: RequestInit): Promise<Response> {
  if (!tunnelChannel || tunnelChannel.readyState !== 'open') {
    throw new Error('VPN not connected')
  }

  // Build HTTP request as string
  const method = options?.method || 'GET'
  const headers = options?.headers || {}
  const body = options?.body as string | undefined

  const requestData = JSON.stringify({ url, method, headers, body })
  tunnelChannel.send(requestData)

  // Wait for response (simplified — real version needs correlation IDs)
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error('VPN request timeout')), 30000)

    const handler = (e: MessageEvent) => {
      clearTimeout(timeout)
      tunnelChannel?.removeEventListener('message', handler)
      vpnStats.update(s => ({ ...s, bytesOut: s.bytesOut + (e.data as ArrayBuffer).byteLength }))

      try {
        const response = JSON.parse(new TextDecoder().decode(e.data as ArrayBuffer))
        resolve(new Response(response.body, {
          status: response.status,
          headers: new Headers(response.headers),
        }))
      } catch {
        reject(new Error('Invalid VPN response'))
      }
    }

    tunnelChannel?.addEventListener('message', handler)
  })
}

/**
 * Check if VPN is connected
 */
export function isConnected(): boolean {
  return tunnelChannel?.readyState === 'open'
}

function startStatsUpdate() {
  statsInterval = setInterval(() => {
    vpnStats.update(s => ({
      ...s,
      uptime: Math.floor((Date.now() - startTime) / 1000),
      peers: tunnelConnection ? 1 : 0,
    }))
  }, 1000)
}

function sendVPNSignal(type: string, data: any) {
  // This would publish to Nostr kind 21000
  // For now, log
  console.log('[VPN] Signal:', type, data)
}

/**
 * Format bytes for display
 */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB'
  return (bytes / 1073741824).toFixed(1) + ' GB'
}

/**
 * Format seconds to mm:ss or hh:mm:ss
 */
export function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}
