#!/bin/bash
# Скрипт обнаружения утечек локальных IP-адресов через WebRTC ICE Candidates
# Принимает на вход лог (например лог браузерной консоли или stdout от Playwright)

LOG_FILE=$1

if [ -z "$LOG_FILE" ]; then
    echo "Использование: $0 <log_file>"
    exit 1
fi

if [ ! -f "$LOG_FILE" ]; then
    echo "Файл $LOG_FILE не найден!"
    exit 1
fi

echo "==== Проверка утечек ICE-кандидатов в $LOG_FILE ===="

# Регулярные выражения для приватных IP (RFC 1918)
# 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
REGEX_192="192\.168\.[0-9]{1,3}\.[0-9]{1,3}"
REGEX_10="10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}"
REGEX_172="172\.(1[6-9]|2[0-9]|3[0-1])\.[0-9]{1,3}\.[0-9]{1,3}"

LEAKS=$(egrep -o "($REGEX_192|$REGEX_10|$REGEX_172)" "$LOG_FILE")

if [ -z "$LEAKS" ]; then
    echo "[PASS] Локальные IP (RFC 1918) не обнаружены в логах ICE-кандидатов."
    echo "Режим TURN-only или mDNS работает корректно."
    exit 0
else
    echo "[FAIL] ОБНАРУЖЕНА УТЕЧКА ПРИВАТНЫХ IP (Скомпрометирована локальная сеть)!"
    echo "Найдены адреса:"
    echo "$LEAKS" | sort | uniq
    exit 1
fi
