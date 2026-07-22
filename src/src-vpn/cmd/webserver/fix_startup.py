import os
import re

with open('startup.go', 'r', encoding='utf-8') as f:
    startup = f.read()

# Add srv creation
startup = startup.replace('	log.Println("✅ Database initialized")', '	log.Println("✅ Database initialized")\n\n\tsrv := &Server{\n\t\tdb: db,\n\t}')

# Add srv.authService assignment
startup = startup.replace('authSvc := auth.NewAuthService(secret)', 'authSvc := auth.NewAuthService(secret)\n\tsrv.authService = authSvc')

with open('startup.go', 'w', encoding='utf-8') as f:
    f.write(startup)

