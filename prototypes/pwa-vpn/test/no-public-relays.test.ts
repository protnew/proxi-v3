import { describe, it, expect } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const srcRoot = path.resolve(__dirname, '../src')
const pattern = /nos\.lol|nostr\.band|damus\.io/i

function walk(dir: string, acc: string[] = []): string[] {
  if (!fs.existsSync(dir)) return acc
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    if (ent.name === 'node_modules' || ent.name === 'archive' || ent.name === 'dist') continue
    const p = path.join(dir, ent.name)
    if (ent.isDirectory()) walk(p, acc)
    else if (/\.(ts|js|svelte|json|mjs|cjs)$/.test(ent.name)) acc.push(p)
  }
  return acc
}

describe('R2 guard mirror: no public relays in active src', () => {
  it('fails if nos.lol / nostr.band / damus.io appear under prototypes/pwa-vpn/src', () => {
    const hits: string[] = []
    for (const f of walk(srcRoot)) {
      const text = fs.readFileSync(f, 'utf8')
      if (pattern.test(text)) hits.push(path.relative(srcRoot, f))
    }
    expect(hits, public relays in: ).toEqual([])
  })
})