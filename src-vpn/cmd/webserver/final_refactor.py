import os
import re

def process_file(filename):
    with open(filename, 'r', encoding='utf-8') as f:
        content = f.read()

    # Skip files that don't need changes
    if 'func handle' not in content and 'var db ' not in content and 'db.' not in content:
        if filename != 'startup.go':
            return

    # Replace handle declarations
    if filename != 'startup.go':
        content = re.sub(r'(?m)^func (handle[a-zA-Z0-9_]+)\(', r'func (s *Server) \1(', content)
        content = re.sub(r'(?m)^func initHub\(', r'func (s *Server) initHub(', content)

        # String replacements for variables inside methods
        content = re.sub(r'\bdb\.', 's.db.', content)
        content = re.sub(r'\bhub\.', 's.hub.', content)
        content = re.sub(r'\bglobalAuthService\.', 's.authService.', content)
        
        content = re.sub(r'\bdb == nil', 's.db == nil', content)
        content = re.sub(r'\bhub == nil', 's.hub == nil', content)
        content = re.sub(r'\bhub != nil', 's.hub != nil', content)
        content = re.sub(r'\bglobalAuthService != nil', 's.authService != nil', content)

        # Remove global variables exactly
        content = content.replace('var db *store.Store\n', '')
        content = content.replace('var globalAuthService *auth.AuthService\n', '')
        content = content.replace('var hub *chat.ChatHub\n', '')

        # Fix stream object name collision in routing.go
        if filename == 'routing.go':
            # In func (s *Server) handleStreamCreate, we have: s := streamMgr.CreateStream
            content = content.replace('s := streamMgr.CreateStream', 'streamObj := streamMgr.CreateStream')
            content = content.replace('s.ID', 'streamObj.ID')
            content = content.replace('s.Name', 'streamObj.Name')
            content = content.replace('s.Streamer', 'streamObj.Streamer')
            
            # fix s.db.Get... replaced in dmstreamObj? No, dms.ID does not contain s.ID as an exact word.
            # wait, 's.ID' replacement without word boundaries caused dmstreamObj.ID!
            # So I will use regex for stream object fields:
            content = re.sub(r'\bstreamObj\.ID\b', 'streamObj.ID', content) # just to be sure we only replace exact word s.ID
            # Actually, I should just fix dmstreamObj -> dms
            content = content.replace('dmstreamObj.ID', 'dms.ID')
            content = content.replace('dmstreamObj.Name', 'dms.Name')

        # fix handleFederationSync if it has store.Store (wait, it didn't! It was my bad regex earlier)

        # fix initHub calls
        content = re.sub(r'\binitHub\(\)', 's.initHub()', content)
        # But wait, in ws_handlers.go we defined unc (s *Server) initHub(). The regex \binitHub\(\) matches the definition!
        # Because unc (s *Server) initHub() contains initHub().
        # So it becomes unc (s *Server) s.initHub().
        # I must fix this!
        content = content.replace('func (s *Server) s.initHub()', 'func (s *Server) initHub()')

    # startup.go
    if filename == 'startup.go':
        content = re.sub(r'\bdb, err = store\.NewStore', r'db, err := store.NewStore', content)
        content = re.sub(r'\bglobalAuthService = auth\.NewAuthService', r'authSvc := auth.NewAuthService', content)
        content = re.sub(r'\bglobalAuthService\b', 'srv.authService', content)
        
        # We need to pass srv.handleXXX
        content = re.sub(r'\b(handle[A-Z][a-zA-Z0-9_]*|initHub)\b', r'srv.\1', content)

    with open(filename, 'w', encoding='utf-8') as f:
        f.write(content)

for filename in os.listdir('.'):
    if filename.endswith('.go'):
        process_file(filename)
