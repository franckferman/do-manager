#!/bin/bash
# @name: redirector-nginx
# @desc: nginx HTTPS reverse proxy forwarding C2 beacon traffic to a backend C2 server
# @var: C2BackendHost=10.20.0.2 - Private IP or hostname of the C2 node (VPC private IP or WireGuard)
# @var: C2BackendPort=443 - C2 backend port
# @var: ListenPort=443 - Public listening port
# @var: Domain= - Domain name for TLS (leave empty to use self-signed cert)
# @var: C2URIPath=/updates - URI path that nginx forwards to C2 (other paths get a 301 to google.com)
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y nginx openssl

# Self-signed fallback cert
CERT_DIR=/etc/nginx/ssl
mkdir -p "$CERT_DIR"
if [[ -z "{{.Domain}}" ]]; then
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout "$CERT_DIR/server.key" \
        -out "$CERT_DIR/server.crt" \
        -subj "/CN=localhost"
    CERT="$CERT_DIR/server.crt"
    KEY="$CERT_DIR/server.key"
else
    apt-get install -y certbot python3-certbot-nginx
    certbot certonly --standalone -d {{.Domain}} --non-interactive --agree-tos -m admin@{{.Domain}}
    CERT="/etc/letsencrypt/live/{{.Domain}}/fullchain.pem"
    KEY="/etc/letsencrypt/live/{{.Domain}}/privkey.pem"
fi

cat > /etc/nginx/sites-available/redirector << NGINXCFG
server {
    listen {{.ListenPort}} ssl;
    ssl_certificate     $CERT;
    ssl_certificate_key $KEY;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    # Forward matching URI path to C2 backend
    location {{.C2URIPath}} {
        proxy_pass          https://{{.C2BackendHost}}:{{.C2BackendPort}};
        proxy_ssl_verify    off;
        proxy_set_header    Host \$host;
        proxy_set_header    X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header    X-Real-IP \$remote_addr;
    }

    # All other traffic gets a redirect - looks like a normal web server
    location / {
        return 301 https://www.google.com;
    }
}

server {
    listen 80;
    return 301 https://\$host\$request_uri;
}
NGINXCFG

ln -sf /etc/nginx/sites-available/redirector /etc/nginx/sites-enabled/redirector
rm -f /etc/nginx/sites-enabled/default

nginx -t
systemctl enable --now nginx
echo "[redirector-nginx] forwarding {{.C2URIPath}} -> https://{{.C2BackendHost}}:{{.C2BackendPort}}"
