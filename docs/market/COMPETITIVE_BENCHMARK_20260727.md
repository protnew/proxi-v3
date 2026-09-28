# Competitive Benchmark — Proxi vs Market Leaders

**Date:** 2026-07-27  
**Role:** CPO  
**Purpose:** Разгон бэклога до плотности лидеров; не занижать scope.

## Sources (official)
- [Proxi (ours)](https://github.com/protnew/proxi)
- [Telegram](https://telegram.org/)
- [Signal](https://signal.org/)
- [Session](https://getsession.org/)
- [Element/Matrix](https://element.io/)
- [Tox](https://tox.chat/)

## Feature matrix (value units)

| Product | Msg | Channels | E2E | P2P store | VPN | No phone | Groups | A/V | Desktop | Mobile | Est. feature units |
|---|---|---|---|---|---|---|---|---|---|---|---|
| [Proxi (ours)](https://github.com/protnew/proxi) | ✅ | ✅ | ✅ | ✅ | 🟡 partial | ✅ | ✅ | 🟡 partial | 🟡 partial | 🟡 partial | **40** |
| [Telegram](https://telegram.org/) | ✅ | ✅ | 🟡 secret_chats_only | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | **200** |
| [Signal](https://signal.org/) | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | **80** |
| [Session](https://getsession.org/) | ✅ | ❌ | ✅ | 🟡 onion_routing | ❌ | ✅ | ✅ | 🟡 limited | ✅ | ✅ | **60** |
| [Element/Matrix](https://element.io/) | ✅ | 🟡 spaces | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | **120** |
| [Tox](https://tox.chat/) | ✅ | ❌ | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | 🟡 limited | **45** |

## Density gap

| Leader | Units | Proxi now (honest) | Gap |
|---|---|---|---|
| Telegram | ~200 | ~40 claimed / **core path broken** | ~160 + reliability |
| Element | ~120 | same | ~80 |
| Signal | ~80 | E2E intent OK, delivery broken | reliability first |

## CPO decision

1. **P0 window:** Auth+DM+AliceBob proof before any parity cosmetics.
2. **Active sprint:** ~38 TODO leaf tasks (10–15 agent-min) on critical path.
3. **ICEBOX:** ~45 parity/VPN/mobile/economy tasks toward Telegram density — pull only after E2E-004.
4. **Differentiation to keep:** no-phone + VPN + P2P content vault (unique combo vs all five).
5. **Full 20×20 matrix:** required on next product fork (e.g. VPN transport choice) via Map-Reduce — not stubbed here.
