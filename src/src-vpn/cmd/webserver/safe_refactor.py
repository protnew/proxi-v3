import os
import re

handlers = []

for filename in os.listdir('.'):
    if not filename.endswith('.go'): continue
    
    with open(filename, 'r', encoding='utf-8') as f:
        content = f.read()
    
    if filename != 'startup.go':
        # Find all func handle...
        content = re.sub(r'(?m)^func (handle[a-zA-Z0-9_]+)\(', r'func (s *Server) \1(', content)
        content = re.sub(r'(?m)^func initHub\(', r'func (s *Server) initHub(', content)
        
        # Replace db., hub., globalAuthService.
        content = re.sub(r'\bdb\.', 's.db.', content)
        content = re.sub(r'\bhub\.', 's.hub.', content)
        content = re.sub(r'\bglobalAuthService\.', 's.authService.', content)
        
        # And their nil checks
        content = re.sub(r'\bdb == nil', 's.db == nil', content)
        content = re.sub(r'\bhub == nil', 's.hub == nil', content)
        content = re.sub(r'\bhub != nil', 's.hub != nil', content)
        content = re.sub(r'\bglobalAuthService != nil', 's.authService != nil', content)

        # Remove global variables
        content = re.sub(r'(?m)^var db \*store\.Store\n', '', content)
        content = re.sub(r'(?m)^var globalAuthService \*auth\.AuthService\n', '', content)
        content = re.sub(r'(?m)^var hub \*chat\.ChatHub\n', '', content)
        
        # Fix initHub call in routing.go maybe?
        content = re.sub(r'\binitHub\(\)', 's.initHub()', content)
        
        # Fix the conflict in handleStreamCreate (s := streamMgr... -> stream := streamMgr...)
        if filename == 'routing.go':
            content = re.sub(r'\bs := streamMgr\.CreateStream', 'streamObj := streamMgr.CreateStream', content)
            # Need to replace s.ID, s.Name, s.Streamer but ONLY for the stream object
            # Since we replaced db. with s.db., s.ID -> streamObj.ID is safe enough for that specific function block
            # Let's just do it string-wise if it exists
            content = content.replace('s.ID', 'streamObj.ID')
            content = content.replace('s.Name', 'streamObj.Name')
            content = content.replace('s.Streamer', 'streamObj.Streamer')
            # Wait, s.db.Get... will not match s.ID, so it's safe.
            # Are there any other s.ID? No Server method uses s.ID since Server doesn't have ID.

        with open(filename, 'w', encoding='utf-8') as f:
            f.write(content)

# Now fix startup.go
with open('startup.go', 'r', encoding='utf-8') as f:
    startup = f.read()

# Replace db = store.NewStore -> db, err := store.NewStore
startup = re.sub(r'\bdb, err = store\.NewStore', r'db, err := store.NewStore', startup)
# globalAuthService = auth.NewAuthService -> authSvc := auth.NewAuthService
startup = re.sub(r'\bglobalAuthService = auth\.NewAuthService', r'authSvc := auth.NewAuthService', startup)

# Create Server instance right after db is available
# Actually, let's insert it inside func run() right after hub is initialized? But hub is now s.initHub()
# Let's just do srv := &Server{db: db, authService: authSvc, hub: chat.NewChatHub()} manually or via regex
# Find where globalAuthService is used in startup.go
startup = re.sub(r'\bglobalAuthService\b', 'srv.authService', startup)

# Replace all handleXXX( -> srv.handleXXX(
startup = re.sub(r'\b(handle[A-Z][a-zA-Z0-9_]*|initHub)\b', r'srv.\1', startup)

with open('startup.go', 'w', encoding='utf-8') as f:
    f.write(startup)
