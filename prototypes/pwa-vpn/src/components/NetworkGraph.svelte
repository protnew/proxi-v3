<!--
  T104: Network Graph view — visualizes P2P mesh connections.
-->
<script lang="ts">
  export let peers: { id: string; addr: string; connected: boolean; latency?: number }[] = [];

  $: onlinePeers = peers.filter(p => p.connected);
  $: avgLatency = onlinePeers.length > 0
    ? Math.round(onlinePeers.reduce((sum, p) => sum + (p.latency || 0), 0) / onlinePeers.length)
    : 0;
</script>

<div class="network-graph">
  <h3>Mesh Network</h3>
  <div class="stats">
    <div class="stat">
      <span class="value">{onlinePeers.length}</span>
      <span class="label">Connected</span>
    </div>
    <div class="stat">
      <span class="value">{avgLatency}ms</span>
      <span class="label">Avg Latency</span>
    </div>
  </div>
  <div class="peers-list">
    {#each peers as peer}
      <div class="peer" class:online={peer.connected}>
        <span class="dot"></span>
        <span class="id">{peer.id.slice(0, 12)}...</span>
        {#if peer.latency}<span class="latency">{peer.latency}ms</span>{/if}
      </div>
    {/each}
  </div>
</div>

<style>
  .network-graph { padding: 16px; }
  .stats { display: flex; gap: 24px; margin: 16px 0; }
  .stat { display: flex; flex-direction: column; }
  .value { font-size: 1.5rem; font-weight: bold; }
  .label { font-size: 0.8rem; color: #666; }
  .peer { display: flex; align-items: center; gap: 8px; padding: 8px; border-radius: 4px; }
  .peer.online .dot { background: #4caf50; }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: #ccc; }
  .id { font-family: monospace; font-size: 0.85rem; }
  .latency { margin-left: auto; color: #666; font-size: 0.8rem; }
</style>
