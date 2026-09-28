<!--
  T105: Content card component for content vault.
-->
<script lang="ts">
  let { content }: { content: { id: string; title: string; type: string; size?: number; date?: string } } = $props();
</script>

<div class="content-card">
  <div class="icon">{content.type === 'image' ? '🖼️' : content.type === 'video' ? '🎬' : '📄'}</div>
  <div class="info">
    <h4>{content.title}</h4>
    <div class="meta">
      {#if content.size}<span>{(content.size / 1024 / 1024).toFixed(1)} MB</span>{/if}
      {#if content.date}<span>{content.date}</span>{/if}
    </div>
  </div>
</div>

<style>
  .content-card { display: flex; gap: 12px; padding: 12px; border-radius: 8px; background: #f5f5f5; cursor: pointer; }
  .content-card:hover { background: #e0e0e0; }
  .icon { font-size: 2rem; }
  .info { flex: 1; }
  h4 { margin: 0; font-size: 0.9rem; }
  .meta { display: flex; gap: 12px; font-size: 0.75rem; color: #666; margin-top: 4px; }
</style>
