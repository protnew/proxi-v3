# API Contract: Unkillable Messenger

This document serves as the formal contract between the Go Backend (Webserver + VPN daemon) and the Svelte Frontend application. It documents all available REST endpoints and WebSocket events.

## Base URL
All API calls should be made to: `http://localhost:9999` (or the configured `WEB_PORT`).

## Authentication
Most endpoints require a JWT token in the Authorization header:
`Authorization: Bearer <token>`

---

## 1. Authentication (`/api/auth/*`)

### `POST /api/auth/signup`
Creates a new identity and returns a JWT token.
**Request Body:**
```json
{
  "passphrase": "my-secret-password"
}
```
**Response:**
```json
{
  "token": "jwt-token-string",
  "npub": "npub1..."
}
```

### `POST /api/auth/login`
Authenticates an existing identity.
**Request Body:**
```json
{
  "npub": "npub1...",
  "passphrase": "my-secret-password"
}
```
**Response:**
```json
{
  "token": "jwt-token-string"
}
```

---

## 2. Core Messaging (`/api/messages`, `/ws`)

### `GET /api/messages`
Retrieves chat history.
**Query Params:**
- `limit` (default 50)

### `POST /api/messages`
Sends a new message.
**Request Body:**
```json
{
  "to": "npub1...", // or "broadcast"
  "text": "Hello world!"
}
```

### `WebSocket /ws`
Real-time bi-directional events stream.
**Client -> Server (Authentication):**
```json
{
  "type": "auth",
  "token": "jwt-token-string"
}
```
**Server -> Client (Incoming Message):**
```json
{
  "id": "msg-1234...",
  "from": "npub1...",
  "to": "npub1...",
  "text": "Encrypted or plaintext message",
  "encrypted": true,
  "timestamp": 1718000000
}
```

---

## 3. Contacts & Profiles (`/api/contacts`, `/api/profiles`)

### `GET /api/contacts`
Lists saved contacts.

### `POST /api/contacts`
Adds a contact.
**Request Body:**
```json
{
  "npub": "npub1...",
  "name": "Alice"
}
```

### `GET /api/profiles`
Returns profile data for a set of npubs.
**Query Params:** `npubs=npub1,npub2`

---

## 4. Channels & Groups (`/api/channels`, `/api/groups/*`)

### `GET /api/channels`
Lists available broadcast channels.

### `POST /api/channels`
Creates a channel.
**Request Body:**
```json
{
  "name": "My Channel",
  "description": "Topic description"
}
```

### `POST /api/groups/create`
Creates a private multi-party group.
**Request Body:**
```json
{
  "name": "Secret Group",
  "members": ["npub1...", "npub2..."]
}
```

### `GET /api/groups/list`
Lists the user's groups.

---

## 5. File Transfers (`/api/files/*`)

### `POST /api/files/upload`
Uploads a file (multipart/form-data).
**Form Fields:**
- `file`: The binary file content.
**Response:**
```json
{
  "id": "file-uuid",
  "url": "/api/files/file-uuid"
}
```

### `GET /api/files/{id}`
Downloads a file.

---

## 6. VPN & Network (`/api/vpn/*`, `/api/mesh/*`)

### `POST /api/vpn/rpc`
Executes internal VPN manager commands (e.g., adding peers to the WireGuard interface).

### `GET /api/mesh/peers`
Lists current Mesh network peers connected to this node.

### `GET /api/mesh/stats`
Returns traffic statistics and health of the mesh network.

---

## 7. Advanced Features

### `POST /api/switch/setup`
Configures the dead man's switch.
**Request Body:**
```json
{
  "recipient": "npub1...",
  "message_text": "I am offline.",
  "interval_days": 7
}
```

### `POST /api/switch/check-in`
Resets the dead man's switch timer.

### `POST /api/messages/schedule`
Schedules a message to be sent at a future timestamp.
**Request Body:**
```json
{
  "to": "npub1...",
  "text": "Hello future!",
  "send_at": 1718000000
}
```

---
*Note: This is an internal local PWA-to-Daemon API. All requests to the daemon MUST be authenticated with the JWT token acquired during signup/login.*
