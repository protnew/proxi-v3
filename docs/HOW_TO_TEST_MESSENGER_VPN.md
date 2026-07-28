# Как тестировать Messenger + VPN (native Windows)

## Старт

```powershell
cd "C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src"
pwsh .\scripts\start-messenger-dev.ps1
```

| Сервис | URL |
|--------|-----|
| UI | http://127.0.0.1:5173/ |
| API health | http://127.0.0.1:8080/api/health |
| VPN RPC | POST http://127.0.0.1:8080/api/vpn/rpc |

Код: **только** `.04-Src`, ветка **`dev`**. Docker не нужен.

---

## Messenger (2 человека / 2 окна)

1. Chrome → http://127.0.0.1:5173/
2. Chrome Incognito → тот же URL
3. В сайдбаре **Мой ID** → 📋
4. Alice: ✏️ → вставить ID Bob → «Начать чат» → написать
5. Bob: открыть чат → увидеть текст

Авто: `npx playwright test e2e/messenger.spec.ts e2e/alice-bob.spec.ts`

---

## VPN — что реально делает кнопка

| Режим | Что происходит | System-wide интернет? |
|-------|----------------|------------------------|
| **Локальный тест** | `start_local_tunnel` → userspace UDP / stub, IP `10.77.0.1`, статус connected | **Нет** (маршруты Windows не трогаем) |
| **К exit-node** | `connect_to_exit_node` + pubkey + `host:port` | Только если есть реальный peer + kernel wg |
| **Раздать** | `start_exit_node` (sharing) | На Windows без wg = userspace/stub |

### Почему раньше «ничего не происходило»

Старый UI строил **WebRTC DataChannel без peer** (signal = `console.log`). Канал никогда не `open` → статус зависал.

**Сейчас** UI ходит в Go **`/api/vpn/rpc`**.

### Ручной smoke VPN

1. Открыть UI, панель VPN внизу сайдбара (развёрнута)
2. Режим **Локальный тест**
3. **Включить локальный туннель**
4. Ждать: статус **Подключён · userspace**, IP **10.77.0.1**, кнопка **Отключить**
5. **Отключить VPN** → **Отключён**

Авто: `npx playwright test e2e/vpn.spec.ts`

### API smoke (curl / PowerShell)

```json
POST /api/vpn/rpc  {"method":"get_status"}
POST /api/vpn/rpc  {"method":"start_local_tunnel"}
POST /api/vpn/rpc  {"method":"disconnect"}
```

---

## Честные ограничения (не врать в тесте)

- Локальный туннель **проверяет** связку UI↔backend↔VPN manager.
- Это **не** «весь Windows трафик через VPN» без kernel WireGuard / реального exit-node.
- Messenger и VPN — **разные** подсистемы; чат работает без VPN.
