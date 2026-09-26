// LEGACY / dead — canon D8: base is prototypes/pwa-vpn/src-tauri. Do not extend this tree.
// Prevents additional console window on Windows in release
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod vpn_bridge;

use std::sync::Mutex;
use vpn_bridge::VpnState;

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .manage(Mutex::new(VpnState::new()))
        .invoke_handler(tauri::generate_handler![
            vpn_bridge::vpn_start,
            vpn_bridge::vpn_status,
            vpn_bridge::vpn_get_public_key,
            vpn_bridge::vpn_start_exit_node,
            vpn_bridge::vpn_stop_exit_node,
            vpn_bridge::vpn_connect_peer,
            vpn_bridge::vpn_add_peer,
            vpn_bridge::vpn_stop,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
