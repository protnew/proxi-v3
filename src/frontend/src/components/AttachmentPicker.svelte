<script>
  import { createEventDispatcher } from 'svelte';
  import { showToast } from '../lib/stores.js';
  import { icons } from '../lib/icons.js';

  const dispatch = createEventDispatcher();
  
  let uploading = $state(false);
  let fileInput;

  function triggerSelect() {
    fileInput.click();
  }

  async function handleFileSelect(e) {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    
    const file = files[0];
    
    // Check file size (e.g., limit to 50MB)
    if (file.size > 50 * 1024 * 1024) {
      showToast('❌ Файл слишком большой (максимум 50MB)');
      return;
    }

    uploading = true;
    showToast('⏳ Загрузка в IPFS...');

    const formData = new FormData();
    formData.append('file', file);

    try {
      const res = await fetch('/api/ipfs/upload', {
        method: 'POST',
        body: formData,
        // Don't set Content-Type header, let browser set it with boundary
        headers: {
          'Authorization': 'Bearer ' + localStorage.getItem('access_token') // if auth is needed
        }
      });

      if (!res.ok) {
        throw new Error('Upload failed');
      }

      const data = await res.json();
      if (data.cid) {
        showToast('✅ Файл загружен!');
        dispatch('upload', {
          cid: data.cid,
          gatewayUrl: data.gatewayUrl,
          name: file.name,
          size: file.size,
          type: file.type
        });
      } else {
        throw new Error('No CID returned');
      }
    } catch (err) {
      console.error('IPFS upload error:', err);
      showToast('❌ Ошибка загрузки файла');
    } finally {
      uploading = false;
      fileInput.value = ''; // Reset input
    }
  }
</script>

<div class="attachment-picker">
  <input 
    type="file" 
    bind:this={fileInput} 
    onchange={handleFileSelect} 
    style="display: none;" 
  />
  <button 
    class="attach-btn" 
    onclick={triggerSelect} 
    disabled={uploading}
    title="Прикрепить файл (IPFS)"
  >
    {#if uploading}
      <span class="spinner">⏳</span>
    {:else}
      {@html icons.clip || '📎'}
    {/if}
  </button>
</div>

<style>
  .attachment-picker {
    display: inline-block;
  }

  .attach-btn {
    background: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    padding: 8px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: var(--transition-fast);
  }

  .attach-btn:hover {
    color: var(--accent-color);
    background: var(--bg-glass);
  }

  .attach-btn:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .spinner {
    animation: spin 1s linear infinite;
    display: inline-block;
  }

  @keyframes spin {
    100% { transform: rotate(360deg); }
  }
</style>
