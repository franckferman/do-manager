#!/bin/bash
# @name: crackbox
# @desc: GPU password-cracking box - hashcat + hcxtools + hashcat-utils, multiple rule sets, cupp/cewl/duplicut, wordlists staged
# @var: WordlistURL= - Optional extra wordlist to fetch (weakpass/SecLists .gz URL); rockyou is always staged
# @var: SpacesBucket= - Optional DO Spaces bucket of pre-staged wordlists to mount via s3fs (no re-download; needs SPACES_KEY/SPACES_SECRET in env)
# @var: SpacesEndpoint=nyc3.digitaloceanspaces.com - DO Spaces endpoint for --template-var SpacesBucket
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
clone() { git clone --depth 1 "$1" "$2" 2>/dev/null || warn "clone failed: $2"; }
clone https://github.com/Mebus/cupp.git cupp
clone https://github.com/iphelix/pack.git pack        # PACK: maskgen/statsgen from cracked pw
if git clone --depth 1 https://github.com/nil0x42/duplicut.git 2>/dev/null; then
    make -C duplicut 2>/dev/null || warn "duplicut build failed"   # dedup huge wordlists fast
fi

# --- rule sets (high-hit-rate + big collections) ----------------------------
cd "{{.CrackDir}}/rules"
fetch_rule() { wget -q "$1" -O "$2" || warn "rule fetch failed: $2"; }
fetch_rule https://raw.githubusercontent.com/NotSoSecure/password_cracking_rules/master/OneRuleToRuleThemAll.rule OneRuleToRuleThemAll.rule
fetch_rule https://raw.githubusercontent.com/stealthsploit/OneRuleToRuleThemStill/main/OneRuleToRuleThemStill.rule OneRuleToRuleThemStill.rule
clone https://github.com/praetorian-inc/Hob0Rules.git hob0
clone https://github.com/clem9669/hashcat-rule.git clem9669
clone https://github.com/NSAKEY/nsa-rules.git nsa
clone https://github.com/kaonashi-passwords/Kaonashi.git kaonashi
clone https://github.com/n0kovo/hashcat-rules-collection.git n0kovo-collection

# --- wordlists --------------------------------------------------------------
cd "{{.CrackDir}}/wordlists"
# Pre-staged wordlists via DO Spaces (s3fs): mount once, no re-download at $/hr.
# Opt-in; needs SPACES_KEY/SPACES_SECRET in the environment (tradeoff: creds on the box).
# Simpler alternative: build this box once, snapshot it (do-manager snapshot), reuse the snapshot.
if [ -n "{{.SpacesBucket}}" ] && [ -n "${SPACES_KEY:-}" ] && [ -n "${SPACES_SECRET:-}" ]; then
    apt-get install -y s3fs && {
        echo "$SPACES_KEY:$SPACES_SECRET" > /etc/passwd-s3fs; chmod 600 /etc/passwd-s3fs
        s3fs "{{.SpacesBucket}}" "{{.CrackDir}}/wordlists" \
            -o url="https://{{.SpacesEndpoint}}" -o use_path_request_style \
            && echo "[+] Spaces wordlists mounted" || warn "s3fs mount failed"
    } || warn "s3fs install failed"
elif [ -n "{{.SpacesBucket}}" ]; then
    warn "SpacesBucket set but SPACES_KEY/SPACES_SECRET not in env — skipping mount"
fi
[ -f rockyou.txt ] || wget -q https://github.com/brannondorsey/naive-hashcat/releases/download/data/rockyou.txt -O rockyou.txt || warn "rockyou fetch failed"
if [ -n "{{.WordlistURL}}" ]; then
    f="$(basename '{{.WordlistURL}}')"
    wget -q "{{.WordlistURL}}" -O "$f" && case "$f" in *.gz) gunzip -f "$f";; esac || warn "wordlist fetch failed"
fi

cat <<EOF
[+] crackbox ready
    wordlists : {{.CrackDir}}/wordlists  (rockyou + any --template-var WordlistURL; big lists: pre-stage via snapshot/Spaces)
    rules     : {{.CrackDir}}/rules  (OneRule*, Hob0Rules, clem9669, nsa, Kaonashi, n0kovo-collection)
    tools     : cupp, cewl, duplicut, PACK (maskgen), hashcat-utils
    WPA crack : hashcat -m 22000 <hash.22000> {{.CrackDir}}/wordlists/rockyou.txt -r {{.CrackDir}}/rules/OneRuleToRuleThemAll.rule
    convert   : hcxpcapngtool -o hash.22000 {{.CrackDir}}/captures/*.pcapng
EOF
