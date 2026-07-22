import { z } from 'zod';

// Схемы для аутентификации
export const AuthSchema = z.object({
  npub: z.string().min(1, 'NPUB обязателен'),
  username: z.string().optional(),
});

// Схемы для сообщений
export const SendMessageSchema = z.object({
  text: z.string().min(1, 'Текст сообщения не может быть пустым'),
  sendAt: z.number(),
  recipient: z.string().optional(),
  sender: z.string().min(1, 'Отправитель обязателен'),
});

// Схемы для профиля
export const IdentityUpdateSchema = z.object({
  npub: z.string().min(1, 'NPUB обязателен'),
  displayName: z.string().optional(),
  avatarUrl: z.string().url('Некорректный URL').optional().or(z.literal('')),
  bio: z.string().optional(),
});

// Схемы для контактов
export const ContactSchema = z.object({
  id: z.string().min(1, 'ID контакта обязателен'),
});

// Схемы для групп
export const CreateGroupSchema = z.object({
  name: z.string().min(1, 'Имя группы обязательно'),
  creatorNpub: z.string().min(1, 'NPUB создателя обязателен'),
  members: z.array(z.string()).optional(),
});

// Схемы для каналов
export const CreateChannelSchema = z.object({
  id: z.string().min(1, 'ID канала обязателен'),
  text: z.string().min(1, 'Описание канала обязательно'),
});

export const JoinChannelSchema = z.object({
  channelName: z.string().min(1, 'Имя канала обязательно'),
  streamerId: z.string().min(1, 'Streamer ID обязателен'),
});

// Схемы для реакций
export const ReactMessageSchema = z.object({
  messageId: z.string().min(1, 'ID сообщения обязателен'),
  userNpub: z.string().min(1, 'NPUB обязателен'),
  emoji: z.string().min(1, 'Эмодзи обязателен'),
});
