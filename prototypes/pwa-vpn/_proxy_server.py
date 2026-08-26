
import http.server, urllib.request, os, sys, socketserver

DIST = r"C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\prototypes\pwa-vpn\dist"
BACKEND = "http://127.0.0.1:8080"

class ProxyHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=DIST, **kwargs)

    def do_GET(self):
        if self.path.startswith('/api/') or self.path.startswith('/ws'):
            self.proxy('GET')
        else:
            super().do_GET()

    def do_POST(self):
        if self.path.startswith('/api/'):
            self.proxy('POST')
        else:
            self.send_error(405)

    def proxy(self, method):
        try:
            length = int(self.headers.get('Content-Length', 0))
            body = self.rfile.read(length) if length else None
            req = urllib.request.Request(BACKEND + self.path, data=body, method=method)
            for h in ['Content-Type', 'Authorization']:
                if self.headers.get(h):
                    req.add_header(h, self.headers[h])
            resp = urllib.request.urlopen(req, timeout=10)
            data = resp.read()
            self.send_response(resp.status)
            self.send_header('Content-Type', resp.headers.get('Content-Type', 'application/json'))
            self.send_header('Content-Length', str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except urllib.error.HTTPError as e:
            data = e.read()
            self.send_response(e.code)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except Exception as e:
            self.send_error(502, str(e))

    def log_message(self, *a): pass

class ThreadingServer(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True

os.chdir(DIST)
ThreadingServer(('0.0.0.0', 5173), ProxyHandler).serve_forever()
