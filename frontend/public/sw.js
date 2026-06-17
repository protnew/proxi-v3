const CACHE_NAME = 'proxi-cache-v1';

self.addEventListener('install', (event) => {
    self.skipWaiting();
});

self.addEventListener('activate', (event) => {
    event.waitUntil(self.clients.claim());
});

const PENDING_REQUESTS = new Map();

self.addEventListener('message', (event) => {
    if (event.data && event.data.type === 'vpn_response') {
        const { requestId, status, statusText, headers, body, error } = event.data;
        const resolver = PENDING_REQUESTS.get(requestId);
        if (resolver) {
            PENDING_REQUESTS.delete(requestId);
            if (error) {
                resolver.resolve(new Response(error, { status: 500 }));
            } else {
                // Decode base64 body if present
                let responseBody = null;
                if (body) {
                    const base64Data = body.split(',')[1];
                    const byteCharacters = atob(base64Data);
                    const byteNumbers = new Array(byteCharacters.length);
                    for (let i = 0; i < byteCharacters.length; i++) {
                        byteNumbers[i] = byteCharacters.charCodeAt(i);
                    }
                    responseBody = new Uint8Array(byteNumbers);
                }

                const responseInit = {
                    status: status,
                    statusText: statusText,
                    headers: new Headers(headers)
                };
                resolver.resolve(new Response(responseBody, responseInit));
            }
        }
    }
});

self.addEventListener('fetch', (event) => {
    const url = new URL(event.request.url);

    // Only intercept requests matching a specific pattern (e.g. /vpn-proxy/...)
    if (url.pathname.startsWith('/vpn-proxy/')) {
        const targetUrl = url.pathname.replace('/vpn-proxy/', '') + url.search;
        
        event.respondWith(new Promise(async (resolve, reject) => {
            const requestId = Math.random().toString(36).substring(2);
            PENDING_REQUESTS.set(requestId, { resolve, reject });

            // Read request body if needed
            let bodyText = null;
            if (event.request.method !== 'GET' && event.request.method !== 'HEAD') {
                bodyText = await event.request.text();
            }

            const headers = {};
            event.request.headers.forEach((val, key) => { headers[key] = val; });

            // Send to main thread
            const clients = await self.clients.matchAll();
            if (clients && clients.length > 0) {
                clients[0].postMessage({
                    type: 'vpn_request',
                    requestId,
                    url: targetUrl,
                    method: event.request.method,
                    headers: headers,
                    body: bodyText
                });
            } else {
                resolve(new Response("No clients available for proxy", {status: 503}));
            }
        }));
    }
});
