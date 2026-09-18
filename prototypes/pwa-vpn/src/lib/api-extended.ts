/**
 * Extended API functions (added 2026-07-23).
 * M-007 Edit/Delete, G-001 Group, M-013 Search, S-001 E2E, VPN-001.
 */
import { request, createGroup, listGroups } from './api';
import type { ApiResponse } from './api';
import { isE2EEnabledLocal, setE2EEnabledLocal } from './api-core';

// ============================================================
// M-013: Full-text search
// ============================================================
export async function searchMessages(query: string, limit = 20): Promise<ApiResponse> {
  return request(`/api/search?q=${encodeURIComponent(query)}&limit=${limit}`);
}

// ============================================================
// M-007: Edit & Delete messages (REST API)
// ============================================================
export async function editMessage(msgId: string, text: string): Promise<ApiResponse> {
  return request('/api/messages/edit', { method: 'POST', body: JSON.stringify({ id: msgId, text }) });
}

export async function deleteMessage(msgId: string): Promise<ApiResponse> {
  return request('/api/messages/delete', { method: 'POST', body: JSON.stringify({ id: msgId }) });
}

// ============================================================
// S-001: E2E encryption toggle (P5: share state with api-core sendDM path)
// ============================================================
export function isE2EEnabled(): boolean { return isE2EEnabledLocal(); }
export function setE2EEnabled(enabled: boolean): void {
  setE2EEnabledLocal(enabled);
  if (typeof localStorage !== 'undefined') localStorage.setItem('proxi_e2e', enabled ? '1' : '0');
}
export function loadE2EPref(): boolean {
  let enabled = true;
  if (typeof localStorage !== 'undefined') {
    enabled = localStorage.getItem('proxi_e2e') !== '0';
  }
  setE2EEnabledLocal(enabled);
  return enabled;
}

// ============================================================
// G-001: Group management helpers
// ============================================================
export async function createGroupUI(name: string, memberPubkeys: string[]): Promise<ApiResponse> {
  return createGroup(name, memberPubkeys);
}

export async function listAllGroups(): Promise<ApiResponse> {
  return listGroups();
}

// ============================================================
// VPN-001: WebRTC proxy toggle
// ============================================================
export async function toggleVPN(enabled: boolean): Promise<ApiResponse> {
  const action = enabled ? 'connect' : 'disconnect';
  return request('/api/vpn/rpc', { method: 'POST', body: JSON.stringify({ action }) });
}

export async function getVPNStatus(): Promise<ApiResponse> {
  return request('/api/vpn/rpc?query=status');
}


// T42B-016: Capacitor bridge — startVpn/stopVpn from JS (Android shell)
export async function nativeStartVpn(): Promise<{ ok: boolean; running: boolean }> {
  const bridge = (window as any).ProxiVpn; // Android WebView @JavascriptInterface (T42B-016)
  if (bridge?.startVpn) {
    return JSON.parse(bridge.startVpn());
  }
  return { ok: false, running: false }; // PWA browser: native VPN unavailable
}

export async function nativeStopVpn(): Promise<{ ok: boolean; running: boolean }> {
  const bridge = (window as any).ProxiVpn;
  if (bridge?.stopVpn) {
    return JSON.parse(bridge.stopVpn());
  }
  return { ok: false, running: false };
}
