#!/bin/bash
# @name: crackbox
# @desc: GPU password-cracking box - hashcat + hcxtools + wordlists/rules staged, ready for WPA (22000) and more
# @var: WordlistURL= - Optional wordlist to download (e.g. a weakpass/SecLists .gz URL); rockyou is always staged
# @var: RulesURL=https://raw.githubusercontent.com/NotSoSecure/password_cracking_rules/master/OneRuleToRuleThemAll.rule - Hashcat rules file
# @var: CrackDir=/opt/crack - Base directory for wordlists, rules and captures
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y hashcat hcxtools p7zip-full curl wget git ca-certificates

# GPU sanity — AI/ML images (gpu-h100x1-base) ship NVIDIA drivers + CUDA. On a plain
# Ubuntu image you must install drivers yourself; warn loudly if the GPU isn't visible.
if command -v nvidia-smi >/dev/null 2>&1; then
    nvidia-smi || true
else
    echo "[!] nvidia-smi not found — no GPU drivers. Use the AI/ML image (gpu-h100x1-base) or install drivers." >&2
fi
hashcat -I || true      # list compute backends (should show the GPU)

mkdir -p "{{.CrackDir}}/wordlists" "{{.CrackDir}}/rules" "{{.CrackDir}}/captures"
cd "{{.CrackDir}}/wordlists"

# rockyou — the baseline list, always staged
if [ ! -f rockyou.txt ]; then
    wget -q https://github.com/brannondorsey/naive-hashcat/releases/download/data/rockyou.txt -O rockyou.txt \
        || echo "[!] rockyou download failed — stage it manually" >&2
fi

# Optional bigger wordlist (operator-supplied URL); auto-decompress .gz
if [ -n "{{.WordlistURL}}" ]; then
    fname="$(basename '{{.WordlistURL}}')"
    wget -q "{{.WordlistURL}}" -O "$fname" && case "$fname" in *.gz) gunzip -f "$fname";; esac \
        || echo "[!] wordlist download failed: {{.WordlistURL}}" >&2
fi

# Rules (OneRuleToRuleThemAll by default — high hit-rate)
if [ -n "{{.RulesURL}}" ]; then
    wget -q "{{.RulesURL}}" -O "{{.CrackDir}}/rules/rules.rule" \
        || echo "[!] rules download failed" >&2
fi

echo "[+] crackbox ready. Wordlists: {{.CrackDir}}/wordlists  Rules: {{.CrackDir}}/rules/rules.rule"
echo "    crack WPA:  hashcat -m 22000 <hash.22000> {{.CrackDir}}/wordlists/rockyou.txt -r {{.CrackDir}}/rules/rules.rule"
echo "    drop captures in {{.CrackDir}}/captures, convert with: hcxpcapngtool -o hash.22000 *.pcapng"
