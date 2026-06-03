// Global reactive state store using Svelte 5 runes-like patterns
// This is imported by components for shared state

import { writable } from 'svelte/store';

export const myId = writable('');
export const connState = writable('trying'); // 'on' | 'off' | 'trying'
export const connText = writable('Подключение...');
export const activeTab = writable('chat');
export const messages = writable([]);
export const onlineUsers = writable([]);
export const peers = writable([]);
export const channels = writable([]);
export const groups = writable([]);
export const toastMessage = writable('');
export const toastVisible = writable(false);

// Toast helper
export function showToast(msg) {
  toastMessage.set(msg);
  toastVisible.set(true);
  setTimeout(() => toastVisible.set(false), 2500);
}

// Escape HTML helper
export function escHtml(s) {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}
