/**
 * QR code generation for sharing public keys and contact info
 */
import QRCode from 'qrcode'

export interface QRData {
  type: 'nostr'
  pubkey: string
  name?: string
  relays?: string[]
}

/**
 * Generate QR code as data URL from Nostr contact data
 */
export async function generateContactQR(data: QRData): Promise<string> {
  // Nostr URI format: nostr:npub1... or nprofile format
  let content: string
  if (data.relays && data.relays.length > 0) {
    // Use nprofile format (bech32 encoded with relays)
    content = JSON.stringify({
      type: data.type,
      pubkey: data.pubkey,
      name: data.name || '',
      relays: data.relays
    })
  } else {
    content = JSON.stringify({
      type: data.type,
      pubkey: data.pubkey,
      name: data.name || ''
    })
  }

  return QRCode.toDataURL(content, {
    width: 256,
    margin: 2,
    color: {
      dark: '#000000',
      light: '#ffffff'
    },
    errorCorrectionLevel: 'M'
  })
}

/**
 * Generate QR code as canvas element
 */
export async function generateContactQRCanvas(
  data: QRData,
  container: HTMLElement
): Promise<void> {
  const content = JSON.stringify({
    type: data.type,
    pubkey: data.pubkey,
    name: data.name || '',
    relays: data.relays || []
  })

  await QRCode.toCanvas(container as HTMLCanvasElement, content, {
    width: 256,
    margin: 2,
    color: {
      dark: '#000000',
      light: '#ffffff'
    }
  })
}

/**
 * Parse scanned QR content
 */
export function parseQRContent(text: string): QRData | null {
  try {
    const data = JSON.parse(text)
    if (data.type === 'nostr' && data.pubkey) {
      return data as QRData
    }
  } catch {
    // Not JSON — might be nostr:npub... URI
    if (text.startsWith('nostr:')) {
      return { type: 'nostr', pubkey: text.slice(6) }
    }
  }
  return null
}

export async function generateInviteQR(url: string): Promise<string> {
  if (!url.startsWith('proxi+vpn://')) throw new Error('not an invite url')
  return QRCode.toDataURL(url, { width: 256, margin: 1, errorCorrectionLevel: 'M' })
}
