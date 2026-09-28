
import WebSocket from 'ws'
import * as secp from '@noble/secp256k1'
import { webcrypto } from 'crypto'
if (!globalThis.crypto) globalThis.crypto = webcrypto

function bytesToHex(b) {
  return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')
}
function hexToBytes(hex) {
  const out = new Uint8Array(hex.length / 2)
  for (let i = 0; i < out.length; i++) out[i] = parseInt(hex.slice(i*2, i*2+2), 16)
  return out
}
async function sha256Hex(data) {
  const buf = await webcrypto.subtle.digest('SHA-256', new TextEncoder().encode(data))
  return bytesToHex(new Uint8Array(buf))
}

// demo seckeys
const aliceSk = '1'.repeat(64)
const bobSk = '2'.repeat(64)
const alicePk = bytesToHex(secp.schnorr.getPublicKey(hexToBytes(aliceSk)))
const bobPk = bytesToHex(secp.schnorr.getPublicKey(hexToBytes(bobSk)))
console.log('alicePk', alicePk)
console.log('bobPk', bobPk)

async function signAndPublish() {
  const content = JSON.stringify({
    type: 'vpn-invite',
    from: alicePk,
    to: bobPk,
    wtAddr: '127.0.0.1:4433',
    wtCertHash: 'ab'.repeat(32),
    timestamp: Math.floor(Date.now()/1000),
  })
  const created_at = Math.floor(Date.now()/1000)
  const kind = 30090
  const tags = [['p', bobPk], ['t', 'vpn-invite']]
  const serialized = JSON.stringify([0, alicePk, created_at, kind, tags, content])
  const id = await sha256Hex(serialized)
  // sign same as identity.ts
  const msgHash = id // already sha256 hex of serialized
  // Wait - identity signs sha256 of serialized again? Let's match identity.ts:
  // msgHash = sha256(serialized), sig = schnorr.sign(msgHash, sk)
  // And id = same sha256(serialized). Good.
  const sigBytes = await secp.schnorr.signAsync(hexToBytes(msgHash), hexToBytes(aliceSk))
  const sig = bytesToHex(sigBytes)
  const event = { id, pubkey: alicePk, created_at, kind, tags, content, sig }

  return new Promise((resolve, reject) => {
    const ws = new WebSocket('ws://127.0.0.1:8090/nostr')
    const timer = setTimeout(() => reject(new Error('timeout')), 8000)
    ws.on('open', () => {
      // Bob subscribes
      ws.send(JSON.stringify(['REQ', 'sub1', { kinds: [30090], '#p': [bobPk] }]))
      // Alice publishes
      ws.send(JSON.stringify(['EVENT', event]))
      console.log('published', id.slice(0,16))
    })
    const msgs = []
    ws.on('message', (data) => {
      const msg = JSON.parse(data.toString())
      msgs.push(msg)
      console.log('RECV', JSON.stringify(msg).slice(0, 200))
      if (msg[0] === 'OK' && msg[1] === id) {
        clearTimeout(timer)
        ws.close()
        resolve({ ok: msg[2], reason: msg[3] || '', msgs })
      }
    })
    ws.on('error', reject)
  })
}

const result = await signAndPublish()
console.log('RESULT', JSON.stringify(result))
if (!result.ok) {
  process.exit(2)
}
