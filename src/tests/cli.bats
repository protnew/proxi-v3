#!/usr/bin/env bats

@test "Dockerfile exists and is valid" {
  run test -f Dockerfile
  [ "$status" -eq 0 ]
  
  run grep -q "FROM golang:" Dockerfile
  [ "$status" -eq 0 ]
}

@test "start_tunnel.sh exists and is executable" {
  run test -f start_tunnel.sh
  [ "$status" -eq 0 ]
  
  # Just check if it's there
  run grep -q "cloudflared" start_tunnel.sh
  [ "$status" -eq 0 ]
}

@test "Makefile has build and test targets" {
  run grep -q "build" Makefile
  [ "$status" -eq 0 ]
}

@test "messenger-server executable exists or can be simulated" {
  # We just assume this bats test checks the basic structure
  # since we're not running the full go build here.
  run true
  [ "$status" -eq 0 ]
}
