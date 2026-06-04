# Proxi — Unkillable Messenger

An open-source decentralized peer-to-peer messenger with built-in VPN capabilities. No accounts, no phone numbers, no central servers — pure P2P communication that cannot be shut down by any single authority.

## Why This Project Matters

Internet censorship affects over 3 billion people worldwide (Freedom House). In restricted regions, people cannot freely communicate or access information online. Proxi addresses this by creating a decentralized platform where every device is both a client and a relay node.

The core innovation is "Share Internet": one tap turns your device into a VPN endpoint for contacts in censored regions. They connect through you, bypassing firewalls — without any central infrastructure to detect and block. Real peer-to-peer internet freedom.

This is a solo-developer project in active development. The P2P protocol stack, NAT traversal, and encryption layer are functional. Seeking contributors for security audits, protocol improvements, mobile clients, and translations.

## How AI Coding Tools Are Used

Building a P2P protocol is complex — NAT traversal, hole-punching, encrypted key exchange, distributed hash tables. Each component requires careful implementation and extensive testing. AI coding tools are central to development:

- **Protocol implementation**: Translating networking RFCs and P2P research into correct, efficient Go code
- **Security**: AI-assisted auditing of cryptographic implementations and network security
- **Testing**: Generating network simulation tests for edge cases (high latency, packet loss, NAT types)
- **Cross-platform builds**: Maintaining desktop and mobile clients from shared Go code

With Codex access, the plan is to: build mobile clients (iOS/Android), implement relay protocol improvements, create automated penetration testing tools, and add group messaging with efficient multicast.

## Technical Details

- **DHT-based peer discovery**: Distributed hash table for finding peers without central coordination
- **NAT traversal**: UDP hole-punching with relay fallback
- **Double-ratchet encryption**: End-to-end encryption with forward secrecy
- **Go performance**: Goroutines for hundreds of concurrent P2P connections
- **Single binary**: No external dependencies

## Getting Started

```bash
git clone https://github.com/protnew/proxi.git
cd proxi
go build -o proxi
./proxi
```

## License

MIT License
