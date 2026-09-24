/** Bech32 npub/nsec helpers. Split from identity.ts (TZ-EXEC-20260924 A1). */

const BECH32_CHARSET = 'qpzry9x8gf2tvdw0s3jn54khce6mua7l'

function bech32Polymod(values: number[]): number {
  const GEN = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3]
  let chk = 1
  for (const v of values) {
    const b = chk >> 25
    chk = ((chk & 0x1ffffff) << 5) ^ v
    for (let i = 0; i < 5; i++) {
      if ((b >> i) & 1) chk ^= GEN[i]
    }
  }
  return chk
}

function bech32HrpExpand(hrp: string): number[] {
  const ret: number[] = []
  for (let i = 0; i < hrp.length; i++) ret.push(hrp.charCodeAt(i) >> 5)
  ret.push(0)
  for (let i = 0; i < hrp.length; i++) ret.push(hrp.charCodeAt(i) & 31)
  return ret
}

function bech32CreateChecksum(hrp: string, data: number[]): number[] {
  const values = bech32HrpExpand(hrp).concat(data).concat([0, 0, 0, 0, 0, 0])
  const polymod = bech32Polymod(values) ^ 1
  const ret: number[] = []
  for (let i = 0; i < 6; i++) {
    ret.push((polymod >> (5 * (5 - i))) & 31)
  }
  return ret
}

function convertBits(data: Uint8Array, fromBits: number, toBits: number): number[] {
  let acc = 0
  let bits = 0
  const ret: number[] = []
  const maxv = (1 << toBits) - 1
  for (let i = 0; i < data.length; i++) {
    acc = (acc << fromBits) | data[i]
    bits += fromBits
    while (bits >= toBits) {
      bits -= toBits
      ret.push((acc >> bits) & maxv)
    }
  }
  if (bits > 0) {
    ret.push((acc << (toBits - bits)) & maxv)
  }
  return ret
}

export function encodeBech32(hrp: string, data: Uint8Array): string {
  const words = convertBits(data, 8, 5)
  const checksum = bech32CreateChecksum(hrp, words)
  let result = hrp + '1'
  for (const w of words) result += BECH32_CHARSET[w]
  for (const c of checksum) result += BECH32_CHARSET[c]
  return result
}

/** Decode bech32 npub/nsec → bytes */
export function decodeBech32(bech: string): { hrp: string; data: Uint8Array } {
  const lower = bech.trim().toLowerCase()
  const pos = lower.lastIndexOf('1')
  if (pos < 1) throw new Error('invalid bech32')
  const hrp = lower.slice(0, pos)
  const dataPart = lower.slice(pos + 1)
  const words: number[] = []
  for (const ch of dataPart) {
    const v = BECH32_CHARSET.indexOf(ch)
    if (v < 0) throw new Error('invalid bech32 char')
    words.push(v)
  }
  if (words.length < 7) throw new Error('bech32 too short')
  const dataWords = words.slice(0, -6)
  let acc = 0, bits = 0
  const bytes: number[] = []
  const maxv = 255
  for (const w of dataWords) {
    acc = (acc << 5) | w
    bits += 5
    while (bits >= 8) {
      bits -= 8
      bytes.push((acc >> bits) & maxv)
    }
  }
  return { hrp, data: new Uint8Array(bytes) }
}
