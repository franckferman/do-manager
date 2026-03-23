#!/bin/bash
# @name: gophish
# @desc: GoPhish phishing framework - download latest release and run as systemd service
# @var: AdminListenPort=3333 - GoPhish admin panel port (bind to 127.0.0.1 only)
# @var: PhishListenPort=8080 - Phishing HTTP listener port
# @var: PhishTLSPort=443 - Phishing HTTPS listener port
# @var: AdminPassword=gophish - Initial admin password (change on first login)
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y curl unzip jq

# Download latest GoPhish
GP_VERSION=$(curl -s https://api.github.com/repos/gophish/gophish/releases/latest | jq -r '.tag_name')
curl -sL "https://github.com/gophish/gophish/releases/download/${GP_VERSION}/gophish-${GP_VERSION}-linux-64bit.zip" \
    -o /tmp/gophish.zip
unzip -q /tmp/gophish.zip -d /opt/gophish
chmod +x /opt/gophish/gophish

# Config
cat > /opt/gophish/config.json << 'GPCFG'
{
    "admin_server": {
        "listen_url": "127.0.0.1:{{.AdminListenPort}}",
        "use_tls": true,
        "cert_path": "gophish_admin.crt",
        "key_path": "gophish_admin.key"
    },
    "phish_server": {
        "listen_url": "0.0.0.0:{{.PhishListenPort}}",
        "use_tls": false
    },
    "db_name": "sqlite3",
    "db_path": "gophish.db",
    "migrations_prefix": "db/db_",
    "contact_address": "",
    "logging": {
        "filename": "",
        "level": ""
    }
}
GPCFG

cat > /etc/systemd/system/gophish.service << 'UNIT'
[Unit]
Description=GoPhish Phishing Framework
After=network.target
[Service]
WorkingDirectory=/opt/gophish
ExecStart=/opt/gophish/gophish
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now gophish
echo "[gophish] admin panel on 127.0.0.1:{{.AdminListenPort}} (SSH tunnel to access)"
echo "[gophish] phishing listener on 0.0.0.0:{{.PhishListenPort}}"
echo "[gophish] access admin via: ssh -L {{.AdminListenPort}}:127.0.0.1:{{.AdminListenPort}} root@<ip>"
