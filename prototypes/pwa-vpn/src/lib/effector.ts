/**
 * T83: Effector root domain and core stores.
 * State management for Proxi messenger.
 */
import { createStore, createEvent, createDomain } from 'effector';

export const rootDomain = createDomain('proxi');

// Auth store
export const $isAuthenticated = rootDomain.store(false);
export const $currentUser = rootDomain.store<{ pubkey: string; name?: string } | null>(null);
export const authenticate = rootDomain.event<{ pubkey: string; name?: string }>();
export const logout = rootDomain.event();

$isAuthenticated.on(authenticate, () => true).on(logout, () => false);
$currentUser.on(authenticate, (_, user) => user).on(logout, () => null);

// Connection store
export const $isConnected = rootDomain.store(false);
export const $vpnEnabled = rootDomain.store(false);
export const setConnected = rootDomain.event<boolean>();
export const toggleVpn = rootDomain.event();

$isConnected.on(setConnected, (_, connected) => connected);
$vpnEnabled.on(toggleVpn, (enabled) => !enabled);

// Chat store
export const $messages = rootDomain.store<Record<string, any[]>>({});
export const $activeChat = rootDomain.store<string | null>(null);
export const messageReceived = rootDomain.event<{ chatId: string; message: any }>();
export const setActiveChat = rootDomain.event<string | null>();

$messages.on(messageReceived, (messages, { chatId, message }) => ({
  ...messages,
  [chatId]: [...(messages[chatId] || []), message],
}));

$activeChat.on(setActiveChat, (_, chatId) => chatId);
