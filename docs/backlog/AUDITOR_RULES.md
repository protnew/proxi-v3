# Supreme Auditor — правила работы с бэклогом Proxi

## Source of truth
- **DB:** `00-Product-and-Agile/01-Backlog-Roadmap/backlog.db`
- **MD mirror:** `BACKLOG.md` (регенерировать из DB, не плодить вторые)
- **Branch:** `dev` для разработки; `main` = prod snapshot

## Запрещено
- Создавать `backlog_*.db` рядом с активным (только archive)
- Ставить DONE без физического test_logs
- Заявлять PASS без вывода терминала
- Файлы ≥500 строк
- Мусор/скрипты вне папки проекта
- Дубли сущностей (второй backlog, второй QA root)

## Обязательный цикл на задачу
1. 10-POV checklist  
2. Security Gate (secrets/input/SQL)  
3. TDD Loop: go test / vitest / playwright / (maestro|bats)  
4. Метрики в `07-QA-and-Testing/03-Coverage-Reports/`  
5. commit + push `dev`

## proof_type enum
| value | значит | можно DONE? |
|---|---|---|
| none | нет proof | нет |
| keyword | grep/exists | нет |
| build | compiles | нет |
| api | HTTP live check | да (слабо) |
| autotest | go/vitest PASS | да |
| e2e | playwright PASS | да |
| manual | browser+screenshot | да вместе с api/autotest |

## Формат приёмки
Успех: `[✅ АУДИТ ПРОЙДЕН. 10/10…]` + таблица метрик  
Провал: `[❌ ОТКЛОНЕНО АУДИТОРОМ]` + корректирующий промпт
