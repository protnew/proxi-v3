/**
 * Nostr VPN signaling client — sends/receives VPN events via Nostr (kind:30090).
 * Architecture tables 26, 53: Nostr is the chosen signaling protocol.
 */

import { SimplePool, type Event } from 'nostr-tools'
import { API_BASE } from './api'

export type VPNEventType = 'vpn-invite' | 'vpn-request' | 'vpn-accept' | 'vpn-reject' | 'vpn-cancel'

export interface VPNEvent {
  type: VPNEventType
  from: string
  to: string
  wtPort?: number
  wtCertHash?: string
  wtAddr?: string
  timestamp: number
}

const KIND_VPN = 30090

export class NostrVPNSignaling {
  private pool: SimplePool | null = null
  private relays: string[] = []
  private subscriptions: (() => void)[] = []
  private handlers: ((event: VPNEvent) => void)[] = []
  private myNpub: string = ''

  /**
   * Initialize the Nostr VPN signaling client.
   * Connects to the local Go relay + public relays.
   */
  async init(myNpub: string): Promise<void> {
    this.myNpub = myNpub
    this.pool = new SimplePool()

    // Local Go relay + public relays
    const base = API_BASE.replace('http', 'ws').replace('https', 'wss')
    this.relays = [
      `${base}/nostr`,           // Local Go server relay
      'wss://relay.damus.io',    // Public relay (fallback)
    ]

    console.log('[Nostr-VPN] Initialized with relays:', this.relays)
  }

  /**
   * Start listening for VPN events addressed to me.
   */
  async subscribeToVPNEvents(): Promise<void> {
    if (!this.pool) return

    // Subscribe to kind:30090 events
    const sub = this.pool.subscribeMany(
      this.relays,
      { kinds: [KIND_VPN], '#p': [this.myNpub] },
      {
        onevent: (event: Event) => {
          try {
            const vpnEvent: VPNEvent = JSON.parse(event.content)
            console.log('[Nostr-VPN] Received:', vpnEvent.type, 'from', vpnEvent.from)
            this.handlers.forEach(h => h(vpnEvent))
          } catch (e) {
            console.error('[Nostr-VPN] Parse error:', e)
          }
        },
      }
    )

    this.subscriptions.push(() => sub.close())
    console.log('[Nostr-VPN] Subscribed to VPN events for', this.myNpub)
  }

  /**
   * Send a VPN event (invite/request/accept/reject) to a friend.
   */
  async sendVPNEvent(
    type: VPNEventType,
    toNpub: string,
    extra?: Partial<VPNEvent>,
  ): Promise<void> {
    if (!this.pool) throw new Error('Not initialized')

    const event: VPNEvent = {
      type,
      from: this.myNpub,
      to: toNpub,
      timestamp: Math.floor(Date.now() / 1000),
      ...extra,
    }

    // Publish via relay
    const nostrEvent = {
      kind: KIND_VPN,
      content: JSON.stringify(event),
      tags: [['p', toNpub]],
      created_at: Math.floor(Date.now() / 1000),
    }

    // Use SimplePool to publish
    const pubs = this.pool.publish(this.relays, nostrEvent as any)
    await Promise.allSettled(pubs)
    console.log('[Nostr-VPN] Sent', type, 'to', toNpub)
  }

  /**
   * "Give VPN to friend" — creates a vpn-invite.
   */
  async inviteFriend(toNpub: string, wtAddr: string, wtCertHash: string): Promise<void> {
    return this.sendVPNEvent('vpn-invite', toNpub, { wtAddr, wtCertHash })
  }

  /**
   * "Request VPN from friend" — creates a vpn-request.
   */
  async requestVPN(fromNpub: string): Promise<void> {
    return this.sendVPNEvent('vpn-request', fromNpub)
  }

  /**
   * Accept a VPN invite or request.
   */
  async acceptVPN(toNpub: string, wtAddr?: string, wtCertHash?: string): Promise<void> {
    return this.sendVPNEvent('vpn-accept', toNpub, { wtAddr, wtCertHash })
  }

  /**
   * Reject a VPN invite or request.
   */
  async rejectVPN(toNpub: string): Promise<void> {
    return this.sendVPNEvent('vpn-reject', toNpub)
  }

  /**
   * Register a handler for incoming VPN events.
   */
  onVPNEvent(handler: (event: VPNEvent) => void): () => void {
    this.handlers.push(handler)
    return () => {
      this.handlers = this.handlers.filter(h => h !== handler)
    }
  }

  /**
   * Cleanup all subscriptions.
   */
  destroy(): void {
    this.subscriptions.forEach(unsub => unsub())
    this.subscriptions = []
    if (this.pool) {
      this.pool.close(this.relays)
    }
  }
}

// Singleton instance
export const nostrVPN = new NostrVPNSignaling()
