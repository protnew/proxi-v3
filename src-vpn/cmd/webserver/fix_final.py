import os

# 1. Fix social.go
with open('social.go', 'r', encoding='utf-8') as f:
    social = f.read()

handlers = ['handleReactionAdd', 'handleReactionRemove', 'handleReactionGet', 'handleMarkRead', 'handleGetReadReceipts', 'handleProfileSave', 'handleProfileGet']
for h in handlers:
    social = social.replace(f"func {h}(", f"func (s *Server) {h}(")
    social = social.replace(f'mux.HandleFunc("/api/social/reaction/add", {h})', f'mux.HandleFunc("/api/social/reaction/add", s.{h})')
    social = social.replace(f'mux.HandleFunc("/api/social/reaction/remove", {h})', f'mux.HandleFunc("/api/social/reaction/remove", s.{h})')
    social = social.replace(f'mux.HandleFunc("/api/social/reaction/get", {h})', f'mux.HandleFunc("/api/social/reaction/get", s.{h})')
    social = social.replace(f'mux.HandleFunc("/api/social/messages/read", {h})', f'mux.HandleFunc("/api/social/messages/read", s.{h})')
    social = social.replace(f'mux.HandleFunc("/api/social/messages/receipts", {h})', f'mux.HandleFunc("/api/social/messages/receipts", s.{h})')
    social = social.replace(f'mux.HandleFunc("/api/social/profile", {h})', f'mux.HandleFunc("/api/social/profile", s.{h})')

# In social.go RegisterRoutes
for h in handlers:
    social = social.replace(f", {h})", f", s.{h})")

# db references in social.go
social = social.replace('db.SaveReaction', 's.db.SaveReaction')
social = social.replace('db.DeleteReaction', 's.db.DeleteReaction')
social = social.replace('db.GetMessageReactions', 's.db.GetMessageReactions')
social = social.replace('db.SaveMessageReadStatus', 's.db.SaveMessageReadStatus')
social = social.replace('db.GetMessageReadReceipts', 's.db.GetMessageReadReceipts')
social = social.replace('db.SaveUserProfile', 's.db.SaveUserProfile')
social = social.replace('db.GetUserProfile', 's.db.GetUserProfile')
social = social.replace('db.', 's.db.') # generic fallback, might duplicate but it's safe if db. is clean
social = social.replace('s.s.db.', 's.db.') # clean up

with open('social.go', 'w', encoding='utf-8') as f:
    f.write(social)

# 2. Fix routing.go
with open('routing.go', 'r', encoding='utf-8') as f:
    routing = f.read()

routing = routing.replace('func startDeadMansSwitchWorker(ctx context.Context, s *store.Store)', 'func startDeadMansSwitchWorker(ctx context.Context, s *Server)')
routing = routing.replace('s.GetDeadMansSwitches()', 's.db.GetDeadMansSwitches()')
routing = routing.replace('s.SaveMessage(msg)', 's.db.SaveMessage(msg)')
routing = routing.replace('s.db.db.GetDeadMansSwitches', 's.db.GetDeadMansSwitches')
routing = routing.replace('s.db.db.SaveMessage', 's.db.SaveMessage')

with open('routing.go', 'w', encoding='utf-8') as f:
    f.write(routing)

# 3. Fix startup.go
with open('startup.go', 'r', encoding='utf-8') as f:
    startup = f.read()

startup = startup.replace('go startDeadMansSwitchWorker(context.Background(), db)', 'go startDeadMansSwitchWorker(context.Background(), srv)')
startup = startup.replace('srv := &http.Server{', 'httpSrv := &http.Server{')
startup = startup.replace('srv.Serve(l)', 'httpSrv.Serve(l)')
startup = startup.replace('srv.Serve(lis)', 'httpSrv.Serve(lis)')
startup = startup.replace('srv.Shutdown(ctx)', 'httpSrv.Shutdown(ctx)')

with open('startup.go', 'w', encoding='utf-8') as f:
    f.write(startup)

