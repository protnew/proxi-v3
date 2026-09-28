/** ONB-003: Empty state copy for product surfaces */
export type EmptyKind = 'contacts' | 'chats' | 'peers' | 'vpn_off' | 'messages'

export interface EmptyState {
  kind: EmptyKind
  title: string
  body: string
  cta?: string
}

const RU: Record<EmptyKind, EmptyState> = {
  contacts: {
    kind: 'contacts',
    title: 'Нет контактов',
    body: 'Добавьте друга по QR или npub — без сервера аккаунтов.',
    cta: 'Добавить контакт',
  },
  chats: {
    kind: 'chats',
    title: 'Нет чатов',
    body: 'Начните переписку с контактом. Сообщения E2E на устройстве.',
    cta: 'Новый чат',
  },
  peers: {
    kind: 'peers',
    title: 'Нет peer-соединений',
    body: 'P2P появится после обмена invite или общего Nostr-сигнала.',
    cta: 'Показать мой QR',
  },
  vpn_off: {
    kind: 'vpn_off',
    title: 'VPN выключен',
    body: 'Нажмите «Подключить», чтобы поднять WebRTC-туннель к peer.',
    cta: 'Подключить VPN',
  },
  messages: {
    kind: 'messages',
    title: 'Пока пусто',
    body: 'Напишите первое сообщение. Offline — уйдёт через relay/outbox.',
  },
}

export function getEmptyState(kind: EmptyKind, locale: 'ru' | 'en' = 'ru'): EmptyState {
  if (locale === 'en') {
    const en: Record<EmptyKind, EmptyState> = {
      contacts: { kind: 'contacts', title: 'No contacts', body: 'Add a friend via QR or npub.', cta: 'Add contact' },
      chats: { kind: 'chats', title: 'No chats', body: 'Start a conversation with a contact.', cta: 'New chat' },
      peers: { kind: 'peers', title: 'No peers', body: 'P2P appears after invite or Nostr signal.', cta: 'Show my QR' },
      vpn_off: { kind: 'vpn_off', title: 'VPN off', body: 'Tap Connect to open a WebRTC tunnel.', cta: 'Connect VPN' },
      messages: { kind: 'messages', title: 'No messages yet', body: 'Send the first message.' },
    }
    return en[kind]
  }
  return RU[kind]
}
