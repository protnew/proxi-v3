<script>
  import { onMount } from 'svelte';
  import { contacts, showToast, escHtml } from '../lib/stores.js';

  let contactId = $state('');
  let contactName = $state('');
  let contactPubKey = $state('');
  let contactEndpoint = $state('');
  let isMessengerFriend = $state(true);
  let grantVpnAccess = $state(false);
  let useAsVpnNode = $state(false);

  async function loadContacts() {
    try {
      const r = await fetch('/api/contacts');
      const d = await r.json();
      contacts.set(d.contacts || []);
    } catch (e) {
      console.warn('Failed to load contacts', e);
    }
  }

  async function addContact() {
    if (!contactId.trim()) {
      showToast('❌ ID (npub) обязателен');
      return;
    }
    try {
      const r = await fetch('/api/contacts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: contactId.trim(),
          name: contactName.trim(),
          publicKey: contactPubKey.trim(),
          endpoint: contactEndpoint.trim(),
          isMessengerFriend: isMessengerFriend,
          grantVpnAccess: grantVpnAccess,
          useAsVpnNode: useAsVpnNode
        }),
      });
      const d = await r.json();
      if (!r.ok) {
        showToast('❌ ' + (d.error?.message || 'Ошибка сохранения'));
        return;
      }
      
      // Reset form
      contactId = '';
      contactName = '';
      contactPubKey = '';
      contactEndpoint = '';
      isMessengerFriend = true;
      grantVpnAccess = false;
      useAsVpnNode = false;
      
      showToast('✅ Контакт сохранен!');
      loadContacts();
    } catch (e) {
      showToast('❌ Ошибка сохранения контакта');
    }
  }

  async function removeContact(id, publicKey) {
    if (!confirm('Удалить контакт ' + id + '?')) return;
    try {
      const r = await fetch('/api/contacts', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id, publicKey }),
      });
      if (!r.ok) {
        const d = await r.json();
        showToast('❌ ' + (d.error?.message || 'Ошибка'));
        return;
      }
      showToast('🗑 Контакт удалён');
      loadContacts();
    } catch (e) {
      showToast('❌ Ошибка удаления контакта');
    }
  }

  onMount(() => {
    loadContacts();
  });
</script>

<div class="contacts-panel">
  <div class="header-row">
    <h1>👥 Адресная книга</h1>
  </div>

  <div class="card add-contact">
    <h3>➕ Добавить контакт</h3>
    <div class="form-group">
      <input bind:value={contactId} placeholder="ID (npub) *" class="form-input mono" />
    </div>
    <div class="form-group">
      <input bind:value={contactName} placeholder="Имя (для отображения)" class="form-input" />
    </div>
    <div class="form-group">
      <input bind:value={contactPubKey} placeholder="Публичный ключ (WG PubKey)" class="form-input mono" />
    </div>
    <div class="form-group">
      <input bind:value={contactEndpoint} placeholder="Endpoint (IP:Port)" class="form-input mono" />
    </div>

    <div class="toggles">
      <label class="toggle-row">
        <input type="checkbox" bind:checked={isMessengerFriend} />
        <span>💬 <b>Чат-друг</b> (Показывать в мессенджере)</span>
      </label>
      <label class="toggle-row">
        <input type="checkbox" bind:checked={grantVpnAccess} />
        <span>📡 <b>Дать мой интернет</b> (Разрешить ему подключаться ко мне)</span>
      </label>
      <label class="toggle-row">
        <input type="checkbox" bind:checked={useAsVpnNode} />
        <span>🌐 <b>Мой VPN-узел</b> (Я смогу переключаться на его интернет)</span>
      </label>
    </div>

    <button onclick={addContact} class="submit-btn">Сохранить контакт</button>
  </div>

  <div class="contacts-list">
    <h3>Список контактов</h3>
    {#if $contacts.length === 0}
      <div class="empty">Пока нет контактов</div>
    {:else}
      {#each $contacts as c}
        <div class="contact-card">
          <div class="c-info">
            <div class="c-name">{c.name || 'Без имени'}</div>
            <div class="c-id">{c.id}</div>
            <div class="c-roles">
              {#if c.isMessengerFriend}<span class="role badge-chat">Чат</span>{/if}
              {#if c.grantVpnAccess}<span class="role badge-vpn-client">Берет VPN</span>{/if}
              {#if c.useAsVpnNode}<span class="role badge-vpn-node">Раздает VPN</span>{/if}
            </div>
          </div>
          <button class="remove-btn" onclick={() => removeContact(c.id, c.publicKey)}>✕</button>
        </div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .contacts-panel {
    padding: 24px;
    max-width: 600px;
    overflow-y: auto;
    height: 100%;
  }
  .header-row {
    margin-bottom: 24px;
  }
  .header-row h1 {
    font-size: 20px;
    font-weight: 600;
  }
  .card {
    background: rgba(20, 20, 20, 0.6);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 14px;
    padding: 20px;
    margin-bottom: 20px;
    backdrop-filter: blur(10px);
  }
  .card h3 {
    font-size: 14px;
    color: rgba(255, 255, 255, 0.6);
    margin-bottom: 16px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .form-group {
    margin-bottom: 12px;
  }
  .form-input {
    width: 100%;
    padding: 12px 16px;
    background: rgba(0, 0, 0, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 10px;
    color: #fff;
    font-size: 14px;
    outline: none;
    transition: border-color 0.2s;
  }
  .form-input:focus {
    border-color: #1e88e5;
  }
  .form-input.mono {
    font-family: monospace;
    font-size: 13px;
  }
  .toggles {
    margin: 20px 0;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .toggle-row {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 14px;
    cursor: pointer;
    color: rgba(255, 255, 255, 0.8);
  }
  .toggle-row input[type="checkbox"] {
    width: 18px;
    height: 18px;
    accent-color: #1e88e5;
  }
  .submit-btn {
    width: 100%;
    padding: 14px;
    background: linear-gradient(135deg, #1e88e5, #1565c0);
    color: #fff;
    border: none;
    border-radius: 10px;
    cursor: pointer;
    font-size: 15px;
    font-weight: 600;
    transition: all 0.2s;
  }
  .submit-btn:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 15px rgba(30, 136, 229, 0.3);
  }
  
  .contacts-list h3 {
    font-size: 14px;
    color: rgba(255, 255, 255, 0.6);
    margin-bottom: 16px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .empty {
    color: rgba(255, 255, 255, 0.4);
    text-align: center;
    padding: 20px;
    font-size: 14px;
  }
  .contact-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: rgba(20, 20, 20, 0.6);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    padding: 16px;
    margin-bottom: 12px;
  }
  .c-info {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .c-name {
    font-weight: 600;
    font-size: 15px;
  }
  .c-id {
    font-family: monospace;
    font-size: 12px;
    color: rgba(255, 255, 255, 0.5);
  }
  .c-roles {
    display: flex;
    gap: 8px;
    margin-top: 4px;
  }
  .role {
    font-size: 10px;
    padding: 2px 8px;
    border-radius: 10px;
    font-weight: 600;
    text-transform: uppercase;
  }
  .badge-chat { background: rgba(76, 175, 80, 0.2); color: #81c784; }
  .badge-vpn-client { background: rgba(30, 136, 229, 0.2); color: #64b5f6; }
  .badge-vpn-node { background: rgba(156, 39, 176, 0.2); color: #ba68c8; }
  
  .remove-btn {
    background: rgba(244, 67, 54, 0.1);
    color: #f44336;
    border: 1px solid rgba(244, 67, 54, 0.3);
    width: 32px;
    height: 32px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s;
  }
  .remove-btn:hover {
    background: rgba(244, 67, 54, 0.8);
    color: #fff;
  }
</style>
