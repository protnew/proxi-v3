<script lang="ts">
  let { friendId = $bindable(''), onCancel, onConfirm }: {
    friendId: string
    onCancel: () => void
    onConfirm: () => void
  } = $props()
</script>
<div class="modal-overlay" onclick={onCancel}>
  <div class="modal" onclick={(e) => e.stopPropagation()} data-testid="vpn-invite-modal">
    <h3>📡 Поделиться VPN с другом</h3>
    <p>ID друга (hex pubkey или demo-ключ 111…/222…):</p>
    <input type="text" placeholder="pubkey друга..." bind:value={friendId} data-testid="vpn-friend-id" />
    <div class="modal-buttons">
      <button class="cancel" onclick={onCancel}>Отмена</button>
      <button class="confirm" data-testid="vpn-share-confirm" onclick={onConfirm}>Раздать</button>
    </div>
  </div>
</div>
