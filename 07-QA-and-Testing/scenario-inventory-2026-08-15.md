# Playwright scenario inventory — 2026-08-15

Источник знаменателя: `npx playwright test --list` → **Total: 57 tests in 24 files**.
Это список suite, не «покрытие продукта». Проценты сценариев в этом файле **не считаются**.

## skip в исходниках (2)

- `e2e/vpn-product-crit.spec.ts` — `test.skip` SOCKS5 panel
- `e2e/vpn.spec.ts` — `test.skip` SOCKS5 dev panel

## В suite, но мёртвый транспорт

`e2e/archived-wt/` входит в `testMatch: e2e/**` (3 spec). Не выкидывать из знаменателя.

## Вне suite (не в testMatch)

- `tests/vpn-e2e.spec.ts`
- `tests/e2e.spec.ts`
- `test/e2e-vpn-tab-p2p.spec.ts`

Запрещено: `npx playwright test tests/vpn-e2e.spec.ts` после suite — орфан затирает `.last-run.json`.

## Не публиковать

scenario % · readiness · PASS = list − fail.
