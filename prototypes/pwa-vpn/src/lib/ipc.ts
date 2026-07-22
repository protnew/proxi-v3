/**
 * T86/T85: IPC bridge — JSON-RPC dispatch between Tauri/Svelte and Go core.
 */
import { invoke } from '@tauri-apps/api/core';

type RpcHandler = (params: any) => Promise<any>;

class IpcDispatcher {
  private handlers = new Map<string, RpcHandler>();

  register(method: string, handler: RpcHandler) {
    this.handlers.set(method, handler);
  }

  async call(method: string, params?: any): Promise<any> {
    // Try Tauri invoke first (desktop)
    try {
      return await invoke(method, params || {});
    } catch {
      // Fallback to HTTP API (web/PWA)
      const handler = this.handlers.get(method);
      if (handler) return handler(params);
      throw new Error(`No handler for ${method}`);
    }
  }

  async dispatch(jsonRpc: { method: string; params?: any; id?: string }) {
    const result = await this.call(jsonRpc.method, jsonRpc.params);
    return { jsonrpc: '2.0', result, id: jsonRpc.id };
  }
}

export const ipc = new IpcDispatcher();

// Register core methods
ipc.register('get_online_users', async () => {
  const resp = await fetch('/api/v1/chat/online');
  return resp.json();
});

ipc.register('send_message', async (params: { to: string; content: string }) => {
  const resp = await fetch('/api/v1/chat/dm', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(params) });
  return resp.json();
});

ipc.register('get_content_list', async () => {
  const resp = await fetch('/api/v1/content/list');
  return resp.json();
});
