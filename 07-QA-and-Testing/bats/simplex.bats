#!/usr/bin/env bats

@test "SimpleX SMP server is running on port 5223" {
  run nc -z localhost 5223
  [ "$status" -eq 0 ]
}
