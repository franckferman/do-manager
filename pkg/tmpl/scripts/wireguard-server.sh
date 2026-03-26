#!/bin/bash
# @name: wireguard-server
# @desc: WireGuard VPN server - generates server keys, configures wg0, enables IP forwarding
# @var: WGListenPort=51820 - WireGuard UDP listen port
# @var: WGServerNet=10.99.0.1/24 - WireGuard interface CIDR (server address)
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y wireguard-tools

# Generate server keypair
umask 077
wg genkey | tee /etc/wireguard/server.key | wg pubkey > /etc/wireguard/server.pub
SERVER_PRIVKEY=$(cat /etc/wireguard/server.key)
SERVER_PUBKEY=$(cat /etc/wireguard/server.pub)

# Enable IP forwarding
echo "net.ipv4.ip_forward = 1" > /etc/sysctl.d/99-wg-forward.conf
sysctl -p /etc/sysctl.d/99-wg-forward.conf

cat > /etc/wireguard/wg0.conf << WG
[Interface]
Address    = {{.WGServerNet}}
ListenPort = {{.WGListenPort}}
PrivateKey = ${SERVER_PRIVKEY}
# Add peers below using: wg set wg0 peer <PUBKEY> allowed-ips <IP>/32
# or append [Peer] blocks and run: wg syncconf wg0 <(wg-quick strip wg0)
WG

chmod 600 /etc/wireguard/wg0.conf
systemctl enable --now wg-quick@wg0

echo "[wireguard-server] interface wg0 up on port {{.WGListenPort}}"
echo "[wireguard-server] server address: {{.WGServerNet}}"
echo "[wireguard-server] SERVER PUBLIC KEY: ${SERVER_PUBKEY}"
echo "[wireguard-server] use this key in the wireguard-client template: --template-var WGServerPublicKey=${SERVER_PUBKEY}"
