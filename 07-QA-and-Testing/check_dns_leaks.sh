#!/bin/bash
# Скрипт обнаружения утечек DNS мимо VPN-туннеля
# Идея: мониторинг порта 53 на всех интерфейсах, кроме wg0/tun0

# Требуется tshark (Wireshark)
if ! command -v tshark &> /dev/null; then
    echo "Ошибка: tshark не установлен. (apt install tshark / choco install wireshark)"
    exit 1
fi

TUNNEL_IFACE="wg0"
MAIN_IFACE="eth0" # Или wlan0 / Wi-Fi

echo "==== Запуск сканирования DNS Leaks ===="
echo "Туннель: $TUNNEL_IFACE, Открытая сеть: $MAIN_IFACE"
echo "Ожидание DNS запросов мимо туннеля (timeout 30s)..."

# Запускаем tshark на 30 секунд. Если поймаем DNS запрос на MAIN_IFACE - это Leak!
# Фильтр: udp port 53
LEAKS=$(tshark -i $MAIN_IFACE -a duration:30 -f "udp port 53" -T fields -e dns.qry.name 2>/dev/null)

if [ -z "$LEAKS" ]; then
    echo "[PASS] DNS утечек не обнаружено. Трафик идет через туннель."
    exit 0
else
    echo "[FAIL] ОБНАРУЖЕНЫ УТЕЧКИ DNS В ОТКРЫТУЮ СЕТЬ!"
    echo "$LEAKS" | sort | uniq
    exit 1
fi
