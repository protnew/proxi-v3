# WIRE-SPEC — контракт сообщений Proxi (P22, 2026-09-22)

Единая спека wire-протокола `/ws` и REST. Источник правды кода: `chat/message.go` (Go), `ChatViewModel.kt` (Android), `api-core.ts` (PWA).

## 1. Базовый фрейм `chat.Message` (JSON, `/ws`)

| Поле | Тип | Назначение |
|---|---|---|
| `type` | string | `chat` \| `join` \| `leave` \| `typing` \| `key_exchange` \| `message_edited` \| `message_deleted` \| `voice` |
| `from` / `to` | string | идентичность отправителя/получателя (npub или hex); `to:"broadcast"` — всем |
| `text` | string | тело; префиксы `nip44:`/`v1.` = клиентский шифротекст (хранится as-is) |
| `ts` | int | unix-секунды; `id` — серверный ID сообщения |
| `replyTo`, `replyToText`, `replyToFrom` | string | ответ на сообщение |
| `forwardedFrom` | string | npub исходного автора пересылки |
| `ttl` | int | self-destruct, сек (0 = никогда) |
| **`group`** | string | **P3**: комната группы `group:<id>`; пусто = DM. Проксируется сквозь decode→encode→REST и хранится в `messages.group_id` |
| `is_e2e` / `encrypted` | bool | флаги шифрования; флаг без шифротекста → 422 (fail-closed) |
| `publicKey` | string | для `key_exchange`: JSON сигнала звонка (см. §3) |
| `voiceData` / `voiceDuration` | string/int | голосовое (legacy fallback) |

## 2. Мета-префиксы в `text` (после E2E-дешифровки)

- `filemeta:{json}` — вложение. Поля: `url` (ТОЛЬКО относительный `/api/files/...`; всё прочее получатель обязан отвергнуть — P11), `name` (basename), `size`, `mime`, `kind` (`file`|`voice`), `dur`.
- `groupmeta:{json}` — инвайты/метаданные группы. Получатель: invite → подтверждение пользователем, не forced-join (P29).

## 3. Сигналинг звонков (диалект `key_exchange`, де-факто до решения X3)

`type:"key_exchange"`, `publicKey` = JSON `{kind:"offer"|"answer"|"ice"|"hangup", ...}`.
Получатель обязан роутить в call-менеджер (Android: `CallManager.handleSignal`; PWA: `calls.handleCallSignal` — реализация = P5, ждёт X3).

## 4. Tombstone / edit (P15)

`type:"message_deleted"` / `"message_edited"`: идентификатор — поле **`id`** (НЕ text). `from` = инициатор. Клиенты применяют к локальным сторам (Room/IDB).

## 5. Доставка

- DM: `SendTo(to)` + echo отправителю (multi-device).
- Группа (P3): отправитель шлёт pairwise-fanout (по одному DM каждому участнику) с полем `group`; получатель кладёт в комнату `group:<id>`.
- REST `GET /api/messages` отдаёт свежие N (DESC+reverse, P14), включая `group`.

## 6. Что НЕ входит (ждёт развилок)

- Модель шифрования групп (pairwise/sender-key/MLS/Loro) — X4.
- Транспорт DM и сигналинга — X1/X3.
- Interop-тесты Android↔PWA по этой спеке — MSG-005 (TODO).
