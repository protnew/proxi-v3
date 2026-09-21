# Messenger + **настоящий** VPN (native Windows)

## Код
- Папка: `.04-Src` (git)
- Ветка: `dev`
- Backend: `:8090` · UI: `:5173`
- **Не Docker**

```powershell
cd "C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src"
pwsh .\scripts\start-messenger-dev.ps1
```

---

## Что такое «настоящий» здесь

| | |
|--|--|
| **Режим REAL** | Локальный **SOCKS5** `127.0.0.1:10808` |
| Трафик | Реальный TCP через процесс сервера (байты ↑↓) |
| Проверка | Кнопка «Проверить IP» → `api.ipify.org` через SOCKS |
| Exit | Upstream SOCKS `host:port` (VPS / Tor `:9050`) |
| **Не делает** | System-wide перехват Windows без Wintun/admin |

System-wide WG/Wintun = ICEBOX (отдельный этап).

---

## Тест VPN (2 минуты)

1. http://127.0.0.1:5173/
2. Панель VPN → **Настоящий SOCKS5**
3. **Включить настоящий SOCKS5**
4. Статус: **ON · SOCKS · socks5**, адрес `127.0.0.1:10808`
5. **Проверить IP через VPN** → видите публичный IP
6. (Опц.) Chrome proxy → SOCKS5 `127.0.0.1:10808`
7. **Отключить VPN**

CLI:
```bash
curl --socks5 127.0.0.1:10808 https://api.ipify.org
```

API:
```json
POST /api/vpn/rpc {"method":"start_real_tunnel","params":{"listen":"127.0.0.1:10808"}}
POST /api/vpn/rpc {"method":"check_egress_ip"}
POST /api/vpn/rpc {"method":"disconnect"}
```

---

## Messenger (2 окна)

1. Chrome + Incognito → :5173  
2. **Мой ID** → 📋  
3. Alice → ID Bob → сообщение  
4. Bob видит текст  

---

## Автотесты

```text
go test . -run "SOCKS5|StartRealTunnel"   # PASS
npx playwright test e2e/vpn.spec.ts
npx playwright test e2e/messenger.spec.ts e2e/alice-bob.spec.ts
```

Live proof (2026-07-28): egress IP `2.54.179.182` via SOCKS, bytes counted.
