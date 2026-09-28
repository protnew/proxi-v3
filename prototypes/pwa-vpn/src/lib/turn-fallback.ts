/**
 * VPN-103: Nostr+TURN Hybrid fallback when ICE fails
 * Detects ICE failure and switches to TURN relay
 */

export interface IceCandidateStats {
  successful: boolean
  hasRelay: boolean
  hasHost: boolean
  hasSrflx: boolean
}

export async function checkIceConnectivity(pc: RTCPeerConnection): Promise<IceCandidateStats> {
  const stats = await pc.getStats()
  let successful = false
  let hasRelay = false
  let hasHost = false
  let hasSrflx = false

  stats.forEach((report) => {
    if (report.type === 'candidate-pair' && (report as any).state === 'succeeded') {
      successful = true
    }
    if (report.type === 'local-candidate') {
      const c = report as any
      if (c.candidateType === 'relay') hasRelay = true
      if (c.candidateType === 'host') hasHost = true
      if (c.candidateType === 'srflx') hasSrflx = true
    }
  })

  return { successful, hasRelay, hasHost, hasSrflx }
}

export interface TurnConfig {
  urls: string[]
  username: string
  credential: string
}

// Free TURN servers for fallback (community-maintained)
const FALLBACK_TURN: TurnConfig[] = [
  {
    urls: ['turn:turn.openrelay.metered.ca:80', 'turn:turn.openrelay.metered.ca:443'],
    username: 'openrelayproject',
    credential: 'openrelayproject',
  },
]

export function getFallbackTurnServers(): TurnConfig[] {
  return FALLBACK_TURN
}

export async function restartIceWithTurn(pc: RTCPeerConnection): Promise<void> {
  const servers = getFallbackTurnServers()
  const config = pc.getConfiguration()
  if (!config.iceServers) config.iceServers = []
  for (const t of servers) {
    config.iceServers.push({ urls: t.urls, username: t.username, credential: t.credential })
  }
  pc.setConfiguration(config)
  // ICE restart: create new offer
  const offer = await pc.createOffer({ iceRestart: true })
  await pc.setLocalDescription(offer)
}
