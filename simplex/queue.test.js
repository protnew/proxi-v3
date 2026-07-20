import { describe, it, expect } from 'vitest';
import { createSMPQueue, sendMessageToQueue } from './queue.js';

describe('SimpleX SMP Queue API', () => {
  it('should create an anonymous queue and return an endpoint URL', async () => {
    // Mock the SMP server interaction
    const endpoint = await createSMPQueue();
    expect(endpoint).toContain('smp://');
    expect(endpoint.length).toBeGreaterThan(20);
  });

  it('should format a message correctly for the queue', async () => {
    const endpoint = 'smp://mock-server:5223/queue-id';
    const result = await sendMessageToQueue(endpoint, 'Hello Alice');
    expect(result).toBe(true);
  });
});
