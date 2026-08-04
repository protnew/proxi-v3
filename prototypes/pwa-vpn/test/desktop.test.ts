/**
 * @vitest-environment jsdom
 */
import { describe, it, expect } from 'vitest';
import { getIsDesktop, getDesktopVpnStatus, getSystemInfo } from '../src/lib/desktop';

describe('desktop bridge', () => {
  it('getIsDesktop returns false in browser', () => {
    expect(getIsDesktop()).toBe(false);
  });

  it('getDesktopVpnStatus returns unavailable when not Tauri', async () => {
    const result = await getDesktopVpnStatus();
    expect(result.status).toBe('unavailable');
    expect(result.peers).toBe(0);
  });

  it('getSystemInfo returns platform when not Tauri', async () => {
    const result = await getSystemInfo();
    expect(result).toHaveProperty('os');
    expect(result).toHaveProperty('arch');
  });
});
