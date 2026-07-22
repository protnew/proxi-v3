# Tauri 2.0 Desktop App — Структура

## Статус: ⏳ Требует установки Rust + Tauri CLI

### Установка (один раз):
```powershell
# 1. Установить Rust
winget install Rustlang.Rustup
rustup default stable

# 2. Установить Tauri CLI
npm install -g @tauri-apps/cli@latest

# 3. Создать desktop проект
cd C:\Сделать\Неубиваемый контент\prototype\
npm create tauri-app@latest -- indestructible-desktop --template svelte-ts

# 4. Или интегрировать в существующий проект
cd pwa-vpn
npm install @tauri-apps/api @tauri-apps/cli
npx tauri init
```

### Компоненты desktop-версии:
1. **WireGuard VPN** — нативный WireGuard через Tauri Rust backend
2. **Системный трей** — сворачивание в трей, уведомления
3. **Автозапуск** — запуск при старте системы
4. **Глобальные хоткеи** — Ctrl+Shift+M для быстрого доступа
5. **Нативные уведомления** — через tauri-plugin-notification

### Интеграция WireGuard:
```rust
// src-tauri/src/vpn.rs
use wireguard_uapi::{get_interface, set_interface};

pub fn start_vpn(config: VpnConfig) -> Result<()> {
    // Создать WireGuard интерфейс
    // Настроить через UAPI/WGQuick
    // Вернуть статус
}
```

### Сборка:
```powershell
cd pwa-vpn
npx tauri build
# → src-tauri/target/release/bundle/msi/*.msi
# → src-tauri/target/release/bundle/nsis/*.exe
```

### Файлы для создания:
```
src-tauri/
├── Cargo.toml
├── tauri.conf.json
├── capabilities/
│   └── default.json
├── icons/
└── src/
    ├── main.rs
    ├── lib.rs
    └── vpn.rs
```
