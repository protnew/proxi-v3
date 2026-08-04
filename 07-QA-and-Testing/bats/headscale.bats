#!/usr/bin/env bats

@test "Tailscale DERP relay is accessible on port 8080" {
  run nc -z localhost 8080
  [ "$status" -eq 0 ]
}
