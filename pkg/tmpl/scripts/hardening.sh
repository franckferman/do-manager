#!/bin/bash
# @name: hardening
# @desc: Basic SSH hardening, fail2ban, and non-root operator user setup
# @var: OperatorUser=operator - Non-root user to create for day-to-day access
# @var: OperatorPubKey= - SSH public key for the operator user (leave empty to skip)
# @var: SSHPort=22 - SSH listening port (change to avoid automated scans)
# @var: MaxAuthTries=3 - Max SSH authentication attempts before disconnect
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y fail2ban ufw

# Create operator user
if ! id "{{.OperatorUser}}" &>/dev/null; then
    useradd -m -s /bin/bash {{.OperatorUser}}
    usermod -aG sudo {{.OperatorUser}}
fi

# Install SSH key if provided
PUBKEY="{{.OperatorPubKey}}"
if [[ -n "$PUBKEY" ]]; then
    OPDIR="/home/{{.OperatorUser}}/.ssh"
    mkdir -p "$OPDIR"
    echo "$PUBKEY" >> "$OPDIR/authorized_keys"
    chmod 700 "$OPDIR"
    chmod 600 "$OPDIR/authorized_keys"
    chown -R {{.OperatorUser}}:{{.OperatorUser}} "$OPDIR"
fi

# SSH hardening
cat > /etc/ssh/sshd_config.d/99-hardening.conf << SSHCFG
Port {{.SSHPort}}
PermitRootLogin prohibit-password
PasswordAuthentication no
MaxAuthTries {{.MaxAuthTries}}
X11Forwarding no
AllowAgentForwarding no
AllowTcpForwarding yes
PrintMotd no
SSHCFG

# fail2ban for SSH
cat > /etc/fail2ban/jail.d/sshd.conf << F2B
[sshd]
enabled  = true
port     = {{.SSHPort}}
maxretry = 3
bantime  = 1h
findtime = 10m
F2B

systemctl enable --now fail2ban
systemctl restart ssh

echo "[hardening] SSH port: {{.SSHPort}}"
echo "[hardening] operator user: {{.OperatorUser}}"
echo "[hardening] fail2ban: enabled"
if [[ "{{.SSHPort}}" != "22" ]]; then
    echo "[hardening] IMPORTANT: reconnect on port {{.SSHPort}}"
fi
