#!/bin/bash
# @name: crackbox
# @desc: GPU password-cracking box - hashcat + hcxtools + hashcat-utils, multiple rule sets, cupp/cewl/duplicut, wordlists staged
# @var: WordlistURL= - Optional extra wordlist to fetch (weakpass/SecLists .gz URL); rockyou is always staged
# @var: CrackDir=/opt/crack - Base directory for wordlists, rules, tools and captures
set -uo pipefail
export DEBIAN_FRONTEND=noninteractive
warn() { echo "[!] $*" >&2; }

apt-get update -y
apt-get install -y hashcat hcxtools hashcat-utils cewl p7zip-full \
    curl wget git build-essential ca-certificates python3 || warn "apt install had failures"

# GPU sanity — AI/ML images (gpu-h100x1-base) ship NVIDIA drivers + CUDA. On a plain
# Ubuntu image drivers are absent; hashcat then has no GPU backend.
if command -v nvidia-smi >/dev/null 2>&1; then nvidia-smi || true
else warn "nvidia-smi missing — no GPU drivers (use the AI/ML image gpu-h100x1-base)"; fi
hashcat -I || true

mkdir -p "{{.CrackDir}}"/{wordlists,rules,tools,captures}

# --- tools ------------------------------------------------------------------
cd "{{.CrackDir}}/tools"
git clone --depth 1 https://github.com/Mebus/cupp.git 2>/dev/null || warn "cupp clone failed"
if git clone --depth 1 https://github.com/nil0x42/duplicut.git 2>/dev/null; then
    make -C duplicut 2>/dev/null || warn "duplicut build failed"   # dedup huge wordlists fast
fi

# --- rule sets (the high-hit-rate ones) -------------------------------------
cd "{{.CrackDir}}/rules"
fetch_rule() { wget -q "$1" -O "$2" || warn "rule fetch failed: $2"; }
fetch_rule https://raw.githubusercontent.com/NotSoSecure/password_cracking_rules/master/OneRuleToRuleThemAll.rule OneRuleToRuleThemAll.rule
fetch_rule https://raw.githubusercontent.com/stealthsploit/OneRuleToRuleThemStill/main/OneRuleToRuleThemStill.rule OneRuleToRuleThemStill.rule
git clone --depth 1 https://github.com/praetorian-inc/Hob0Rules.git hob0 2>/dev/null || warn "Hob0Rules clone failed"
git clone --depth 1 https://github.com/clem9669/hashcat-rule.git clem9669 2>/dev/null || warn "clem9669 clone failed"

# --- wordlists --------------------------------------------------------------
cd "{{.CrackDir}}/wordlists"
[ -f rockyou.txt ] || wget -q https://github.com/brannondorsey/naive-hashcat/releases/download/data/rockyou.txt -O rockyou.txt || warn "rockyou fetch failed"
if [ -n "{{.WordlistURL}}" ]; then
    f="$(basename '{{.WordlistURL}}')"
    wget -q "{{.WordlistURL}}" -O "$f" && case "$f" in *.gz) gunzip -f "$f";; esac || warn "wordlist fetch failed"
fi

cat <<EOF
[+] crackbox ready
    wordlists : {{.CrackDir}}/wordlists  (rockyou + any --template-var WordlistURL)
    rules     : {{.CrackDir}}/rules  (OneRule*, Hob0Rules, clem9669)
    tools     : cupp, cewl, duplicut, hashcat-utils
    WPA crack : hashcat -m 22000 <hash.22000> {{.CrackDir}}/wordlists/rockyou.txt -r {{.CrackDir}}/rules/OneRuleToRuleThemAll.rule
    convert   : hcxpcapngtool -o hash.22000 {{.CrackDir}}/captures/*.pcapng
EOF
