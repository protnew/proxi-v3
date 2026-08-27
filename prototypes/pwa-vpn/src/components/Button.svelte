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
  .btn { display: inline-flex; align-items: center; gap: 8px; border: none; border-radius: 8px; cursor: pointer; font-weight: 500; transition: all 180ms ease-in-out; }
  .btn:hover:not(:disabled) { filter: brightness(1.08); }
  .btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .primary { background: var(--accent); color: var(--text-on-accent); }
  .secondary { background: var(--bg-tertiary); color: var(--text); border: 1px solid var(--border); }
  .danger { background: var(--danger); color: var(--text-on-accent); }
  .ghost { background: transparent; color: var(--accent); }
  .sm { padding: 4px 12px; font-size: 12px; }
  .md { padding: 8px 16px; font-size: 16px; }
  .lg { padding: 12px 24px; font-size: 16px; }
  .spinner { width: 16px; height: 16px; border: 2px solid color-mix(in srgb, var(--text-on-accent) 30%, transparent); border-top-color: var(--text-on-accent); border-radius: 50%; animation: spin 0.8s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>
