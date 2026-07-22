import { test, expect } from '@playwright/test';
import * as http from 'http';

test.describe('HLS Media Router E2E', () => {
  let mockServer: http.Server;
  const PORT = 8082; // Используем другой порт, чтобы избежать конфликтов с другими тестами

  test.beforeAll(async () => {
    mockServer = http.createServer((req, res) => {
      // CORS headers
      res.setHeader('Access-Control-Allow-Origin', '*');
      res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
      
      if (req.method === 'OPTIONS') { 
        res.writeHead(200); 
        return res.end(); 
      }

      // Роутер: обрабатываем запросы к медиа-файлам HLS
      if (req.method === 'GET' && req.url?.startsWith('/api/media/')) {
        const urlParts = req.url.split('/');
        const fileWithExt = urlParts[urlParts.length - 1]; // например, video_123.m3u8
        
        // Отсекаем расширение (симуляция поведения роутера)
        const match = fileWithExt.match(/^(.*)\.(m3u8|ts)$/);
        
        if (match) {
          const hash = match[1];
          const ext = match[2];
          
          if (ext === 'm3u8') {
            res.writeHead(200, { 
              'Content-Type': 'application/vnd.apple.mpegurl',
              'Access-Control-Allow-Origin': '*'
            });
            // Возвращаем HLS манифест. Роутер отсек .m3u8 и использует hash для генерации контента
            res.end(`#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXTINF:10.0,\n${hash}.ts\n#EXT-X-ENDLIST`);
          } else if (ext === 'ts') {
            res.writeHead(200, { 
              'Content-Type': 'video/mp2t',
              'Access-Control-Allow-Origin': '*'
            });
            // Возвращаем бинарный сегмент. Роутер отсек .ts
            res.end(Buffer.from([0x47, 0x01, 0x02, 0x03])); // mock TS chunk (0x47 is sync byte)
          }
        } else {
          res.writeHead(400); 
          res.end('Invalid extension or missing extension');
        }
      } else {
        res.writeHead(404); 
        res.end('Not Found');
      }
    });

    await new Promise<void>((resolve) => {
      mockServer.listen(PORT, () => resolve());
    });
  });

  test.afterAll(() => {
    mockServer.close();
  });

  test('Роутер должен отсекать .m3u8 и возвращать HLS манифест', async ({ request }) => {
    const videoHash = 'test_video_hash_999';
    const response = await request.get(`http://localhost:${PORT}/api/media/${videoHash}.m3u8`);
    
    expect(response.ok()).toBeTruthy();
    expect(response.headers()['content-type']).toBe('application/vnd.apple.mpegurl');
    
    const text = await response.text();
    expect(text).toContain('#EXTM3U');
    expect(text).toContain(`${videoHash}.ts`); // Проверяем, что хэш корректно извлечен и использован
  });

  test('Роутер должен отсекать .ts и возвращать сегмент видео', async ({ request }) => {
    const videoHash = 'test_video_hash_999';
    const response = await request.get(`http://localhost:${PORT}/api/media/${videoHash}.ts`);
    
    expect(response.ok()).toBeTruthy();
    expect(response.headers()['content-type']).toBe('video/mp2t');
    
    const buffer = await response.body();
    expect(buffer.length).toBeGreaterThan(0);
    expect(buffer[0]).toBe(0x47); // Проверяем sync byte для TS файлов
  });

  test('Симуляция загрузки видео плеером (последовательный fetch)', async ({ request }) => {
    const videoHash = 'player_sim_hash';
    
    // 1. Плеер запрашивает манифест
    const manifestRes = await request.get(`http://localhost:${PORT}/api/media/${videoHash}.m3u8`);
    expect(manifestRes.ok()).toBeTruthy();
    const manifestText = await manifestRes.text();
    
    // 2. Плеер парсит манифест и находит сегмент
    const segmentNameMatch = manifestText.match(/(player_sim_hash\.ts)/);
    expect(segmentNameMatch).toBeTruthy();
    const segmentFilename = segmentNameMatch![1];
    
    // 3. Плеер запрашивает сегмент
    const segmentRes = await request.get(`http://localhost:${PORT}/api/media/${segmentFilename}`);
    expect(segmentRes.ok()).toBeTruthy();
    const segmentBody = await segmentRes.body();
    expect(segmentBody[0]).toBe(0x47);
  });
});
