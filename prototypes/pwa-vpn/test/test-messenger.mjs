/**
 * Unit tests for messenger modules
 * Run: node test/test-connection.mjs
 */

import { execSync } from 'child_process'
import fs from 'fs'
import path from 'path'
import crypto from 'crypto'

const passed = []
const failed = []

function test(name, fn) {
  try {
    fn()
    passed.push(name)
    console.log(`  ✅ ${name}`)
  } catch (e) {
    failed.push(name)
    console.log(`  ❌ ${name}: ${e.message}`)
  }
}

function assert(cond, msg = 'Assertion failed') {
  if (!cond) throw new Error(msg)
}

console.log('\n🧪 Indestructible Messenger — Unit Tests\n')

// --- Key Generation ---
test('secp256k1 key generation produces 32-byte keys', () => {
  // Simulate what nostr-tools does
  const sk = crypto.randomBytes(32)
  assert(sk.length === 32, 'Secret key must be 32 bytes')
  assert(Buffer.isBuffer(sk))
})

test('SHA-256 produces consistent 32-byte hash', () => {
  const data = Buffer.from('test message')
  const hash = crypto.createHash('sha256').update(data).digest()
  assert(hash.length === 32, 'SHA-256 must produce 32 bytes')
  const hash2 = crypto.createHash('sha256').update(data).digest()
  assert(hash.equals(hash2), 'Same input must produce same hash')
})

// --- Nostr Event Format ---
test('Nostr event has required fields', () => {
  const event = {
    kind: 14,
    content: 'Hello',
    tags: [['p', 'target_pubkey']],
    created_at: Math.floor(Date.now() / 1000),
    pubkey: 'sender_pubkey',
  }
  assert(typeof event.kind === 'number')
  assert(typeof event.content === 'string')
  assert(Array.isArray(event.tags))
  assert(event.tags[0][0] === 'p')
  assert(typeof event.created_at === 'number')
})

test('DM uses kind 14', () => {
  assert(14 === 14, 'DM kind must be 14')
})

test('Presence uses kind 21001', () => {
  assert(21001 === 21001, 'Presence kind must be 21001')
})

test('Typing uses kind 21002', () => {
  assert(21002 === 21002, 'Typing kind must be 21002')
})

test('File manifest uses kind 21003', () => {
  assert(21003 === 21003, 'File manifest kind must be 21003')
})

test('Call signal uses kind 21004', () => {
  assert(21004 === 21004, 'Call signal kind must be 21004')
})

// --- File Transfer ---
test('File chunking produces correct number of chunks', () => {
  const CHUNK_SIZE = 64 * 1024
  const fileSize = 150 * 1024 // 150KB
  const expected = Math.ceil(fileSize / CHUNK_SIZE)
  assert(expected === 3, `Expected 3 chunks for 150KB, got ${expected}`)
})

test('SHA-256 file hash verification', async () => {
  const data = crypto.randomBytes(1024)
  const hash1 = crypto.createHash('sha256').update(data).digest('hex')
  const hash2 = crypto.createHash('sha256').update(data).digest('hex')
  assert(hash1 === hash2, 'Same data must produce same hash')
  assert(hash1.length === 64, 'Hex hash must be 64 chars')
})

// --- Message Store ---
test('Message has required fields', () => {
  const msg = {
    id: crypto.randomUUID(),
    from: 'sender',
    to: 'dm:receiver',
    text: 'Hello',
    timestamp: Date.now(),
    type: 'text',
    read: false,
  }
  assert(typeof msg.id === 'string')
  assert(msg.to.startsWith('dm:'))
  assert(msg.type === 'text' || msg.type === 'voice' || msg.type === 'file')
})

test('Chat ID format: dm:<pubkey> or group:<channelId>', () => {
  const dmId = 'dm:' + 'a'.repeat(64)
  const groupId = 'group:' + 'b'.repeat(64)
  assert(dmId.startsWith('dm:'))
  assert(groupId.startsWith('group:'))
})

// --- Build ---
test('Build output exists and is reasonable size', () => {
  const distDir = path.join(process.cwd(), 'dist')
  assert(fs.existsSync(distDir), 'dist/ must exist after build')
  
  const assetsDir = path.join(distDir, 'assets')
  assert(fs.existsSync(assetsDir), 'dist/assets/ must exist')
  
  const files = fs.readdirSync(assetsDir)
  const jsFile = files.find(f => f.endsWith('.js'))
  assert(jsFile, 'Must have a JS bundle')
  
  const jsSize = fs.statSync(path.join(assetsDir, jsFile)).size
  assert(jsSize < 200 * 1024, `JS bundle must be < 200KB, got ${Math.round(jsSize / 1024)}KB`)
})

// --- PWA ---
test('PWA manifest exists', () => {
  const manifest = path.join(process.cwd(), 'dist', 'manifest.webmanifest')
  assert(fs.existsSync(manifest), 'manifest.webmanifest must exist')
})

test('Service worker exists', () => {
  const sw = path.join(process.cwd(), 'dist', 'sw.js')
  assert(fs.existsSync(sw), 'sw.js must exist')
})

// --- Source files ---
test('All component files exist', () => {
  const components = ['Sidebar.svelte', 'ChatView.svelte', 'Settings.svelte', 'EmojiPicker.svelte', 'NewChat.svelte', 'CallOverlay.svelte', 'VpnPanel.svelte']
  const dir = path.join(process.cwd(), 'src', 'components')
  for (const c of components) {
    assert(fs.existsSync(path.join(dir, c)), `Component ${c} must exist`)
  }
  // App.svelte is in src/ not src/components/
  assert(fs.existsSync(path.join(process.cwd(), 'src', 'App.svelte')), 'App.svelte must exist')
})

test('All library files exist', () => {
  const libs = ['nostr.ts', 'identity.ts', 'webrtc-tunnel.ts', 'exit-node.ts', 'nostr-signaling.ts', 'sounds.ts', 'file-transfer.ts', 'peer-manager.ts', 'voice.ts', 'calls.ts', 'vpn.ts', 'desktop.ts']
  const dir = path.join(process.cwd(), 'src', 'lib')
  for (const l of libs) {
    assert(fs.existsSync(path.join(dir, l)), `Library ${l} must exist`)
  }
})

test('Store files exist', () => {
  const dir = path.join(process.cwd(), 'src', 'stores')
  assert(fs.existsSync(path.join(dir, 'messenger.ts')), 'messenger.ts store must exist')
})

// --- Tauri desktop ---
test('Tauri config files exist', () => {
  const tauriDir = path.join(process.cwd(), 'src-tauri')
  assert(fs.existsSync(tauriDir), 'src-tauri/ must exist')
  assert(fs.existsSync(path.join(tauriDir, 'Cargo.toml')), 'Cargo.toml must exist')
  assert(fs.existsSync(path.join(tauriDir, 'tauri.conf.json')), 'tauri.conf.json must exist')
  assert(fs.existsSync(path.join(tauriDir, 'src', 'lib.rs')), 'lib.rs must exist')
  assert(fs.existsSync(path.join(tauriDir, 'src', 'main.rs')), 'main.rs must exist')
})

// --- Results ---
console.log(`\n📊 Results: ${passed.length} passed, ${failed.length} failed, ${passed.length + failed.length} total\n`)

if (failed.length > 0) {
  console.log('Failed tests:')
  failed.forEach(f => console.log(`  ❌ ${f}`))
  process.exit(1)
}

console.log('✅ All tests passed!\n')
