/**
 * @vitest-environment jsdom
 * Cover Tauri branch of desktop.ts via resetModules
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

describe('desktop.ts Tauri path', () => {
  beforeEach(() => {
    vi.resetModules();
    (window as any).__TAURI_INTERNALS__ = {};
  });
  afterEach(() => {
    delete (window as any).__TAURI_INTERNALS__;
    vi.resetModules();
    vi.unmock('@tauri-apps/api/core');
    vi.unmock('@tauri-apps/plugin-notification');
  });

  it('getIsDesktop true when Tauri present', async () => {
    vi.doMock('@tauri-apps/api/core', () => ({
      invoke: vi.fn().mockResolvedValue(JSON.stringify({ status: 'up', peers: 2 })),
    }));
    const d = await import('../src/lib/desktop');
    expect(d.getIsDesktop()).toBe(true);
  });

  it('getDesktopVpnStatus parses invoke result', async () => {
    vi.doMock('@tauri-apps/api/core', () => ({
      invoke: vi.fn().mockResolvedValue(JSON.stringify({ status: 'connected', peers: 3 })),
    }));
    const d = await import('../src/lib/desktop');
    const r = await d.getDesktopVpnStatus();
    expect(r).toEqual({ status: 'connected', peers: 3, phase: 'connected' });
  });

  it('startDesktopVpn / stopDesktopVpn invoke', async () => {
    const invoke = vi.fn().mockResolvedValue('ok');
    vi.doMock('@tauri-apps/api/core', () => ({ invoke }));
    const d = await import('../src/lib/desktop');
    await expect(d.startDesktopVpn('cfg')).resolves.toBe('ok');
    await expect(d.stopDesktopVpn()).resolves.toBe('ok');
    expect(invoke).toHaveBeenCalled();
  });

  it('getSystemInfo via invoke', async () => {
    vi.doMock('@tauri-apps/api/core', () => ({
      invoke: vi.fn().mockResolvedValue(JSON.stringify({ os: 'windows', arch: 'x64' })),
    }));
    const d = await import('../src/lib/desktop');
    const r = await d.getSystemInfo();
    expect(r).toEqual({ os: 'windows', arch: 'x64' });
  });

  it('showNotification uses tauri plugin', async () => {
    const sendNotification = vi.fn();
    vi.doMock('@tauri-apps/api/core', () => ({ invoke: vi.fn() }));
    vi.doMock('@tauri-apps/plugin-notification', () => ({ sendNotification }));
    const d = await import('../src/lib/desktop');
    await d.showNotification('Hi', 'Body');
    expect(sendNotification).toHaveBeenCalledWith({ title: 'Hi', body: 'Body' });
  });

  it('showNotification falls back if plugin import fails', async () => {
    vi.doMock('@tauri-apps/api/core', () => ({ invoke: vi.fn() }));
    vi.doMock('@tauri-apps/plugin-notification', () => { throw new Error('no plugin'); });
    class N { static permission = 'granted'; constructor(_t: string, _o?: any) {} }
    vi.stubGlobal('Notification', N as any);
    const d = await import('../src/lib/desktop');
    await d.showNotification('A', 'B');
  });
});
