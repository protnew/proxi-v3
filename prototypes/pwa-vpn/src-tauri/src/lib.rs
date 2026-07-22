use tauri::Manager;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            Some(vec![]),
        ))
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .invoke_handler(tauri::generate_handler![
            get_vpn_status,
            start_vpn,
            stop_vpn,
            get_system_info,
        ])
        .setup(|app| {
            // Set up system tray
            let tray = tauri::tray::TrayIconBuilder::new()
                .icon(app.default_window_icon().unwrap().clone())
                .menu(&tauri::menu::Menu::default(app)?)
                .on_menu_event(|app, event| {
                    match event.id.as_ref() {
                        "quit" => app.exit(0),
                        "show" => {
                            if let Some(w) = app.get_webview_window("main") {
                                w.show().ok();
                                w.set_focus().ok();
                            }
                        }
                        _ => {}
                    }
                })
                .tooltip("Indestructible Messenger")
                .build(app)?;
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

/// Get VPN connection status
#[tauri::command]
fn get_vpn_status() -> String {
    // TODO: Check actual WireGuard interface status
    "{\"status\":\"disconnected\",\"peers\":0}".to_string()
}

/// Start VPN connection
#[tauri::command]
async fn start_vpn(config: String) -> Result<String, String> {
    // TODO: Create WireGuard interface, apply config
    // For now, return mock success
    println!("VPN config received: {}", config);
    Ok("{\"status\":\"connecting\"}".to_string())
}

/// Stop VPN connection
#[tauri::command]
fn stop_vpn() -> String {
    // TODO: Tear down WireGuard interface
    "{\"status\":\"disconnected\"}".to_string()
}

/// Get system info for diagnostics
#[tauri::command]
fn get_system_info() -> String {
    let os = std::env::consts::OS;
    let arch = std::env::consts::ARCH;
    format!("{{\"os\":\"{}\",\"arch\":\"{}\"}}", os, arch)
}
