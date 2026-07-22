/**
 * Exit node — acts as internet gateway for peers
 * Receives HTTP requests via Data Channel → fetches → returns response
 * This runs on the "friend in a free country" side
 */
import { WebRTCTunnel } from './webrtc-tunnel'

export class ExitNode {
  private tunnels: Map<string, WebRTCTunnel> = new Map()

  /**
   * Register a tunnel to handle requests from a peer
   */
  addTunnel(peerPubkey: string, tunnel: WebRTCTunnel) {
    this.tunnels.set(peerPubkey, tunnel)

    tunnel.on('message', (data: string) => {
      this.handleMessage(peerPubkey, data)
    })
  }

  private async handleMessage(peerPubkey: string, data: string) {
    const tunnel = this.tunnels.get(peerPubkey)
    if (!tunnel) return

    try {
      const msg = JSON.parse(data)

      if (msg.type === 'http-request') {
        console.log(`[ExitNode] Fetching: ${msg.url}`)
        
        try {
          const response = await fetch(msg.url, {
            method: msg.method || 'GET',
            headers: {
              'User-Agent': 'Mozilla/5.0 (compatible; IndestructibleVPN/1.0)',
            },
          })
          const body = await response.text()
          
          tunnel.send(JSON.stringify({
            type: 'http-response',
            id: msg.id,
            status: response.status,
            body,
          }))
        } catch (fetchError) {
          tunnel.send(JSON.stringify({
            type: 'http-response',
            id: msg.id,
            status: 502,
            body: `Exit node error: ${fetchError}`,
          }))
        }
      }
    } catch (e) {
      console.error('[ExitNode] Error handling message:', e)
    }
  }

  removeTunnel(peerPubkey: string) {
    const tunnel = this.tunnels.get(peerPubkey)
    tunnel?.close()
    this.tunnels.delete(peerPubkey)
  }
}
