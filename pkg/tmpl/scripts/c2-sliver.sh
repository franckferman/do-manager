#!/bin/bash
# @name: c2-sliver
# @desc: Sliver C2 framework - download latest release and run as systemd service
# @var: C2Port=31337 - Multiplayer teamserver port
# @var: HTTPSPort=443 - HTTPS listener port
# @var: OperatorName=operator - Initial operator name
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y curl unzip mingw-w64 binutils-mingw-w64

# Download latest Sliver server
SLIVER_VERSION=$(curl -s https://api.github.com/repos/BishopFox/sliver/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
curl -sL "https://github.com/BishopFox/sliver/releases/download/${SLIVER_VERSION}/sliver-server_linux" \
    -o /usr/local/bin/sliver-server
chmod +x /usr/local/bin/sliver-server

# Download sliver client too
curl -sL "https://github.com/BishopFox/sliver/releases/download/${SLIVER_VERSION}/sliver-client_linux" \
    -o /usr/local/bin/sliver-client
chmod +x /usr/local/bin/sliver-client

# Initial setup (generates certs, DB)
/usr/local/bin/sliver-server unpack --force

# Generate operator config
/usr/local/bin/sliver-server operator --name {{.OperatorName}} --lhost 0.0.0.0 --lport {{.C2Port}} \
    --save /root/{{.OperatorName}}.cfg

cat > /etc/systemd/system/sliver.service << 'UNIT'
[Unit]
Description=Sliver C2 Teamserver
After=network.target
[Service]
ExecStart=/usr/local/bin/sliver-server daemon -l 0.0.0.0 -p {{.C2Port}}
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now sliver
echo "[sliver] teamserver started on port {{.C2Port}}"
echo "[sliver] operator config saved to /root/{{.OperatorName}}.cfg"
echo "[sliver] copy it to your client: scp root@<ip>:/root/{{.OperatorName}}.cfg ~/.sliver/configs/"
