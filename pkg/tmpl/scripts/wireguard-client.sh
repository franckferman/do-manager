#!/bin/bash
# @name: wireguard-client
# @desc: WireGuard VPN client (peer) - connects to a WireGuard server (C2 node or bastion)
# @var: WGServerEndpoint= - Public IP:port of the WireGuard server (e.g. 1.2.3.4:51820)
# @var: WGServerPublicKey= - Public key of the WireGuard server (get from server after deploy)
# @var: WGClientNet=10.99.0.2/24 - WireGuard interface CIDR for this client
# @var: WGAllowedIPs=10.99.0.0/24 - Routes to send through the tunnel (use 0.0.0.0/0 for full tunnel)
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y wireguard-tools

umask 077
wg genkey | tee /etc/wireguard/client.key | wg pubkey > /etc/wireguard/client.pub
CLIENT_PRIVKEY=$(cat /etc/wireguard/client.key)
CLIENT_PUBKEY=$(cat /etc/wireguard/client.pub)

cat > /etc/wireguard/wg0.conf << WG
[Interface]
Address    = {{.WGClientNet}}
PrivateKey = ${CLIENT_PRIVKEY}

[Peer]
PublicKey  = {{.WGServerPublicKey}}
Endpoint   = {{.WGServerEndpoint}}
AllowedIPs = {{.WGAllowedIPs}}
PersistentKeepalive = 25
WG

chmod 600 /etc/wireguard/wg0.conf
systemctl enable --now wg-quick@wg0

echo "[wireguard-client] tunnel up -> {{.WGServerEndpoint}}"
echo "[wireguard-client] client address: {{.WGClientNet}}"
echo "[wireguard-client] CLIENT PUBLIC KEY: ${CLIENT_PUBKEY}"
echo "[wireguard-client] add this peer on the server:"
echo "  wg set wg0 peer ${CLIENT_PUBKEY} allowed-ips $(echo {{.WGClientNet}} | cut -d'/' -f1)/32"
