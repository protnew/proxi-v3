use serde::{Deserialize, Serialize};
use std::process::{Child, Command, Stdio};
use std::io::{BufRead, BufReader, Write};
use std::sync::Mutex;
use tauri::State;

// Go VPN process state
pub struct VpnState {
    process: Option<Child>,
    public_key: String,
}

impl VpnState {
    pub fn new() -> Self {
        VpnState {
            process: None,
            public_key: String::new(),
        }
    }
}

#[derive(Serialize, Deserialize)]
struct RpcRequest {
    method: String,
    #[serde(default)]
    params: serde_json::Value,
}

#[derive(Serialize, Deserialize)]
struct RpcResponse {
    #[serde(skip_serializing_if = "Option::is_none")]
    result: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<RpcError>,
}

#[derive(Serialize, Deserialize)]
struct RpcError {
    code: i32,
    message: String,
}

// Start VPN Go process
#[tauri::command]
pub fn vpn_start(state: State<Mutex<VpnState>>) -> Result<serde_json::Value, String> {
    let mut vpn = state.lock().map_err(|e| e.to_string())?;

    if vpn.process.is_some() {
        return Ok(serde_json::json!({"status": "already_running"}));
    }

    // Start Go bridge process
    let child = Command::new("./src-vpn-bridge")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::inherit())
        .spawn()
        .map_err(|e| format!("Failed to start VPN: {}", e))?;

    vpn.process = Some(child);
    Ok(serde_json::json!({"status": "started"}))
}

// Get VPN status
#[tauri::command]
pub fn vpn_status(state: State<Mutex<VpnState>>) -> Result<serde_json::Value, String> {
    let mut vpn = state.lock().map_err(|e| e.to_string())?;

    if vpn.process.is_none() {
        return Ok(serde_json::json!({
            "state": "disconnected",
            "publicKey": vpn.public_key,
        }));
    }

    // Send RPC to Go process
    let response = send_rpc(&mut vpn, "get_status", serde_json::Value::Null)?;
    Ok(response)
}

// Get public key for sharing
#[tauri::command]
pub fn vpn_get_public_key(state: State<Mutex<VpnState>>) -> Result<String, String> {
    let vpn = state.lock().map_err(|e| e.to_string())?;
    Ok(vpn.public_key.clone())
}

// Start exit node (share internet)
#[tauri::command]
pub fn vpn_start_exit_node(state: State<Mutex<VpnState>>) -> Result<serde_json::Value, String> {
    let mut vpn = state.lock().map_err(|e| e.to_string())?;
    send_rpc(&mut vpn, "start_exit_node", serde_json::Value::Null)
}

// Stop exit node
#[tauri::command]
pub fn vpn_stop_exit_node(state: State<Mutex<VpnState>>) -> Result<serde_json::Value, String> {
    let mut vpn = state.lock().map_err(|e| e.to_string())?;
    send_rpc(&mut vpn, "stop_exit_node", serde_json::Value::Null)
}

// Connect to friend's exit node
#[tauri::command]
pub fn vpn_connect_peer(
    state: State<Mutex<VpnState>>,
    public_key: String,
    endpoint: String,
) -> Result<serde_json::Value, String> {
    let mut vpn = state.lock().map_err(|e| e.to_string())?;
    send_rpc(
        &mut vpn,
        "connect_to_exit_node",
        serde_json::json!({
            "publicKey": public_key,
            "endpoint": endpoint,
        }),
    )
}

// Add a friend/peer
#[tauri::command]
pub fn vpn_add_peer(
    state: State<Mutex<VpnState>>,
    name: String,
    public_key: String,
    endpoint: String,
) -> Result<serde_json::Value, String> {
    let mut vpn = state.lock().map_err(|e| e.to_string())?;
    send_rpc(
        &mut vpn,
        "add_peer",
        serde_json::json!({
            "name": name,
            "publicKey": public_key,
            "endpoint": endpoint,
        }),
    )
}

// Stop VPN completely
#[tauri::command]
pub fn vpn_stop(state: State<Mutex<VpnState>>) -> Result<serde_json::Value, String> {
    let mut vpn = state.lock().map_err(|e| e.to_string())?;

    if let Some(ref mut child) = vpn.process {
        let _ = send_rpc(&mut VpnState::new(), "disconnect", serde_json::Value::Null);
        let _ = child.kill();
        vpn.process = None;
    }

    Ok(serde_json::json!({"status": "stopped"}))
}

// Internal: send JSON-RPC to Go process
fn send_rpc(
    vpn: &mut VpnState,
    method: &str,
    params: serde_json::Value,
) -> Result<serde_json::Value, String> {
    let child = vpn.process.as_mut().ok_or("VPN not running")?;

    let request = serde_json::json!({
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
    });

    // Write to stdin
    if let Some(ref mut stdin) = child.stdin {
        writeln!(stdin, "{}", request).map_err(|e| format!("Write error: {}", e))?;
        stdin.flush().map_err(|e| format!("Flush error: {}", e))?;
    }

    // Read from stdout
    if let Some(ref mut stdout) = child.stdout {
        let mut reader = BufReader::new(stdout);
        let mut line = String::new();
        reader.read_line(&mut line).map_err(|e| format!("Read error: {}", e))?;

        let response: serde_json::Value =
            serde_json::from_str(&line).map_err(|e| format!("Parse error: {}", e))?;

        if let Some(error) = response.get("error") {
            return Err(error.to_string());
        }

        return Ok(response.get("result").cloned().unwrap_or(serde_json::Value::Null));
    }

    Err("No stdout".to_string())
}
