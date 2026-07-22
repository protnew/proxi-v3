/**
 * T98: Settings store.
 */
import { createStore, createEvent } from 'effector';

export const $settings = createStore({
  theme: 'light' as 'light' | 'dark',
  notifications: true,
  soundEnabled: true,
  vpnAutoConnect: false,
  fontSize: 14,
  language: 'ru',
});

export const updateSetting = createEvent<{ key: string; value: any }>();
export const resetSettings = createEvent();

$settings
  .on(updateSetting, (s, { key, value }) => ({ ...s, [key]: value }))
  .on(resetSettings, () => ({ theme: 'light', notifications: true, soundEnabled: true, vpnAutoConnect: false, fontSize: 14, language: 'ru' }));

// Persist to localStorage
$settings.watch((s) => {
  try { localStorage.setItem('proxi-settings', JSON.stringify(s)); } catch {}
});

// Load from localStorage on init
try {
  const saved = localStorage.getItem('proxi-settings');
  if (saved) { const parsed = JSON.parse(saved); for (const k in parsed) updateSetting({ key: k, value: parsed[k] }); }
} catch {}
