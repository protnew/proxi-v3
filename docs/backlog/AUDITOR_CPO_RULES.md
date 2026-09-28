# AUDITOR RULES + CPO DEFINITION OF READY

## 1. DoR (Definition of Ready)

Задача готова к выполнению если:
1. `task_id`, `title`, `description` заполнены
2. `acceptance_criteria` ≥ 1 проверяемое условие
3. `agent_minutes` между 5 и 20 (leaf-size)
4. `depends_on_task_id` указан или явно NULL
5. `proof_type` ∈ {autotest, api, e2e, playwright, manual, none}
6. `epic` назначен

## 2. DoD (Definition of Done)

Задача DONE только если:
1. `status = 'DONE'`
2. `proof_type ≠ 'none'` — есть подтверждение
3. `test_logs` содержит конкретный результат (PASS count, URL, файл)
4. `updated_at` обновлён
5. Если `proof_type ∈ {autotest,e2e,playwright,api}` — команда проверки записана

## 3. Шкала Proof

| proof_type | Что значит | Доверие |
|------------|-----------|---------|
| autotest | pytest/go test/vitest PASS | Высокое |
| e2e | Playwright/cypress PASS | Высокое |
| playwright | Playwright PASS | Высокое |
| api | curl/RPC вернул ожидаемый результат | Среднее |
| manual | Человек видел | Среднее |
| none | Нет proof | **DONE запрещён** |

## 4. CPO Priority Score

`priority_score = user_value / agent_minutes`

- `user_value` 1–10 (impact на пользователя/продукт)
- `agent_minutes` 5–20 (leaf-size)
- Сортировка: DESC priority_score

## 5. Epic Mapping

| Epic | Блоки |
|------|-------|
| A-Messaging | Auth, MessengerCore, Groups |
| B-VPN-Privacy | VPN, P2PContent |
| C-Quality-Security | QualityGate, TestingInfra, Security, E2EProof |
| D-UserExperience | UI, MVP |
| E-Platform-Scale | DevOps, DocsCompliance, MobileDesktop, Economy |

## 6. Запрещённые маркеры в test_logs

- "100% DONE" без proof → отклонить
- "PASS" без count/command → отклонить
- "works" без evidence → отклонить
