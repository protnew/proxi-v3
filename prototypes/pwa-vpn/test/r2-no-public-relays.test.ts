import { describe, it, expect } from 'vitest'
import { readFileSync, existsSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'

const ROOT = join(__dirname, '..')
const PATTERN = /nos\.lol|nostr\.band|damus\.io/

function walk(dir: string, out: string[] = []): string[] {
  if (!existsSync(dir)) return out
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name === 'archive' || name === '_quarantine_p9' || name === 'dist') continue
    const p = join(dir, name)
    const st = statSync(p)
    if (st.isDirectory()) walk(p, out)
    else if (/\.(ts|tsx|js|svelte|css|html)$/.test(name)) out.push(p)
  }
  return out
}

describe('R2/P9 no public relays in active PWA src', () => {
  it('Settings.svelte Profile object still has empty-string fields (regression: neutralize script)', () => {
    const s = readFileSync(join(ROOT, 'src/components/Settings.svelte'), 'utf8')
    expect(s).toMatch(/pubkey:\s*''/)
    expect(s).toMatch(/name:\s*''/)
    expect(s).toMatch(/about:\s*''/)
    expect(s).not.toMatch(/pubkey:\s+name:/)
  })

  it('active src has zero public relay host hits', () => {
    const files = walk(join(ROOT, 'src'))
    const hits: string[] = []
    for (const f of files) {
      const t = readFileSync(f, 'utf8')
      if (PATTERN.test(t)) hits.push(f)
    }
    expect(hits, hits.join('\n')).toEqual([])
  })
})
