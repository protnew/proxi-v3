# WAN-тест ПК↔ПК

Кто гоняет: Алексей, после инсталлятора. Этот файл тест не запускает.

1. Две машины Windows 10 1809+ в разных сетях. Один ПК и loopback не считаются.
2. На обеих: инсталлятор. Если SmartScreen — More info, затем Run. Подписи нет, это ограничение волны.
3. Не жать Connect, пока служба не видна: `sc query proxi04-vpn-helper` = есть, StartAutomatic. (Имя с 2026-09-25 — проектное; `ProxiHelper` = старое, чужое.)
4. Донор: «Дать VPN» только с адресом или onion. Пустой инвайт — ошибка, не «раздаю». Core RPC: `create_invite` → `start_egress_listener` → отдать {onion, wtAddr, certHash, token, exp}.
5. Скопировать строку `proxi+vpn://v1?…` или QR. Не путать с QR контакта.
6. Клиент: вставить строку, принять → UI вызывает `connect_invite` (first-frame auth {npub,token,sig} до CONNECT). В статусе leg `onion:` или `wt:` + endpoint донора, не WebRTC-offer.
7. Через туннель записать `proof.txt`: client_ip_via_tunnel, donor_ip, match, killswitch, udp_mode, timestamp. Место для env и iperf оставить пустым, если замера нет.
8. Pass: IP клиента через туннель = IP донора, match=yes, killswitch=on, udp_mode не пустой.
9. Fail: тот же IP без туннеля, пустой инвайт, служба не RUNNING.
10. Откат: `proxi04-vpn-helper.exe --disconnect` (чистит split-routes адаптера `Proxi0` + kill-switch), затем `--uninstall`. Если сеть мертва — команда из `WFP-ROLLBACK.txt`. Повторный uninstall должен кончиться 0.
11. При провале: `proxi04-vpn-helper.exe --diag` → zip-дамп в каталоге рядом, файл не удалять.
12. После простоя dead-man's-switch может сработать по старому check-in. Это ожидаемо.

Инсталлятор без подписи. Манифест sha256 лежит рядом с файлом установки.
