# Proxi - Unkillable Messenger

An open-source decentralized peer-to-peer messenger with built-in VPN capabilities and content distribution. No accounts, no phone numbers, no central servers. Pure P2P communication that cannot be shut down by any single authority.

## Why This Matters

Internet censorship affects over 3 billion people worldwide according to Freedom House. In restricted regions people cannot freely communicate, access information, or express opinions online. Traditional messengers rely on central servers that can be blocked, monitored, or shut down. Even VPN services can be detected and blocked through deep packet inspection. Proxi takes a fundamentally different approach.

Every user device becomes both a client and a relay node. There is no central server to block, no company to subpoena, no single point of failure. The network routes around damage automatically because every node is equal.

## Core Innovation: Share Internet

The key feature is "Share Internet". One tap turns your device into a VPN endpoint for your contacts in censored regions. Your friend connects through your device, bypassing firewalls and surveillance, without any central infrastructure that could be detected and blocked. This is real peer-to-peer internet sharing.

You do not need to be a technical expert. One button. Your friend gets internet access through your device. That simple.

## Technical Architecture

- **DHT-based peer discovery**: Distributed hash table for finding peers without central coordination or bootstrap servers
- **NAT traversal**: UDP hole-punching with relay fallback for restrictive firewalls and symmetric NATs
- **Double-ratchet encryption**: End-to-end encryption providing forward secrecy and break-in recovery, same protocol used by Signal
- **Go performance**: Goroutines handle hundreds of concurrent P2P connections efficiently with minimal memory footprint
- **Single binary**: No external dependencies, cross-compile for any platform Go supports

## Current State

Solo-developer project in active development. The core P2P stack is functional:

- Peer discovery via DHT
- NAT traversal with hole-punching
- Encrypted messaging between peers
- VPN relay functionality
- Go-based cross-platform binary

Seeking contributors for security audits, mobile client development, protocol improvements, and translations.

## How AI Coding Tools Are Used

Building a P2P protocol stack is inherently complex. NAT traversal, hole-punching, encrypted key exchange, distributed hash tables, and relay protocols each require careful implementation. AI coding tools are central to the development workflow:

- Translating networking RFCs and P2P research papers into correct, efficient Go code
- AI-assisted security auditing of cryptographic implementations and network protocols
- Generating network simulation tests for edge cases like high latency, packet loss, and unusual NAT configurations
- Cross-platform build management from shared Go codebase

With Codex access the plan is to build iOS and Android mobile clients with native UI, implement relay protocol improvements for better throughput, create automated penetration testing tools, and add group messaging with efficient multicast routing.

## Roadmap

- [ ] Mobile clients (iOS and Android with native UI)
- [ ] File sharing through the P2P network
- [ ] Group messaging with efficient multicast
- [ ] Tor bridge integration for extreme censorship scenarios
- [ ] Bandwidth optimization for satellite and low-connectivity environments
- [ ] Multi-language interface (15+ languages)
- [ ] External security audit

## Getting Started

```bash
git clone https://github.com/protnew/proxi.git
cd proxi
go build -o proxi
./proxi
```

## Contributing

Security audits, protocol contributions, mobile development, and translations all welcome. This project deals with cryptography and network security so careful code review is especially valued.

## License

MIT License
