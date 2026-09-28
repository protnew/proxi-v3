import { writable } from 'svelte/store';

export const themes = [
  { id: 'midnight', name: 'Midnight (Dark)' },
  { id: 'telegram', name: 'Telegram' },
  { id: 'discord', name: 'Discord' },
  { id: 'dracula', name: 'Dracula' },
  { id: 'nord', name: 'Nord' },
  { id: 'hacker', name: 'Hacker' },
  { id: 'cyberpunk', name: 'Cyberpunk' },
  { id: 'sunset', name: 'Sunset' },
  { id: 'forest', name: 'Forest' },
  { id: 'ocean', name: 'Ocean' }
];

const storedThemeId = localStorage.getItem('proxi_theme_id') || 'midnight';
export const currentThemeId = writable(storedThemeId);

export function setTheme(id) {
  const theme = themes.find(t => t.id === id);
  if (!theme) return;
  
  localStorage.setItem('proxi_theme_id', id);
  currentThemeId.set(id);
  
  document.documentElement.setAttribute('data-theme', id);
}
