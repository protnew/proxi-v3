<!--
  T96: Toggle/Switch UI component.
  Used for VPN toggle, dark mode, settings.
-->
<script lang="ts">
  export let checked = false;
  export let onChange: (value: boolean) => void = () => {};
  export let label = '';
  export let disabled = false;

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
    on:click={toggle}
    disabled={disabled}
    role="switch"
    aria-checked={checked}
  >
    <span class="knob"></span>
  </button>
</div>

<style>
  .toggle-wrapper { display: flex; align-items: center; gap: 8px; }
  .label { font-size: 0.9rem; }
  .toggle {
    width: 44px; height: 24px; border-radius: 12px;
    background: #ccc; border: none; padding: 2px;
    cursor: pointer; transition: background 0.2s; position: relative;
  }
  .toggle.checked { background: #6200ee; }
  .toggle:disabled { opacity: 0.5; cursor: not-allowed; }
  .knob {
    width: 20px; height: 20px; border-radius: 50%;
    background: white; transition: transform 0.2s;
    box-shadow: 0 1px 3px rgba(0,0,0,0.2);
  }
  .toggle.checked .knob { transform: translateX(20px); }
</style>
