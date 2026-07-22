<script>
  import { onMount } from 'svelte';

  let skipLink;

  onMount(() => {
    // Ensure skip link is available immediately
    if (skipLink) {
      skipLink.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          const target = document.getElementById('main-content');
          if (target) {
            target.focus();
            target.scrollIntoView({ behavior: 'smooth' });
          }
        }
      });
    }
  });

  function handleClick(e) {
    e.preventDefault();
    const target = document.getElementById('main-content');
    if (target) {
      target.focus();
      target.scrollIntoView({ behavior: 'smooth' });
    }
  }
</script>

<a
  href="#main-content"
  class="skip-nav"
  bind:this={skipLink}
  on:click={handleClick}
  aria-label="Skip to main content"
>
  Skip to main content
</a>

<style>
  .skip-nav {
    position: absolute;
    top: -100%;
    left: 0;
    background: #1e88e5;
    color: #fff;
    padding: 8px 16px;
    z-index: 9999;
    font-size: 14px;
    border-radius: 0 0 4px 0;
    text-decoration: none;
    transition: top 0.2s;
    cursor: pointer;
  }
  .skip-nav:focus {
    top: 0;
    outline: 2px solid #fff;
    outline-offset: 2px;
  }
</style>
