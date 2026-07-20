import os

with open('startup.go', 'r', encoding='utf-8') as f:
    startup = f.read()

startup = startup.replace('func autoConnectPeers()', 'func (s *Server) autoConnectPeers()')
startup = startup.replace('func scheduledMessagesLoop()', 'func (s *Server) scheduledMessagesLoop()')
startup = startup.replace('go autoConnectPeers()', 'go srv.autoConnectPeers()')
startup = startup.replace('go scheduledMessagesLoop()', 'go srv.scheduledMessagesLoop()')

# Fix inside autoConnectPeers
startup = startup.replace('db.GetPeers()', 's.db.GetPeers()')

# Fix inside scheduledMessagesLoop
startup = startup.replace('db == nil || hub == nil', 's.db == nil || s.hub == nil')
startup = startup.replace('db.GetPendingScheduled()', 's.db.GetPendingScheduled()')
startup = startup.replace('hub.Broadcast(', 's.hub.Broadcast(')
startup = startup.replace('hub.SendTo(', 's.hub.SendTo(')
startup = startup.replace('db.MarkScheduledSent(', 's.db.MarkScheduledSent(')

with open('startup.go', 'w', encoding='utf-8') as f:
    f.write(startup)

