#!/bin/bash
# @name: c2-havoc
# @desc: Havoc C2 framework - build from source and run teamserver as systemd service
# @var: C2Port=443 - Teamserver and HTTPS listener port
# @var: C2Host= - Public hostname or IP used in listener config
# @var: TeamserverPassword=changeme443 - Operator console password
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y git build-essential cmake libssl-dev libz-dev \
    libfontconfig1-dev python3-dev python3-pip \
    mingw-w64 nasm golang-go

cd /opt
git clone https://github.com/HavocFramework/Havoc.git
cd Havoc
make ts-build

cat > /opt/havoc-profile.yaotl << 'PROFILE'
Teamserver {
    Host = "0.0.0.0"
    Port = {{.C2Port}}
    Build {
        Compiler64 = "/usr/bin/x86_64-w64-mingw32-g++"
        Compiler86 = "/usr/bin/i686-w64-mingw32-g++"
        Nasm       = "/usr/bin/nasm"
    }
}
Operators {
    operator "operator" {
        Password = "{{.TeamserverPassword}}"
    }
}
Listeners {
    Http {
        Name         = "https-listener"
        Hosts        = ["{{.C2Host}}"]
        HostBind     = "0.0.0.0"
        HostRotation = "round-robin"
        Port         = {{.C2Port}}
        Secure       = true
    }
}
PROFILE

cat > /etc/systemd/system/havoc.service << 'UNIT'
[Unit]
Description=Havoc C2 Teamserver
After=network.target
[Service]
WorkingDirectory=/opt/Havoc
ExecStart=/opt/Havoc/havoc server --profile /opt/havoc-profile.yaotl
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now havoc
echo "[havoc] teamserver started on port {{.C2Port}}"
echo "[havoc] connect with Havoc client -> password: {{.TeamserverPassword}}"
