//! D8 stage 1=K. Spawn unprivileged Go-core on 127.0.0.1:0 with a per-boot JWT.
//! Does not use vpn_bridge.rs (two bugs + ready notification).

use std::process::{Command, Stdio};

pub struct CoreSpawn {
    pub port: u16,
    pub jwt: String,
}

#[allow(dead_code)]
pub fn spawn_go_core(bin: &str, jwt: &str) -> Result<CoreSpawn, String> {
    if jwt.is_empty() || jwt == "change-me-in-production" {
        return Err("per-boot jwt required".into());
    }
    let mut cmd = Command::new(bin);
    cmd.env("JWT_SECRET_PIN", "1")
        .env("JWT_SECRET", jwt)
        .env("PORT", "0")
        .env("PROXI_BIND", "127.0.0.1")
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    let _child = cmd.spawn().map_err(|e| e.to_string())?;
    Ok(CoreSpawn { port: 0, jwt: jwt.to_string() })
}

pub fn helper_elevated_args(helper_bin: &str) -> Vec<String> {
    vec![
        helper_bin.to_string(),
        "--elevated-spawn".into(),
    ]
}
