/**
 * api.ts — Re-export barrel for backward compatibility.
 *
 * Original monolithic file split into:
 *   - api-core.ts: request, identity, WebSocket, callbacks (~380 lines)
 *   - api-actions.ts: REST wrappers, direct function exports (~230 lines)
 *
 * All imports from './lib/api' continue to work unchanged.
 */

export * from './api-core';
export * from './api-actions';
