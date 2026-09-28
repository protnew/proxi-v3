/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { isE2EEnabled, setE2EEnabled, loadE2EPref } from '../src/lib/api-extended';

describe('api-extended', () => {
  beforeEach(() => localStorage.clear());

  it('E2E enabled by default', () => {
    expect(isE2EEnabled()).toBe(true);
  });

  it('setE2EEnabled(false) persists', () => {
    setE2EEnabled(false);
    expect(isE2EEnabled()).toBe(false);
    expect(localStorage.getItem('proxi_e2e')).toBe('0');
  });

  it('setE2EEnabled(true) persists', () => {
    setE2EEnabled(false);
    setE2EEnabled(true);
    expect(isE2EEnabled()).toBe(true);
    expect(localStorage.getItem('proxi_e2e')).toBe('1');
  });

  it('loadE2EPref reads from localStorage', () => {
    localStorage.setItem('proxi_e2e', '0');
    expect(loadE2EPref()).toBe(false);
  });

  it('loadE2EPref defaults true when no stored value', () => {
    expect(loadE2EPref()).toBe(true);
  });
});
