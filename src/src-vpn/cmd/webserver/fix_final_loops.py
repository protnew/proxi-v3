import os

with open('startup.go', 'r', encoding='utf-8') as f:
    startup = f.read()

startup = startup.replace('func deadMansSwitchLoop()', 'func (s *Server) deadMansSwitchLoop()')
startup = startup.replace('db.GetExpiredSwitches()', 's.db.GetExpiredSwitches()')
startup = startup.replace('db.MarkSwitchTriggered(', 's.db.MarkSwitchTriggered(')
startup = startup.replace('go deadMansSwitchLoop()', 'go srv.deadMansSwitchLoop()')

with open('startup.go', 'w', encoding='utf-8') as f:
    f.write(startup)

with open('ws_handlers.go', 'r', encoding='utf-8') as f:
    ws = f.read()

ws = ws.replace('chat.ServeWS(hub,', 'chat.ServeWS(s.hub,')

with open('ws_handlers.go', 'w', encoding='utf-8') as f:
    f.write(ws)
