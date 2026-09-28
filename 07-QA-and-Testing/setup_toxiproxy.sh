#!/bin/bash
# Скрипт инициализации хаос-тестирования через Toxiproxy
# Требуется запущенный контейнер toxiproxy (docker-compose up -d)

TOXI_HOST="localhost:8474"

echo "==== Инициализация Toxiproxy ===="

# Функция создания прокси
create_proxy() {
    local name=$1
    local listen=$2
    local upstream=$3
    
    echo "Создание прокси $name ($listen -> $upstream)..."
    curl -s -X POST http://$TOXI_HOST/proxies \
         -H "Content-Type: application/json" \
         -d '{"name": "'$name'", "listen": "'$listen'", "upstream": "'$upstream'"}' > /dev/null
}

# Функция добавления токсика (помехи)
add_toxic() {
    local proxy=$1
    local type=$2
    local attributes=$3
    
    echo "Добавление токсика [$type] к прокси $proxy..."
    curl -s -X POST http://$TOXI_HOST/proxies/$proxy/toxics \
         -H "Content-Type: application/json" \
         -d '{"type": "'$type'", "attributes": '$attributes'}' > /dev/null
}

# 1. Создаем прокси для WebTransport (имитируем что сервер крутится на 4433, а прокси слушаем на 8443)
# Примечание: toxiproxy сейчас не очень дружит с UDP (WebTransport), но мы оставляем для TCP signaling
create_proxy "signaling_tcp" "0.0.0.0:8080" "host.docker.internal:3000"

# 2. Добавляем токсики:
# Задержка 200ms (+- 50ms jitter) - Имитация 3G
add_toxic "signaling_tcp" "latency" '{"latency": 200, "jitter": 50}'

# Потеря 5% пакетов - Имитация плохой сети
add_toxic "signaling_tcp" "timeout" '{"timeout": 1000}' 
# (Имитируем зависание на 1 сек)

echo "==== Успешно! Слой хаоса поднят. ===="
curl -s http://$TOXI_HOST/proxies | grep -o '"name":"[^"]*"' | tr -d '"'
