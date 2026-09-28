<script lang="ts">
  let {
    currentChat,
    showSearch = false,
    searchQ = $bindable(''),
    searchHits = [],
    showHeaderMenu = false,
    onBack,
    goBack,
    onCallClick,
    onSearchClick,
    onMenuClick,
    onSearchInput,
    onAbout,
  }: {
    currentChat: { avatar: string; name: string; typing?: unknown[] }
    showSearch?: boolean
    searchQ: string
    searchHits?: Array<{ chatName: string; message: { text: string } }>
    showHeaderMenu?: boolean
    onBack: () => void
    goBack: () => void
    onCallClick: () => void
    onSearchClick: () => void
    onMenuClick: () => void
    onSearchInput: () => void
    onAbout: () => void
  } = $props()
</script>
<div class="chat-header">
  <button class="back-btn" onclick={onBack} title="Назад">‹ Назад</button>
  <button class="back-btn" onclick={goBack}>←</button>
  <div class="avatar">{currentChat.avatar}</div>
  <div class="peer-info">
    <span class="peer-name">{currentChat.name}</span>
    <span class="peer-status">
      {#if currentChat.typing && currentChat.typing.length > 0}
        <em>печатает...</em>
      {:else}
        онлайн недавно <!-- P31-presence -->
      {/if}
    </span>
  </div>
  <button class="hbtn" type="button" title="Звонок" aria-label="Звонок" onclick={onCallClick}>📞</button>
  <button class="hbtn" type="button" title="Поиск" aria-label="Поиск" onclick={onSearchClick}>🔍</button>
  <button class="hbtn" type="button" title="Меню" aria-label="Меню" onclick={onMenuClick}>⋮</button>
</div>
{#if showSearch}
  <div class="search-panel" data-testid="chat-search-panel">
    <input type="search" placeholder="Поиск…" bind:value={searchQ} oninput={onSearchInput} data-testid="chat-search-input" />
    {#each searchHits.slice(0, 8) as hit}
      <div class="search-hit">{hit.chatName}: {hit.message.text.slice(0, 80)}</div>
    {/each}
  </div>
{/if}
{#if showHeaderMenu}
  <div class="header-menu" data-testid="chat-header-menu">
    <button type="button" onclick={onAbout}>О чате</button>
    <button type="button" onclick={onSearchClick}>Поиск</button>
  </div>
{/if}
