<script lang="ts">
  const EMOJI_GROUPS = [
    { name: 'Смайлы', emojis: ['😀','😂','🤣','😊','😍','🥰','😘','😜','🤪','😎','🤓','🧐','😤','😡','🤬','😢','😭','😱','🤯','🥳','🤩','😴','🤮','🤧','😷','🤒','👻','💀','👽','🤖'] },
    { name: 'Жесты', emojis: ['👍','👎','👌','✌️','🤞','🤟','🤘','🤙','👋','🤚','✋','🖖','👏','🙌','🤝','🙏','💪','🖕','✍️'] },
    { name: 'Сердца', emojis: ['❤️','🧡','💛','💚','💙','💜','🖤','🤍','💔','💕','💞','💓','💗','💖','💝'] },
    { name: 'Разное', emojis: ['🔥','⭐','🌟','✨','💫','🌈','☀️','🌙','⚡','💧','🌊','❄️','🌸','🍀','🌍','💯','🏆','🎉','🎊','🎈','🎁','💬','🔒','🔑','💡','🎯','✅','❌'] },
  ]

  let { onSelect, showEmoji = false }: {
    onSelect: (emoji: string) => void;
    showEmoji?: boolean;
  } = $props();

  function pick(e: string) { onSelect(e); showEmoji = false }
</script>

{#if showEmoji}
  <div class="emoji-picker">
    {#each EMOJI_GROUPS as group}
      <div class="group">
        <div class="gn">{group.name}</div>
        <div class="eg">
          {#each group.emojis as emoji}
            <button class="eb" onclick={() => pick(emoji)}>{emoji}</button>
          {/each}
        </div>
      </div>
    {/each}
  </div>
{/if}

<style>
  .emoji-picker { position: absolute; bottom: 60px; left: 60px; width: 300px; max-height: 280px; overflow-y: auto; background: var(--bg-tertiary); border: 1px solid var(--bg-hover); border-radius: 12px; padding: 8px; z-index: 100; box-shadow: 0 4px 20px rgba(0,0,0,0.4); }
  .gn { font-size: 12px; color: var(--text-muted); padding: 4px 8px; }
  .eg { display: flex; flex-wrap: wrap; }
  .eb { width: 34px; height: 34px; display: flex; align-items: center; justify-content: center; font-size: 18px; background: none; border: none; cursor: pointer; border-radius: 6px; padding: 0; }
  .eb:hover { background: var(--bg-hover); }
</style>
