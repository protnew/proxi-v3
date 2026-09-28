/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach, vi } from 'vitest';

// Mock @tauri-apps/api/core
vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn().mockRejectedValue(new Error('not in tauri')),
}));

import { ipc } from '../src/lib/ipc';

describe('IPC dispatcher', () => {
  it('ipc is a singleton object', () => {
    expect(ipc).toBeDefined();
    expect(typeof ipc.call).toBe('function');
    expect(typeof ipc.register).toBe('function');
    expect(typeof ipc.dispatch).toBe('function');
  });

  it('register + call executes handler', async () => {
    ipc.register('test_method', async (params) => {
      return { echo: params };
    });
    const result = await ipc.call('test_method', { value: 42 });
    expect(result).toEqual({ echo: { value: 42 } });
  });

  it('dispatch returns JSON-RPC 2.0 response', async () => {
    ipc.register('dispatch_test', async () => 'ok');
    const result = await ipc.dispatch({ method: 'dispatch_test', id: '1' });
    expect(result.jsonrpc).toBe('2.0');
    expect(result.result).toBe('ok');
    expect(result.id).toBe('1');
  });
});
