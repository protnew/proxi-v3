import os
import re

with open("store.go", "r", encoding="utf-8") as f:
    content = f.read()

# Extract package and imports
header_match = re.search(r'(package store.*?import \s*\([^)]+\))', content, re.DOTALL)
header = header_match.group(1) if header_match else "package store\n\nimport (\n)\n"

parts = re.split(r'\n(?=func |type |var |const )', content)

users_funcs = ["User", "Profile", "identity", "prekey", "Group", "Referral", "Role", "Member"]
messages_funcs = ["Message", "Thread", "Reaction", "ReactionCount", "Channel", "ReadReceipt", "Scheduled", "SaveMessage", "GetMessage"]
streams_funcs = ["Stream", "File", "Media", "Upload", "GetStream", "Chunk"]
analytics_funcs = ["Analytics", "Report", "Metric", "Vote", "Donation", "Ban", "Audit"]

files = {
    "users.go": [],
    "messages.go": [],
    "streams.go": [],
    "analytics.go": [],
    "store.go": [], # Core Store struct and Open DB funcs
}

for i, part in enumerate(parts):
    if i == 0: continue # Header
    
    part_name_match = re.match(r'(func|type|var|const)\s+(?:\([^*]*\*?[^)]+\)\s+)?([A-Za-z0-9_]+)', part)
    if not part_name_match:
        files["store.go"].append(part)
        continue
        
    name = part_name_match.group(2)
    part_text = part.strip()
    
    assigned = False
    
    if name == "Store" or name == "NewStore" or name == "Close" or name == "DB" or name.startswith("init"):
        files["store.go"].append(part_text)
        continue

    for kw in analytics_funcs:
        if kw.lower() in name.lower() or name.startswith("Get" + kw):
            files["analytics.go"].append(part_text)
            assigned = True
            break
    if not assigned:
        for kw in streams_funcs:
            if kw.lower() in name.lower() or name.startswith("Get" + kw):
                files["streams.go"].append(part_text)
                assigned = True
                break
    if not assigned:
        for kw in users_funcs:
            if kw.lower() in name.lower() or name.startswith("Get" + kw):
                files["users.go"].append(part_text)
                assigned = True
                break
    if not assigned:
        for kw in messages_funcs:
            if kw.lower() in name.lower() or name.startswith("Get" + kw):
                files["messages.go"].append(part_text)
                assigned = True
                break
    if not assigned:
        files["store.go"].append(part_text)

for fname, blocks in files.items():
    if blocks:
        with open(fname, "w", encoding="utf-8") as f:
            f.write(header + "\n\n" + "\n\n".join(blocks) + "\n")

os.rename("store.go", "store.go.bak")
