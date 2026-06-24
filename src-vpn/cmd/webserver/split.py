import os
import re

with open("main.go", "r", encoding="utf-8") as f:
    content = f.read()

# Extract package and imports
header_match = re.search(r'(package main.*?import \s*\([^)]+\))', content, re.DOTALL)
header = header_match.group(1) if header_match else "package main\n\nimport (\n)\n"

parts = re.split(r'\n(?=func |type |var |const )', content)

auth_funcs = ["authMiddleware", "handleLogin", "handleRegister", "validateToken", "JWT"]
ws_funcs = ["nhooyrWSConn", "handleNostrWS", "websocket", "hub", "Client"]
startup_funcs = ["run", "main", "scheduledMessagesLoop", "deadMansSwitchLoop", "autoConnectPeers"]
helper_funcs = ["rateLimit", "cors", "securityHeaders", "setCache", "writeJSON", "writeError", "truncate", "containsPathTraversal", "secureJoin"]

files = {
    "auth.go": [],
    "websocket.go": [],
    "startup.go": [],
    "helpers.go": [],
    "routing.go": [],
}

for i, part in enumerate(parts):
    if i == 0: continue # Header
    
    part_name_match = re.match(r'(func|type|var|const)\s+([A-Za-z0-9_]+)', part)
    if not part_name_match:
        files["routing.go"].append(part)
        continue
        
    name = part_name_match.group(2)
    part_text = part.strip()
    
    assigned = False
    for kw in auth_funcs:
        if kw.lower() in name.lower() or name.startswith("handleLogin"):
            files["auth.go"].append(part_text)
            assigned = True
            break
    if not assigned:
        for kw in ws_funcs:
            if kw.lower() in name.lower() or "WS" in name:
                files["websocket.go"].append(part_text)
                assigned = True
                break
    if not assigned:
        for kw in startup_funcs:
            if kw.lower() == name.lower() or name in startup_funcs:
                files["startup.go"].append(part_text)
                assigned = True
                break
    if not assigned:
        for kw in helper_funcs:
            if kw.lower() in name.lower() or name in helper_funcs:
                files["helpers.go"].append(part_text)
                assigned = True
                break
    if not assigned:
        files["routing.go"].append(part_text)

for fname, blocks in files.items():
    if blocks:
        with open(fname, "w", encoding="utf-8") as f:
            f.write(header + "\n\n" + "\n\n".join(blocks) + "\n")

os.rename("main.go", "main.go.bak")
