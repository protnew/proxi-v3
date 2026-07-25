# Android App — Connect to Go Backend

## Current State
Android prototype exists at `prototypes/android/` with:
- Kotlin + Compose UI
- Nostr identity generation
- WebSocket connection to Go backend

## Connection Steps
1. Start Go backend: `go run ./cmd/webserver/` (port 8080)
2. In Android app, set server URL: `ws://YOUR_IP:8080/ws`
3. Generate identity (auto on first launch)
4. Connect — WebSocket auth via JWT

## API Endpoints Used
- POST /api/auth/signup — register
- GET /api/identity — get npub
- WS /ws — real-time messaging

## Build APK
```bash
cd prototypes/android
./gradlew assembleDebug
# Output: app/build/outputs/apk/debug/app-debug.apk
```
