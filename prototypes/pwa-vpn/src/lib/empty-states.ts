/** Empty state copy for product surfaces */
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
    body: 'Добавьте друга по QR или адресу из его профиля.',
    cta: 'Добавить контакт',
  },
  chats: {
    kind: 'chats',
    title: 'Нет переписок',
    body: 'Начните чат с другом. Переписка остаётся на этом устройстве.',
    cta: 'Новый чат',
  },
  peers: {
    kind: 'peers',
    title: 'Пока нет связи с другом',
    body: 'Связь появится после обмена приглашением.',
    cta: 'Показать мой QR',
  },
  vpn_off: {
    kind: 'vpn_off',
    title: 'VPN выключен',
    body: 'Нажмите «Подключить», чтобы открыть канал к другу.',
    cta: 'Подключить',
  },
  messages: {
    kind: 'messages',
    title: 'Пока пусто',
    body: 'Напишите первое сообщение. Если друг офлайн — уйдёт позже.',
  },
}

export function getEmptyState(kind: EmptyKind, locale: 'ru' | 'en' = 'ru'): EmptyState {
  if (locale === 'en') {
    const en: Record<EmptyKind, EmptyState> = {
      contacts: { kind: 'contacts', title: 'No contacts', body: 'Add a friend via QR or their address.', cta: 'Add contact' },
      chats: { kind: 'chats', title: 'No chats', body: 'Start a conversation with a friend.', cta: 'New chat' },
      peers: { kind: 'peers', title: 'Not connected yet', body: 'Connection appears after you exchange an invite.', cta: 'Show my QR' },
      vpn_off: { kind: 'vpn_off', title: 'VPN off', body: 'Tap Connect to open a channel to a friend.', cta: 'Connect' },
      messages: { kind: 'messages', title: 'No messages yet', body: 'Send the first message. Offline friends get it later.' },
    }
    return en[kind]
  }
  return RU[kind]
}
