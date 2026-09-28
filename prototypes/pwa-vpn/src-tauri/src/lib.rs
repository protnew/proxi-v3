mod core_spawn;
use tauri::Manager;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // TZ-FINAL 2.4: fail with a human message instead of a cryptic wry panic
    // when the WebView2 runtime is missing.
    #[cfg(windows)]
    if !webview2_available() {
        native_alert(
            "Proxi requires the Microsoft WebView2 Runtime, which is not installed.\n\
             Install it from https://developer.microsoft.com/microsoft-edge/webview2/ and start Proxi again.",
            "Proxi — WebView2 missing",
        );
        std::process::exit(1);
    }

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
            helper_spawn_plan,
        ])
        .setup(|app| {
            // TZ-FINAL 2.5: real tray menu — Show/Quit entries + left-click
            // restore + close-to-tray (see on_window_event below).
            let show = tauri::menu::MenuItem::with_id(app, "show", "Show Proxi", true, None::<&str>)?;
            let quit = tauri::menu::MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
            let menu = tauri::menu::Menu::with_items(app, &[&show, &quit])?;
            let _tray = tauri::tray::TrayIconBuilder::new()
                .icon(app.default_window_icon().unwrap().clone())
                .menu(&menu)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "quit" => app.exit(0),
                    "show" => {
                        if let Some(w) = app.get_webview_window("main") {
                            w.show().ok();
                            w.set_focus().ok();
                        }
                    }
                    _ => {}
                })
                .on_tray_icon_event(|tray, _event| {
                    // Any click on the icon restores the window.
                    let app = tray.app_handle();
                    if let Some(w) = app.get_webview_window("main") {
                        w.show().ok();
                        w.set_focus().ok();
                    }
                })
                .tooltip("Proxi")
                .build(app)?;
            Ok(())
        })
        .on_window_event(|window, event| {
            // TZ-FINAL 2.5: the close button hides to tray; quitting is via
            // the tray menu.
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                window.hide().ok();
                api.prevent_close();
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

/// TZ-FINAL 2.4: WebView2 Evergreen installs versioned folders under
/// EdgeWebView2\Application, each containing msedgewebview2.dll.
#[cfg(windows)]
fn webview2_available() -> bool {
    use std::path::Path;
    let bases = [
        r"C:\Program Files (x86)\Microsoft\EdgeWebView2\Application",
        r"C:\Program Files\Microsoft\EdgeWebView2\Application",
    ];
    for base in bases {
        let dir = Path::new(base);
        let Ok(entries) = std::fs::read_dir(dir) else {
            continue;
        };
        for entry in entries.flatten() {
            if entry.path().join("msedgewebview2.dll").is_file() {
                return true;
            }
        }
    }
    false
}

/// Zero-dependency MessageBoxW (user32) for pre-webview fatal errors.
#[cfg(windows)]
fn native_alert(text: &str, caption: &str) {
    #[link(name = "user32")]
    extern "system" {
        fn MessageBoxW(hwnd: isize, text: *const u16, caption: *const u16, utype: u32) -> i32;
    }
    let t: Vec<u16> = text.encode_utf16().chain(std::iter::once(0u16)).collect();
    let c: Vec<u16> = caption.encode_utf16().chain(std::iter::once(0u16)).collect();
    const MB_ICONERROR: u32 = 0x0000_0010;
    const MB_OK: u32 = 0x0000_0000;
    unsafe {
        MessageBoxW(0, t.as_ptr(), c.as_ptr(), MB_ICONERROR | MB_OK);
    }
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

/// D8 stage 1: elevated helper args. Does not call vpn_bridge.
#[tauri::command]
fn helper_spawn_plan(bin: String) -> Vec<String> {
    core_spawn::helper_elevated_args(&bin)
}
