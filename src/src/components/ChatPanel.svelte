<script>
  let messages = $state([]);
  let input = $state('');
</script>

<div class="chat-panel">
  <div class="messages">
    <div class="empty">
      <h3>💬 Мессенджер — Phase 1</h3>
      <p>Nostr-протокол. Без серверов. Без цензуры.</p>
    </div>
  </div>
  <div class="input-bar">
    <input bind:value={input} placeholder="Написать сообщение..." disabled />
    <button disabled>→</button>
  </div>
</div>

<style>
  .chat-panel { display: flex; flex-direction: column; height: 100%; }
  .messages { flex: 1; overflow-y: auto; padding: 24px; }
  .empty { text-align: center; margin-top: 40%; opacity: 0.4; }
  .empty h3 { margin-bottom: 8px; }
  .input-bar { display: flex; padding: 12px; border-top: 1px solid #222; gap: 8px; }
  input {
    flex: 1; padding: 10px 16px;
    background: #1a1a1a; border: 1px solid #333;
    border-radius: 10px; color: #e0e0e0; font-size: 14px;
  }
  button {
    padding: 10px 20px; background: #1e88e5;
    color: #fff; border: none; border-radius: 10px; cursor: pointer;
  }
  button:disabled { opacity: 0.3; cursor: not-allowed; }
</style>
