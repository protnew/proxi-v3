<script lang="ts">
  let { friendId = $bindable(''), onCancel, onConfirm }: {
    friendId: string
    onCancel: () => void
    onConfirm: () => void
  } = $props()
</script>
<div class="modal-overlay" onclick={onCancel}>
  <div class="modal" onclick={(e) => e.stopPropagation()}>
    <h3>🤝 Запросить VPN у друга</h3>
    <p>ID друга (hex pubkey):</p>
    <input type="text" placeholder="pubkey друга..." bind:value={friendId} />
    <div class="modal-buttons">
      <button class="cancel" onclick={onCancel}>Отмена</button>
      <button class="confirm" onclick={onConfirm}>Запросить</button>
    </div>
  </div>
</div>
