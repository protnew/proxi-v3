<!--
  T96: Toggle/Switch UI component.
  Used for VPN toggle, dark mode, settings.
-->
<script lang="ts">
  let { checked = false, onChange = () => {}, label = '', disabled = false }: {
    checked?: boolean;
    onChange?: (value: boolean) => void;
    label?: string;
    disabled?: boolean;
  } = $props();

  function toggle() {
    if (disabled) return;
    checked = !checked;
    onChange(checked);
  }
</script>

<div class="toggle-wrapper" class:disabled>
  {#if label}<span class="label">{label}</span>{/if}
  <button
    class="toggle"
    class:checked
    onclick={toggle}
    disabled={disabled}
    role="switch"
    aria-checked={checked}
  >
    <span class="knob"></span>
  </button>
</div>

<style>
  .toggle-wrapper { display: flex; align-items: center; gap: 8px; }
  .label { font-size: 16px; color: var(--text); }
  .toggle {
    width: 48px; height: 24px; border-radius: 12px;
    background: var(--border); border: none; padding: 4px;
    cursor: pointer; transition: all 180ms ease-in-out; position: relative;
  }
  .toggle:hover:not(:disabled) { filter: brightness(1.08); }
  .toggle:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  .toggle.checked { background: var(--accent); }
  .toggle:disabled { opacity: 0.5; cursor: not-allowed; }
  .knob {
    width: 16px; height: 16px; border-radius: 50%;
    background: var(--text-on-accent); transition: transform 180ms ease-in-out;
    box-shadow: var(--shadow);
  }
  .toggle.checked .knob { transform: translateX(24px); }
</style>
