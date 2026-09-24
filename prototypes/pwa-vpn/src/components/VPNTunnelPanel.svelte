<script lang="ts">
  let { incomingEvent, onReject, onAccept }: {
    incomingEvent: { type: string; from: string; wtAddr?: string }
    onReject: () => void
    onAccept: () => void
  } = $props()
</script>
<div class="modal-overlay" data-testid="vpn-incoming-modal">
  <div class="modal">
    <h3>{incomingEvent.type === 'vpn-invite' ? '📡 Друг предлагает VPN!' : '🤝 Друг просит VPN'}</h3>
    <p>От: <code>{incomingEvent.from.slice(0, 24)}…</code></p>
    {#if incomingEvent.wtAddr}<p>Exit: <code>{incomingEvent.wtAddr}</code></p>{/if}
    <div class="modal-buttons">
      <button class="cancel" data-testid="vpn-reject" onclick={onReject}>Отклонить</button>
      <button class="confirm" data-testid="vpn-accept" onclick={onAccept}>
        {incomingEvent.type === 'vpn-invite' ? 'Принять' : 'Разрешить'}
      </button>
    </div>
  </div>
</div>
