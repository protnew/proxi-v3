# Ручной тест: 2 пользователя (native)

## Старт
```powershell
pwsh scripts/start-messenger-dev.ps1
```
- UI http://127.0.0.1:5173/
- API http://127.0.0.1:8080/api/health

## Alice → Bob
| Шаг | Alice | Bob |
|-----|-------|-----|
| 1 | Chrome → :5173 | Incognito → :5173 |
| 2 | New Chat → 📋 свой ID | New Chat → 📋 свой ID |
| 3 | Вставить ID Bob → Начать чат | — |
| 4 | Написать сообщение | — |
| 5 | — | Чат с Alice → видит текст |

Авто: `npx playwright test e2e/messenger.spec.ts e2e/alice-bob.spec.ts`
