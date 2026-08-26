<!-- T87: UI Button component -->
<script lang="ts">
  let { variant = 'primary', size = 'md', disabled = false, loading = false, onclick }: {
    variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
    size?: 'sm' | 'md' | 'lg';
    disabled?: boolean;
    loading?: boolean;
    onclick?: () => void;
  } = $props();
</script>

<button class="btn {variant} {size}" {disabled} onclick>
  {#if loading}<span class="spinner"></span>{/if}
  <slot />
</button>

<style>
  .btn { display: inline-flex; align-items: center; gap: 8px; border: none; border-radius: 8px; cursor: pointer; font-weight: 500; transition: opacity 0.15s; }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .primary { background: #6200ee; color: white; }
  .secondary { background: #e0e0e0; color: #333; }
  .danger { background: #d32f2f; color: white; }
  .ghost { background: transparent; color: #6200ee; }
  .sm { padding: 4px 12px; font-size: 0.8rem; }
  .md { padding: 8px 16px; font-size: 0.9rem; }
  .lg { padding: 12px 24px; font-size: 1rem; }
  .spinner { width: 16px; height: 16px; border: 2px solid rgba(255,255,255,0.3); border-top-color: white; border-radius: 50%; animation: spin 0.6s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>
