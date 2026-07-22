<script lang="ts">
  import { setIdentity, getPubkey, sendDM } from '../lib/api';
  import * as stores from '../stores/messenger';
  import { onMount } from 'svelte';

  const TESTER_1 = '1111111111111111111111111111111111111111111111111111111111111111';
  const TESTER_2 = '2222222222222222222222222222222222222222222222222222222222222222';

  let currentKey = $state('');

  onMount(() => {
    currentKey = getPubkey();
  });

  function loginAs(role: 'tester1' | 'tester2') {
    const key = role === 'tester1' ? TESTER_1 : TESTER_2;
    setIdentity(key);
    
    // Clear chats for clean test
    stores.chats.set([]);
    stores.contacts.set([]);
    
    // Reload page to re-init everything with new key
    window.location.reload();
  }

  async function connectToOther() {
    const isT1 = currentKey === TESTER_1;
    const targetKey = isT1 ? TESTER_2 : TESTER_1;
    const targetName = isT1 ? 'Tester 2 (Bob)' : 'Tester 1 (Alice)';
    
    stores.ensureDMChat(targetKey, targetName);
    stores.activeChatId.set(`dm:${targetKey}`);
    
    const msgText = `Привет! Я ${isT1 ? 'Tester 1' : 'Tester 2'}. Это автоматический E2E тест соединения. 🚀`;
    const msg = await sendDM(targetKey, msgText);
    stores.addMessage(`dm:${targetKey}`, msg);
  }
</script>

<div class="demo-panel">
  <div class="demo-header">QA / Demo Tools</div>
  <div class="demo-content">
    <div class="status">
      {#if currentKey === TESTER_1}
        <span class="badge t1">Logged in as Tester 1</span>
      {:else if currentKey === TESTER_2}
        <span class="badge t2">Logged in as Tester 2</span>
      {:else}
        <span class="badge regular">Regular User</span>
      {/if}
    </div>
    
    <div class="actions">
      <button onclick={() => loginAs('tester1')} class={currentKey === TESTER_1 ? 'active' : ''}>
        👤 Alice (T1)
      </button>
      <button onclick={() => loginAs('tester2')} class={currentKey === TESTER_2 ? 'active' : ''}>
        👤 Bob (T2)
      </button>
    </div>

    {#if currentKey === TESTER_1 || currentKey === TESTER_2}
      <div class="connect-action">
        <button class="auto-connect-btn" onclick={connectToOther}>
          ⚡ Auto-Connect & Send
        </button>
      </div>
    {/if}
  </div>
</div>

<style>
  .demo-panel {
    position: fixed;
    bottom: 24px;
    right: 24px;
    width: 280px;
    background: rgba(15, 23, 42, 0.7);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5), inset 0 1px 0 rgba(255, 255, 255, 0.1);
    z-index: 1000;
    color: #fff;
    font-family: 'Inter', system-ui, sans-serif;
  }

  .demo-header {
    background: linear-gradient(90deg, #3b82f6, #8b5cf6);
    padding: 10px 16px;
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 1px;
    text-shadow: 0 1px 2px rgba(0,0,0,0.3);
    border-top-left-radius: 15px;
    border-top-right-radius: 15px;
  }

  .demo-content {
    padding: 16px;
  }

  .status {
    margin-bottom: 12px;
    text-align: center;
  }

  .badge {
    display: inline-block;
    padding: 4px 10px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 600;
  }
  .badge.t1 { background: rgba(59, 130, 246, 0.2); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.3); }
  .badge.t2 { background: rgba(139, 92, 246, 0.2); color: #a78bfa; border: 1px solid rgba(139, 92, 246, 0.3); }
  .badge.regular { background: rgba(156, 163, 175, 0.2); color: #9ca3af; border: 1px solid rgba(156, 163, 175, 0.3); }

  .actions {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
  }

  button {
    flex: 1;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: #e2e8f0;
    padding: 8px;
    border-radius: 8px;
    font-size: 12px;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  button:hover {
    background: rgba(255, 255, 255, 0.1);
    border-color: rgba(255, 255, 255, 0.2);
  }

  button.active {
    background: rgba(255, 255, 255, 0.15);
    border-color: rgba(255, 255, 255, 0.3);
    box-shadow: inset 0 0 10px rgba(0,0,0,0.2);
  }

  .auto-connect-btn {
    width: 100%;
    background: linear-gradient(135deg, #10b981, #059669);
    border: none;
    color: white;
    font-weight: 600;
    padding: 10px;
    text-shadow: 0 1px 2px rgba(0,0,0,0.2);
    box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3);
  }
  .auto-connect-btn:hover {
    background: linear-gradient(135deg, #34d399, #10b981);
    transform: translateY(-1px);
    box-shadow: 0 6px 16px rgba(16, 185, 129, 0.4);
  }
  .auto-connect-btn:active {
    transform: translateY(1px);
  }
</style>
