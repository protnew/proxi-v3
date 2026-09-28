import { describe, it, expect } from 'vitest';
import { BLEAdapter } from './ble_adapter.js';

describe('BLE / Wi-Fi Direct Adapter (Offline Mesh)', () => {
  it('should successfully scan and connect to a peer', async () => {
    const adapter = new BLEAdapter();
    const result = await adapter.startScanning();
    expect(result).toBe("DEVICE_FOUND");
    expect(adapter.isConnected).toBe(true);
  });

  it('should send a SimpleX queue message over BLE', async () => {
    const adapter = new BLEAdapter();
    await adapter.startScanning();
    const sent = await adapter.sendQueueMessage('Hello Offline World!');
    expect(sent).toBe(true);
  });
});
