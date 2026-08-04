/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach } from 'vitest';
import { searchMessages, searchInChat } from '../src/lib/search';
import { chats } from '../src/stores/messenger';

describe('search', () => {
  beforeEach(() => {
    chats.set([]);
  });

  it('searchMessages returns empty for empty query', () => {
    expect(searchMessages('')).toEqual([]);
    expect(searchMessages('   ')).toEqual([]);
  });

  it('searchInChat returns empty for empty query', () => {
    expect(searchInChat('chat1', '')).toEqual([]);
  });

  it('searchInChat returns empty for non-existent chat', () => {
    expect(searchInChat('nonexistent', 'test')).toEqual([]);
  });

  it('searchMessages finds matching messages', () => {
    chats.set([{
      id: 'c1', name: 'Test Chat', messages: [
        { id: 'm1', text: 'Hello world', from: 'a', timestamp: 1000 },
        { id: 'm2', text: 'World peace', from: 'b', timestamp: 2000 },
        { id: 'm3', text: 'Goodbye', from: 'a', timestamp: 3000 },
      ]
    } as any]);
    const results = searchMessages('world');
    expect(results.length).toBe(2);
    expect(results[0].chatId).toBe('c1');
  });

  it('searchInChat finds messages in specific chat', () => {
    chats.set([{
      id: 'c1', name: 'Test', messages: [
        { id: 'm1', text: 'Hello test', from: 'a', timestamp: 1000 },
        { id: 'm2', text: 'No match here', from: 'b', timestamp: 2000 },
      ]
    } as any]);
    const results = searchInChat('c1', 'test');
    expect(results.length).toBe(1);
    expect(results[0].text).toBe('Hello test');
  });

  it('searchMessages respects limit', () => {
    const msgs = Array.from({ length: 10 }, (_, i) => ({
      id: 'm' + i, text: 'find me ' + i, from: 'a', timestamp: i * 1000
    }));
    chats.set([{ id: 'c1', name: 'T', messages: msgs } as any]);
    const results = searchMessages('find', 3);
    expect(results.length).toBe(3);
  });

  it('searchMessages is case-insensitive', () => {
    chats.set([{
      id: 'c1', name: 'T', messages: [
        { id: 'm1', text: 'HELLO World', from: 'a', timestamp: 1000 },
      ]
    } as any]);
    expect(searchMessages('hello').length).toBe(1);
    expect(searchMessages('WORLD').length).toBe(1);
  });
});
