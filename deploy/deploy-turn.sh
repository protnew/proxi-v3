#!/bin/bash
# TURN server deployment for Indestructible VPN
# Architecture table 05_Relay_Architecture
# VPS: Any $5/mo (Hetzner CX21, Vultr 1GB, DigitalOcean $5 droplet)
#
# After deploy, set env vars on webserver:
#   TURN_URL=turn:YOUR_VPS_IP:3478
#   TURN_USER=proxi
#   TURN_PASS=changeme_strong_password_here

set -e

echo "=== Installing coturn ==="
apt-get update && apt-get install -y coturn

echo "=== Configuring ==="
sed -i 's|#TURNSERVER_ENABLED=1|TURNSERVER_ENABLED=1|' /etc/default/coturn

# Get server public IP
SERVER_IP=$(curl -s http://api.ipify.org)
echo "Server IP: $SERVER_IP"

cat > /etc/turnserver.conf <<EOF
listening-port=3478
tls-listening-port=5349
listening-ip=0.0.0.0
external-ip=$SERVER_IP
user=proxi:$(openssl rand -base64 24)
realm=indestructible.vpn
server-name=indestructible-turn
fingerprint
lt-cred-mech
total-quota=100
bps-capacity=0
log-file=/var/log/turnserver.log
simple-log
no-cli
no-tcp-relay
EOF

echo "=== Generated credentials ==="
grep "^user=" /etc/turnserver.conf

echo "=== Restarting coturn ==="
systemctl restart coturn
systemctl enable coturn

echo "=== Verify ==="
sleep 2
systemctl status coturn --no-pager

echo ""
echo "=== Configure your webserver with these env vars: ==="
PASS=$(grep "^user=" /etc/turnserver.conf | sed 's/user=proxi://')
echo "TURN_URL=turn:$SERVER_IP:3478"
echo "TURN_USER=proxi"
echo "TURN_PASS=$PASS"
