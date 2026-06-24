<script>
  import { themes, currentThemeId, setTheme } from '../lib/theme.js';
</script>

<div class="theme-switcher">
  {#each themes as theme}
    <button 
      class="theme-btn" 
      class:active={$currentThemeId === theme.id}
      onclick={() => setTheme(theme.id)}
      title="{theme.name}"
      data-theme-preview={theme.id}
    >
      {#if $currentThemeId === theme.id}
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
      {/if}
    </button>
  {/each}
</div>

<style>
  .theme-switcher {
    display: flex;
    justify-content: center;
    gap: 12px;
    padding: 12px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border-glass);
    z-index: 100;
    overflow-x: auto;
  }
  
  .theme-btn {
    flex-shrink: 0;
    width: 32px;
    height: 32px;
    border-radius: 50%;
    border: 2px solid transparent;
    cursor: pointer;
    transition: transform 0.2s cubic-bezier(0.34, 1.56, 0.64, 1), box-shadow 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #fff;
  }
  
  .theme-btn:hover {
    transform: scale(1.15);
  }
  
  .theme-btn.active {
    border-color: var(--text-primary);
    box-shadow: 0 0 0 2px var(--bg-panel), 0 0 0 4px var(--text-primary);
  }

  /* Preview colors mapped to the data attributes */
  .theme-btn[data-theme-preview="midnight"] { background: hsl(220, 30%, 15%); }
  .theme-btn[data-theme-preview="telegram"] { background: hsl(211, 82%, 56%); }
  .theme-btn[data-theme-preview="discord"] { background: hsl(235, 86%, 65%); }
  .theme-btn[data-theme-preview="dracula"] { background: hsl(265, 89%, 78%); }
  .theme-btn[data-theme-preview="nord"] { background: hsl(193, 43%, 67%); }
  .theme-btn[data-theme-preview="hacker"] { background: hsl(120, 100%, 50%); color: #000; }
  .theme-btn[data-theme-preview="cyberpunk"] { background: hsl(346, 100%, 50%); }
  .theme-btn[data-theme-preview="sunset"] { background: hsl(355, 100%, 71%); }
  .theme-btn[data-theme-preview="forest"] { background: hsl(145, 63%, 42%); }
  .theme-btn[data-theme-preview="ocean"] { background: hsl(195, 100%, 35%); }
</style>
