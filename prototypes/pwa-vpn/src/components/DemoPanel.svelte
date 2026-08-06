<script lang="ts">
  import { setIdentity, getPubkey, sendDM, connectRelays } from '../lib/api';
  import * as stores from '../stores/messenger';
  import { onMount, onDestroy } from 'svelte';

  // Fixed test identities (64-char hex)
  const TESTER_1 = '1'.repeat(64);
  const TESTER_2 = '2'.repeat(64);

  let currentKey = $state('');
  let myIP = $state('detecting...');
  let tunnelIP = $state('');
  let wsStatus = $state('checking...');
  let intervalId: ReturnType<typeof setInterval>;

  onMount(() => {
    currentKey = getPubkey();
    detectIP();
    checkWS();
    // Poll WS status every 3s
    intervalId = setInterval(checkWS, 3000);
  });

  onDestroy(() => {
    if (intervalId) clearInterval(intervalId);
  });

  async function detectIP() {
    try {
      const resp = await fetch('https://api.ipify.org?format=json');
      const data = await resp.json();
      myIP = data.ip;
    } catch {
      myIP = '(blocked / offline)';
    }
    // Also try local IP via WebRTC
    try {
      const pc = new RTCPeerConnection({ iceServers: [] });
      pc.createDataChannel('');
      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      pc.onicecandidate = (e) => {
        if (e.candidate) {
          const ip = e.candidate.candidate.match(/(\d+\.\d+\.\d+\.\d+)/);
          if (ip && !ip[1].startsWith('127.')) {
            tunnelIP = ip[1];
            pc.close();
          }
        }
      };
    } catch {}
  }

  function checkWS() {
    import('../lib/api').then(api => {
      const status = api.getStatus();
      const t = (window as any).__transportStatus;
      const goOk = !!(status?.connected || t?.go);
      const nostrOk = !!(t?.nostr || (window as any).__nostrChat?.isConnected);
      const parts: string[] = [];
      if (goOk) parts.push('Go WS ✅');
      else parts.push('Go WS ✕');
      if (nostrOk) parts.push('Nostr ✅');
      else parts.push('Nostr ✕');
      wsStatus = parts.join(' · ');
      if (status?.url && goOk) wsStatus += ` (${status.url})`;
    });
  }

  function loginAs(role: 'tester1' | 'tester2') {
    const key = role === 'tester1' ? TESTER_1 : TESTER_2;
    // Store the key in localStorage so initIdentity picks it up
    localStorage.setItem('proxi_demo_role', role);
    // Also set it directly
    setIdentity(key, key); // pubkey=privateKey (test mode)
    // Reload to re-init WS + identity
    window.location.reload();
  }

  async function startDMTest() {
    const isT1 = currentKey === TESTER_1;
    const targetKey = isT1 ? TESTER_2 : TESTER_1;
    const targetName = isT1 ? 'Bob' : 'Alice';

    // Create DM chat with the other tester
    stores.ensureDMChat(targetKey, targetName);
    stores.activeChatId.set(`dm:${targetKey}`);

    // Send a test message
    const msgText = `Привет! Это ${isT1 ? 'Alice' : 'Bob'}. Тест DM ${new Date().toLocaleTimeString()}`;
    try {
      const msg = await sendDM(targetKey, msgText);
      stores.addMessage(`dm:${targetKey}`, msg);
    } catch (e) {
      console.error('DM send failed:', e);
    }
  }

  let copied = $state(false);
  function copyId() {
    navigator.clipboard.writeText(currentKey);
    copied = true;
    setTimeout(() => copied = false, 2000);
  }
</script>

<div class="demo-panel">
  <div class="demo-header">🔧 QA / Test Panel</div>

  <div class="demo-content">
    <!-- IP Address -->
    <div class="info-block">
      <div class="info-label">🌐 Public IP</div>
      <div class="info-value">{myIP}</div>
      {#if tunnelIP}
        <div class="info-sub">Local: {tunnelIP}</div>
      {/if}
    </div>

    <!-- WS Status -->
    <div class="info-block">
      <div class="info-label">⚡ WebSocket</div>
      <div class="info-value ws-{wsStatus.includes('Go WS ✅') || wsStatus.includes('Nostr ✅') ? 'ok' : 'fail'}">{wsStatus}</div>
    </div>

    <!-- Identity -->
    <div class="info-block">
      <div class="info-label">🔑 Мой ID</div>
      <div class="id-row">
        <code class="id-value">{currentKey ? currentKey.slice(0, 12) + '...' + currentKey.slice(-8) : '(empty)'}</code>
        {#if currentKey}
          <button class="copy-btn" onclick={copyId} title="Копировать полный ID">
            {copied ? '✓' : '📋'}
          </button>
        {/if}
      </div>
    </div>

    <!-- Role selector -->
    <div class="role-section">
      <div class="section-title">Login as test user:</div>
      <div class="actions">
        <button class="role-btn {currentKey === TESTER_1 ? 'active t1' : ''}" onclick={() => loginAs('tester1')}>
          👤 Alice
        </button>
        <button class="role-btn {currentKey === TESTER_2 ? 'active t2' : ''}" onclick={() => loginAs('tester2')}>
          👤 Bob
        </button>
      </div>
    </div>

    <!-- Quick DM test -->
    {#if currentKey === TESTER_1 || currentKey === TESTER_2}
      <div class="dm-section">
        <button class="dm-btn" onclick={startDMTest}>
          ⚡ Начать чат с {currentKey === TESTER_1 ? 'Bob' : 'Alice'}
        </button>
        <div class="hint">Откроется чат + отправится тестовое сообщение</div>
      </div>
    {:else}
      <div class="hint center">
        Нажми Alice или Bob, потом открой в другом браузере второго.
      </div>
    {/if}
  </div>
</div>

<style>
  .demo-panel {
    position: fixed;
    bottom: 16px;
    right: 16px;
    width: 300px;
    background: rgba(15, 23, 42, 0.92);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
    z-index: 1000;
    color: #e2e8f0;
    font-family: 'Inter', system-ui, sans-serif;
    font-size: 13px;
  }

  .demo-header {
    background: linear-gradient(90deg, #3b82f6, #8b5cf6);
    padding: 8px 16px;
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 1px;
    border-radius: 12px 12px 0 0;
  }

  .demo-content { padding: 12px 16px; }

  .info-block {
    margin-bottom: 10px;
    padding-bottom: 8px;
    border-bottom: 1px solid rgba(255,255,255,0.05);
  }

  .info-label {
    font-size: 10px;
    text-transform: uppercase;
    color: #64748b;
    margin-bottom: 3px;
  }

  .info-value {
    font-family: 'JetBrains Mono', monospace;
    font-size: 13px;
    color: #f1f5f9;
  }
  .info-sub {
    font-size: 10px;
    color: #475569;
    margin-top: 2px;
  }

  .ws-ok { color: #4ade80; }
  .ws-fail { color: #f87171; }

  .id-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .id-value {
    flex: 1;
    font-family: monospace;
    font-size: 12px;
    color: #94a3b8;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .copy-btn {
    background: rgba(255,255,255,0.05);
    border: 1px solid rgba(255,255,255,0.1);
    border-radius: 6px;
    padding: 2px 8px;
    cursor: pointer;
    color: #e2e8f0;
    font-size: 12px;
  }
  .copy-btn:hover { background: rgba(255,255,255,0.1); }

  .role-section { margin-top: 8px; }
  .section-title {
    font-size: 11px;
    color: #64748b;
    margin-bottom: 6px;
  }

  .actions {
    display: flex;
    gap: 8px;
  }

  .role-btn {
    flex: 1;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: #e2e8f0;
    padding: 8px 12px;
    border-radius: 8px;
    font-size: 13px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .role-btn:hover {
    background: rgba(255,255,255,0.1);
  }
  .role-btn.active.t1 {
    background: rgba(59, 130, 246, 0.25);
    border-color: #3b82f6;
    color: #60a5fa;
  }
  .role-btn.active.t2 {
    background: rgba(139, 92, 246, 0.25);
    border-color: #8b5cf6;
    color: #a78bfa;
  }

  .dm-section { margin-top: 10px; }
  .dm-btn {
    width: 100%;
    background: linear-gradient(135deg, #10b981, #059669);
    border: none;
    color: white;
    font-weight: 600;
    padding: 10px;
    border-radius: 8px;
    cursor: pointer;
    font-size: 13px;
    box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3);
  }
  .dm-btn:hover {
    transform: translateY(-1px);
    box-shadow: 0 6px 16px rgba(16, 185, 129, 0.4);
  }

  .hint {
    font-size: 11px;
    color: #475569;
    margin-top: 4px;
    text-align: center;
  }
  .hint.center { padding: 8px 0; }
</style>
