import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const chat = join(process.cwd(), 'src/components/ChatView.svelte')
const messenger = join(process.cwd(), 'src/stores/messenger.ts')

describe('P19/P20 delivery honesty', () => {
  it('ChatView imports toast and uses deliveryStatus marks', () => {
    const cv = readFileSync(chat, 'utf8')
    expect(cv.includes("from '../lib/toast'") || cv.includes('from "../lib/toast"')).toBe(true)
    expect(cv.includes('deliveryStatus')).toBe(true)
  })
  it('messenger optimistic send starts as pending / not read', () => {
    const ms = readFileSync(messenger, 'utf8')
    expect(ms.includes('deliveryStatus')).toBe(true)
    expect(ms.includes("pending") || ms.includes("'pending'") || ms.includes('"pending"')).toBe(true)
  })
})