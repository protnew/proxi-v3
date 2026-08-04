#!/usr/bin/env bats

@test "MQTT broker is running on port 1883" {
  run nc -z localhost 1883
  [ "$status" -eq 0 ]
}

@test "MQTT WSS is running on port 9001" {
  run nc -z localhost 9001
  [ "$status" -eq 0 ]
}
