# Proxi - Unkillable Messenger

A decentralized peer-to-peer messenger with built-in VPN capabilities. No accounts, no phone numbers, no central servers. Pure P2P communication that cannot be shut down by any single authority.

## Why This Project Exists

Internet censorship affects over 3 billion people worldwide (Freedom House). In restricted regions people cannot freely communicate or access information. Traditional messengers rely on central servers that can be blocked or shut down. Even VPN services can be detected and blocked.

Proxi takes a different approach: every device is both a client and a relay node. No central server to block, no company to subpoena, no single point of failure.

## Core Innovation: Share Internet

One tap turns your device into a VPN endpoint for contacts in censored regions. Your friend connects through you, bypassing firewalls, without any central infrastructure to detect and block. Real peer-to-peer internet sharing.

## Current State

Solo-developer project. The core P2P stack is functional:

- Peer discovery via distributed hash table (DHT)
- NAT traversal with UDP hole-punching and relay fallback
- End-to-end encrypted messaging (double-ratchet protocol, same as Signal)
- VPN relay functionality
- Go-based single binary with no external dependencies

## Vision

Build the definitive anti-censorship communication platform: iOS and Android mobile clients, file sharing through the P2P network, group messaging with efficient multicast, Tor bridge integration for extreme censorship scenarios, bandwidth optimization for satellite connections, and 15+ language interface. Make free communication available to everyone on the planet.
