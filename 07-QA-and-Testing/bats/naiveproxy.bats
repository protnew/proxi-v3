#!/usr/bin/env bats

@test "NaiveProxy Caddy server is responding to HTTPS" {
  # Проверка, что порт 443 слушается
  run nc -z localhost 443
  [ "$status" -eq 0 ]
}

@test "NaiveProxy masks as standard web server (Active Probing)" {
  # Запрос без авторизации должен вернуть 200 OK (фейковый сайт) или 401
  run curl -k -I https://localhost
  [[ "$output" == *"HTTP/1.1 200"* ]] || [[ "$output" == *"HTTP/2 200"* ]]
}
