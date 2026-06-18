// Global reactive state store using Svelte 5 runes-like patterns
// This is imported by components for shared state

import { writable } from 'svelte/store';

export const myId = writable('');
export const connState = writable('trying'); // 'on' | 'off' | 'trying'
export const connText = writable('Подключение...');
export const activeTab = writable('chat');
export const messages = writable([]);
export const onlineUsers = writable([]);
export const contacts = writable([]);
export const peers = writable([]);
export const channels = writable([]);
export const groups = writable([]);
export const toastMessage = writable('');
export const toastVisible = writable(false);

const storedTheme = localStorage.getItem('proxi_theme') || 'dark';
export const theme = writable(storedTheme);

export function toggleTheme() {
  theme.update(t => {
    const newTheme = t === 'dark' ? 'light' : 'dark';
    localStorage.setItem('proxi_theme', newTheme);
    document.body.className = newTheme;
    return newTheme;
  });
}

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
