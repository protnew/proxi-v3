import os
import re

# files.go
with open('files.go', 'r', encoding='utf-8') as f:
    files = f.read()

files = re.sub(r'\bhandleFileList\(', 's.handleFileList(', files)
files = re.sub(r'\bhandleFileDownload\(', 's.handleFileDownload(', files)
files = re.sub(r'\bhandleFileDelete\(', 's.handleFileDelete(', files)

with open('files.go', 'w', encoding='utf-8') as f:
    f.write(files)

# media.go
with open('media.go', 'r', encoding='utf-8') as f:
    media = f.read()

media = re.sub(r'\bdb\.', 's.db.', media)

with open('media.go', 'w', encoding='utf-8') as f:
    f.write(media)

# routing.go
with open('routing.go', 'r', encoding='utf-8') as f:
    routing = f.read()

routing = routing.replace('dmstreamObj.ID', 'dms.ID')
routing = routing.replace('dmstreamObj.Name', 'dms.Name')

with open('routing.go', 'w', encoding='utf-8') as f:
    f.write(routing)

# ws_handlers.go
with open('ws_handlers.go', 'r', encoding='utf-8') as f:
    ws = f.read()

ws = re.sub(r'\binitHub\(\)', 's.initHub()', ws)

with open('ws_handlers.go', 'w', encoding='utf-8') as f:
    f.write(ws)

# startup.go: undefined db
with open('startup.go', 'r', encoding='utf-8') as f:
    startup = f.read()

# I need to make sure db is passed correctly.
# If startup.go has undefined db, it's because I changed db, err = store.NewStore to db, err := store.NewStore inside a block or it doesn't exist globally anymore.
# But wait, startup.go uses db later. Since we did db, err := ... in run(), db is available in run().
# Let's check where db is undefined in startup.go.
