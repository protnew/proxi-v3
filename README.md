# 🔥 Proxi — Unkillable Messenger

A decentralized peer-to-peer messenger with built-in VPN capabilities and a content distribution platform. No accounts, no phone numbers, no central servers — pure peer-to-peer communication that cannot be shut down.

## The Problem We're Solving

In many regions around the world, internet access is censored, monitored, or restricted. Traditional messengers rely on central servers that can be blocked by governments or taken offline. Even VPN services can be detected and blocked. Proxi takes a fundamentally different approach: every user's device becomes both a client and a relay node.

The killer feature is **"Share Internet"**: with a single tap, your device becomes a VPN endpoint for your contacts. Your friend in a censored region connects through your device, bypassing firewalls and surveillance — without any central infrastructure that could be blocked. This is real peer-to-peer internet sharing.

Built with Go for maximum performance, minimal resource usage, and cross-platform support. The P2P architecture means there is no single point of failure — kill one node, the network routes around it.

## Core Features

- **P2P Messaging**: Direct encrypted communication between devices, no server intermediary
- **VPN Sharing**: Turn your device into a VPN node for contacts in censored regions
- **Content Distribution**: Share files, media, and documents through the P2P network
- **No Registration**: No accounts, phone numbers, or email required — just start talking
- **End-to-End Encryption**: All messages encrypted by default, nobody can read them in transit
- **NAT Traversal**: Works behind firewalls and NATs using hole-punching techniques
- **Offline Message Queue**: Messages delivered when both parties come online
- **Multi-Platform**: Desktop and mobile clients planned

## Why Go?

Go was chosen for its excellent concurrency model (goroutines handle hundreds of P2P connections effortlessly), small binary sizes, cross-platform compilation, and built-in networking libraries. The entire P2P stack fits in a single binary with no external dependencies.

## Getting Started

```bash
git clone https://github.com/protnew/proxi.git
cd proxi
go build -o proxi
./proxi
```

## Architecture

Proxi uses a distributed hash table (DHT) for peer discovery, libp2p-inspired protocols for NAT traversal, and double-ratchet encryption for message security. Each node maintains a routing table of known peers and can relay traffic for others.

## License

MIT License
