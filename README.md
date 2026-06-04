# Proxi — Unkillable Messenger

An open-source decentralized peer-to-peer messenger with built-in VPN capabilities. No accounts, no phone numbers, no central servers — pure P2P communication that cannot be shut down by any single authority.

## Community Impact

Internet censorship affects over 3 billion people worldwide according to Freedom House. In restricted regions, people cannot freely communicate, access information, or express opinions online. Proxi addresses this by creating a truly decentralized communication platform where every user's device becomes both a client and a relay node.

The "Share Internet" feature is the key innovation: with a single tap, your device becomes a VPN endpoint for your contacts in censored regions. Your friend connects through your device, bypassing firewalls — without any central infrastructure that could be detected and blocked. This is real peer-to-peer internet freedom.

Our community includes digital rights advocates, journalists in restricted regions, privacy-focused developers, and ordinary people who believe communication should be free. As an MIT-licensed open-source project, we encourage security audits, protocol contributions, and translations.

## How We Use AI Coding Tools

Building a P2P protocol stack is complex — NAT traversal, hole-punching, encrypted key exchange, distributed hash tables — each component requires careful implementation and extensive testing. AI coding tools are central to our development:

- **Protocol implementation**: Translating networking RFCs and P2P research papers into correct, efficient Go code
- **Security review**: AI-assisted code auditing for cryptographic implementations and network security
- **Cross-platform builds**: Maintaining desktop and mobile clients from a shared Go codebase
- **Testing**: Generating network simulation tests for edge cases (high latency, packet loss, NAT types)

With Codex access, we could accelerate mobile client development, implement our planned relay protocol improvements, and build automated penetration testing tools to verify security guarantees.

## Technical Details

- **DHT-based peer discovery**: Distributed hash table for finding peers without central coordination
- **NAT traversal**: UDP hole-punching and relay fallback for restrictive firewalls
- **Double-ratchet encryption**: End-to-end encryption that provides forward secrecy and break-in recovery
- **Go performance**: Goroutines handle hundreds of concurrent P2P connections efficiently
- **Minimal footprint**: Single binary with no external dependencies

## Roadmap

- Mobile clients (iOS and Android)
- File sharing through the P2P network
- Group messaging with efficient multicast
- Tor bridge integration for extreme censorship scenarios
- Bandwidth optimization for low-connectivity environments
- Multi-language interface (15+ languages)

## Getting Started

```bash
git clone https://github.com/protnew/proxi.git
cd proxi
go build -o proxi
./proxi
```

## License

MIT License — security audits, protocol contributions, and translations welcome.
