import { describe, expect, it } from 'vitest'
import { readFileSync, existsSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'

const PUBLIC = [
  'relay.damus.io',
  'nos.lol',
  'nostr.band',
  'relay.nostr.band',
]

function walk(dir: string, acc: string[] = []): string[] {
  if (!existsSync(dir)) return acc
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name === 'dist' || name === '_quarantine_p9') continue
    const p = join(dir, name)
    const st = statSync(p)
    if (st.isDirectory()) walk(p, acc)
    else if (/\.(ts|js|svelte|json|md)$/i.test(name)) acc.push(p)
  }
  return acc
}

describe('no public relays in pwa sources (R2)', () => {
  it('src/ and serveable sources avoid public relay hosts', () => {
    const roots = [join(process.cwd(), 'src')]
    const hits: string[] = []
    for (const root of roots) {
      for (const file of walk(root)) {
        const text = readFileSync(file, 'utf8')
        for (const host of PUBLIC) {
          if (text.includes(host)) hits.push(`${file} :: ${host}`)
        }
      }
    }
    expect(hits, `public relays found:\n${hits.join('\n')}`).toEqual([])
  })
})