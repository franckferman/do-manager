<div align="center">

# do-manager

**A modular Go CLI and library for managing DigitalOcean infrastructure.**

[![CI](https://github.com/franckferman/do-manager/actions/workflows/ci.yml/badge.svg)](https://github.com/franckferman/do-manager/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-AGPL--3.0-blue?style=flat-square)](LICENSE)
[![DigitalOcean API](https://img.shields.io/badge/API-DigitalOcean_v2-0080ff?style=flat-square&logo=digitalocean)](https://docs.digitalocean.com/reference/api/)

</div>

---

## Table of Contents

- [Overview](#overview)
- [Why do-manager?](#why-do-manager)
- [Features](#features)
- [Project Structure](#project-structure)
- [Installation](#installation)
- [Configuration](#configuration)
- [CLI Usage](#cli-usage)
  - [Droplets](#droplets)
  - [SSH Keys](#ssh-keys)
  - [Regions / Sizes / Images](#regions--sizes--images)
  - [DNS](#dns)
  - [Firewalls](#firewalls)
  - [Reserved IPs](#reserved-ips)
  - [Snapshots](#snapshots)
  - [VPC](#vpc)
  - [Campaign](#campaign)
  - [Audit](#audit)
  - [Templates](#templates)
  - [JSON Output](#json-output)
  - [Shell Completion](#shell-completion)
- [Library Usage](#library-usage)
- [License](#license)

---

## Overview

`do-manager` is built directly on [`godo`](https://github.com/digitalocean/godo), the official DigitalOcean Go client. Two interfaces, one codebase:

- A **CLI tool** for operational use from the terminal - campaign orchestration, IP rotation, node rebuild, security audit.
- A **Go library** (`pkg/`) that can be imported directly into any Go program.

Unlike `doctl`, `do-manager` calls the API directly via `godo` - no subprocess, no string parsing, proper Go types throughout.

The design targets **Red Team infrastructure**: deploy a full engagement stack in one command, rotate IPs when they get burned, rebuild a single compromised node without touching the rest, tear everything down cleanly. The primitive operations (create/list/delete/power) are there, but the value is in the higher-level abstractions: `campaign`, firewall presets, `rotate-ips`, `rebuild`, `audit`.

---

## Why do-manager?

Three tools are commonly used to manage DO infrastructure from a Red Team perspective:

| | doctl | bash + curl | Terraform / Pulumi | do-manager |
|---|---|---|---|---|
| **Type** | Official CLI | Ad-hoc scripts | Infrastructure-as-Code | CLI + Go library |
| **Campaign orchestration** | No | Manual | Partial | **Yes** (deploy/status/destroy/rotate) |
| **Firewall presets** | No | No | No | **Yes** (c2/phishing/redirector/bastion/lockdown) |
| **IP rotation** | No | Script it yourself | No | **Yes** (rotate-ips) |
| **Parallel batch ops** | No | `&` + wait | Partial | **Yes** (goroutines, per-slot errors) |
| **Security audit** | No | No | No | **Yes** (exit 1 on CRITICAL/HIGH) |
| **Importable as Go lib** | No | No | No | **Yes** (`import "pkg/droplet"`) |
| **Requires state file** | No | No | Yes (.tfstate) | No (stateless) |
| **Requires external binary** | Yes (doctl) | curl, jq | Yes (terraform) | **No** |

**doctl** is fine for manual one-off ops. It covers every DO resource but has no campaign concept, no firewall presets, and no Red Team abstractions.

**bash + curl** is where most people start. It breaks down under engagement pressure: parallel provisioning races, ID extraction with fragile `grep`/`jq`, no error aggregation per Droplet, scripts that diverge between operators.

**Terraform** is the right choice for long-lived stable infrastructure. It is overkill for engagements: statefile management is a liability during ops, the feedback loop is slow, and there are no Red Team primitives.

**do-manager** is the choice when you need to deploy a full stack fast, rotate IPs when they get burned, rebuild a single node without disrupting the campaign, tear everything down cleanly, and optionally embed all of that into a Go automation tool.

---

## Features

| Area | Operations |
|---|---|
| **Droplets** | list, get, create (batch, `--wait`), delete, power on/off/reboot, ssh, exec (parallel), ips, rebuild |
| **SSH Keys** | list, get, add (file or string), delete |
| **Regions / Sizes / Images** | list with full specs |
| **DNS** | domain CRUD + record CRUD (A, AAAA, CNAME, MX, TXT, NS, SRV, CAA) |
| **Firewalls** | list, get, create, delete, attach, detach + 5 opinionated presets |
| **Reserved IPs** | list, get, reserve, delete, assign, unassign |
| **Snapshots** | list, get, create (`--wait`), delete |
| **VPC** | list, get, create, delete, members |
| **Campaign** | deploy (multi-role), status, destroy, rotate-ips |
| **Audit** | security scan with CRITICAL/HIGH/MEDIUM/INFO findings, CI-friendly exit codes |
| **Templates** | list, show, dump + `--template` on droplet/campaign + `text/template` variable substitution |

Additional:
- `-o json` on every command for scripting and pipelines
- `--user-data` / `--user-data-file` on `droplet create` and `campaign deploy`
- `--vpc-uuid` on `droplet create` and `campaign deploy`
- Shell completion for bash, zsh, fish, PowerShell
- `ldflags` version embedding (Version, Commit, BuildDate)
- GoReleaser for multi-platform binary releases (linux/darwin/windows, amd64/arm64)

---

## Project Structure

```
do-manager/
├── main.go
├── go.mod
│
├── cmd/                      # CLI commands (not imported externally)
│   ├── root.go
│   ├── output.go             # -o json global flag
│   ├── completion.go
│   ├── version.go
│   ├── droplet.go            # droplet + ssh/exec/ips/rebuild
│   ├── sshkey.go
│   ├── region.go
│   ├── dns.go
│   ├── firewall.go           # firewall + preset command
│   ├── reservedip.go
│   ├── snapshot.go
│   ├── vpc.go
│   ├── campaign.go           # campaign deploy/status/destroy/rotate-ips
│   ├── audit.go
│   └── template.go           # template list/show/dump
│
├── internal/
│   └── config/
│       └── config.go
│
└── pkg/                      # importable library packages
    ├── client/               # authenticated godo.Client factory
    ├── droplet/              # Droplet CRUD + batch + rebuild
    ├── sshkey/
    ├── region/               # Regions, Sizes, Images
    ├── dns/                  # Domains + DNS records
    ├── firewall/             # Firewalls + RuleSpec parser
    ├── reservedip/
    ├── snapshot/
    ├── vpc/
    └── tmpl/                 # embedded cloud-init templates + renderer
```

`pkg/` is stable API surface. `cmd/` is CLI-only and not meant to be imported.

---

## Installation

**Prerequisites:** Go 1.23+

### go install

```bash
go install github.com/franckferman/do-manager@latest
```

### From source

```bash
git clone https://github.com/franckferman/do-manager.git
cd do-manager
go mod tidy
make build
./do-manager version
```

### Binary release

Pre-built binaries for linux/darwin/windows on amd64/arm64 are available on the [Releases](https://github.com/franckferman/do-manager/releases) page.

---

## Configuration

Token resolution order (first match wins):

| Method | Example |
|---|---|
| `--token` flag | `do-manager --token dop_v1_xxx droplet list` |
| `DO_TOKEN` env var | `export DO_TOKEN=dop_v1_xxx` |
| Config file | `~/.do-manager.yaml` |

**Config file** (`~/.do-manager.yaml`):

```yaml
token: dop_v1_your_token_here
```

Generate a token at: https://cloud.digitalocean.com/account/api/tokens

---

## CLI Usage

```
do-manager [command] [subcommand] [flags]
```

Global flags available on every command:

```
--token string     DigitalOcean API token (overrides DO_TOKEN)
-o, --output string  Output format: table (default) | json
```

---

### Droplets

```bash
# List / inspect
do-manager droplet list
do-manager droplet get <id-or-name>

# Create - single
do-manager droplet create --name web-01 --region fra1 --size s-1vcpu-1gb --wait

# Create - batch (5 Droplets in parallel, named lab-01 to lab-05)
do-manager droplet create --name lab --count 5 --region fra1 --wait

# Create with startup script and VPC
do-manager droplet create --name c2-node \
  --region fra1 --snapshot gophish-v2 \
  --vpc-uuid <uuid> \
  --user-data-file ./setup.sh \
  --wait

# Delete
do-manager droplet delete <id>           # single
do-manager droplet delete 111 222 333    # multiple in parallel
do-manager droplet delete --tag lab --force  # by tag

# Power
do-manager droplet power on <id>
do-manager droplet power off <id>
do-manager droplet power reboot <id>

# Rebuild - wipe disk, reinstall from new image, keep ID + reserved IP
do-manager droplet rebuild <id> --image ubuntu-22-04-x64 --wait

# SSH - interactive session
do-manager droplet ssh <id-or-name>
do-manager droplet ssh <id-or-name> --user ubuntu --port 2222

# Print IPs (one per line, pipeable)
do-manager droplet ips
do-manager droplet ips --tag c2 | xargs nmap -sV -p 443

# Execute a command on all Droplets with a tag (parallel)
do-manager droplet exec --tag lab -- "uname -r"
do-manager droplet exec --tag c2 -- "systemctl status havoc"
```

**`droplet create` flags:**

| Flag | Default | Description |
|---|---|---|
| `--name`, `-n` | required | Droplet name |
| `--count`, `-c` | `1` | Provision N Droplets in parallel |
| `--region`, `-r` | `nyc1` | Region slug |
| `--size`, `-s` | `s-1vcpu-1gb` | Size slug |
| `--image`, `-i` | `ubuntu-22-04-x64` | Image slug |
| `--ssh-keys` | none | SSH key IDs (comma-separated) |
| `--tags` | none | Tags |
| `--user-data` | none | Cloud-init script (inline) |
| `--user-data-file` | none | Path to startup script |
| `--vpc-uuid` | none | Place inside this VPC |
| `--ipv6` | false | Enable IPv6 |
| `--backups` | false | Enable automatic backups |
| `--wait`, `-w` | false | Poll until active, print IP |

---

### SSH Keys

```bash
do-manager ssh-key list
do-manager ssh-key get <fingerprint>
do-manager ssh-key add --name homelab --file ~/.ssh/id_ed25519.pub
do-manager ssh-key add --name ci --public-key "ssh-ed25519 AAAA..."
do-manager ssh-key delete <fingerprint>
```

---

### Regions / Sizes / Images

```bash
do-manager region list
do-manager size list
do-manager image list                        # distribution images
do-manager image list --type application
do-manager image list --type user            # your own snapshots
```

---

### DNS

```bash
# Domains
do-manager dns list
do-manager dns get example.com
do-manager dns create --domain example.com --ip 1.2.3.4
do-manager dns delete example.com

# Records
do-manager dns records example.com
do-manager dns add --domain example.com --type A     --name @    --data 1.2.3.4
do-manager dns add --domain example.com --type CNAME --name mail --data @
do-manager dns add --domain example.com --type TXT   --name @    --data "v=spf1 mx -all"
do-manager dns add --domain example.com --type MX    --name @    --data "mail.example.com" --ttl 3600
do-manager dns rm-record example.com <record-id>
```

---

### Firewalls

Rule format: `"proto:ports:addresses"`
- `proto`: `tcp` | `udp` | `icmp`
- `ports`: single port, range (`80-443`), or `0` for icmp
- `addresses`: comma-separated CIDRs/IPs, or `any` (expands to `0.0.0.0/0,::/0`)

```bash
do-manager firewall list
do-manager firewall get <id>
do-manager firewall create --name my-fw \
  --inbound "tcp:443:any" \
  --inbound "tcp:22:203.0.113.1" \
  --outbound "tcp:0-65535:any" \
  --droplets 12345678
do-manager firewall attach <id> --droplets 12345678,87654321
do-manager firewall detach <id> --droplets 12345678
do-manager firewall delete <id>
```

#### Firewall Presets

Apply an opinionated ruleset for a common role. SSH is restricted to `--operator-ip` when specified.

| Profile | Inbound | Outbound | Use case |
|---|---|---|---|
| `c2` | 443, 80, 53/tcp+udp, SSH(op-ip) | all | Command & Control server |
| `phishing` | 443, 80, 8080, SSH(op-ip) | all | GoPhish / evilginx2 |
| `redirector` | 443, 80, SSH(op-ip) | 443, 80 | Traffic redirector |
| `bastion` | SSH(op-ip) only | all | Jump host |
| `lockdown` | SSH(op-ip) only | none | Fully locked node |

```bash
do-manager firewall preset c2 \
  --name c2-fw \
  --droplets 12345678 \
  --operator-ip 203.0.113.1
```

---

### Reserved IPs

Static addresses that survive Droplet deletion or rebuilds. Point DNS to the reserved IP and swap the underlying Droplet freely.

```bash
do-manager reservedip list
do-manager reservedip get 1.2.3.4
do-manager reservedip reserve --region fra1 --droplet 12345678
do-manager reservedip assign 1.2.3.4 --droplet 87654321
do-manager reservedip unassign 1.2.3.4
do-manager reservedip delete 1.2.3.4
```

---

### Snapshots

Snapshots are the foundation for repeatable deployments. The workflow: provision one Droplet, install and configure your tooling (GoPhish, Havoc, evilginx2, nginx redirector config, WireGuard), take a snapshot, delete the source Droplet. On engagement day, `campaign deploy --snapshot <slug>` restores that exact state across N nodes in parallel. No reinstalling, no drift between nodes.

```bash
do-manager snapshot list
do-manager snapshot get <id>

# Freeze a configured Droplet into a reusable image
do-manager snapshot create --droplet 12345678 --name gophish-v2 --wait

# Now deploy that exact state to 5 nodes
do-manager campaign deploy --name phish-q2 --snapshot gophish-v2 --count 5 ...

do-manager snapshot delete <id>
```

---

### VPC

An isolated private network scoped to a region. Droplets share a private IP range; traffic between them never leaves the DO fabric.

**What VPC is and is not.** A VPC is a network isolation layer - it gives your C2 node a private IP that only other Droplets in the same VPC can reach. It is not the channel you use to proxy C2 traffic; that is the redirector's job. The redirector (nginx, socat, Apache mod_proxy) forwards inbound HTTPS to the C2 over the private IP. No public firewall rule needed on the C2 node.

Three common approaches to route traffic from redirector to C2:

| Method | How it works | Notes |
|---|---|---|
| **DO VPC private IP** | Both nodes same region/VPC. nginx forwards to `10.x.x.x:port`. | Zero extra setup. Same region required. |
| **WireGuard** | VPN tunnel between redirector and C2. C2 listens on `wg0` only. | Cross-provider, cross-region. Extra cloud-init setup. |
| **SSH reverse tunnel** | C2 runs `ssh -R 8080:localhost:8080 user@redirector`. | Simple. Use autossh/systemd to survive disconnects. |

```
           Internet
               |
   Agent HTTPS :443
               |
        [Redirector]  1.2.3.4  public: 80/443 open
         nginx proxy_pass http://10.20.0.2:443
               |
  VPC 10.20.0.0/24  (private, stays inside datacenter fabric)
               |
        [C2 Server]  10.20.0.2  no public C2 port
                       public firewall: SSH only from operator IP
```

```bash
do-manager vpc list
do-manager vpc get <id>
do-manager vpc create --name c2-net --region fra1 --ip-range 10.20.0.0/24
do-manager vpc members <id>
do-manager vpc delete <id>

# Attach a Droplet to the VPC at creation time
do-manager droplet create --name c2-node --vpc-uuid <id> ...

# Or let campaign create the VPC automatically
do-manager campaign deploy --name op-ghost --vpc-auto ...
```

---

### Campaign

Deploy and tear down a complete infrastructure campaign with a single command. A campaign groups all resources under the tag `campaign:<name>`.

The campaign lifecycle exists because Red Team infra has a specific tempo: provision fast, rotate IPs when they get burned, rebuild compromised nodes without disrupting the rest, tear down cleanly at the end. `campaign deploy` creates Droplets, firewall, and reserved IPs in the right order and tags everything. `campaign destroy` deletes all of it in one shot. Nothing left behind.

#### Simple (single role)

```bash
do-manager campaign deploy \
  --name phish-q2 \
  --count 3 \
  --region fra1 \
  --snapshot gophish-v2 \
  --inbound "tcp:443:any" --inbound "tcp:22:203.0.113.1" \
  --outbound "tcp:0-65535:any" \
  --reserve-ips \
  --user-data-file ./setup_gophish.sh

do-manager campaign status --name phish-q2
do-manager campaign destroy --name phish-q2 --force
```

#### Multi-role (heterogeneous infrastructure)

Deploy C2 nodes and redirectors in one command. Each role gets its own image, firewall, and reserved IPs.

```bash
do-manager campaign deploy \
  --name op-ghost \
  --region fra1 \
  --operator-ip 203.0.113.1 \
  --vpc-auto \
  --role "c2:count=2:snapshot=c2-snap:preset=c2" \
  --role "redirector:count=3:snapshot=redir-snap:preset=redirector:reserve-ips"
```

**Role spec format:** `name[:count=N][:snapshot=slug][:preset=name][:size=slug][:reserve-ips][:user-data-file=path]`

| Token | Description |
|---|---|
| `count=N` | Number of Droplets for this role |
| `snapshot=slug` | Image slug (falls back to `--snapshot`) |
| `preset=name` | Firewall preset: c2, phishing, redirector, bastion, lockdown |
| `size=slug` | Size slug (falls back to `--size`) |
| `reserve-ips` | Reserve one IP per Droplet |
| `user-data-file=path` | Startup script for this role |

`--vpc-auto` creates a VPC automatically and places all Droplets inside it.
`--operator-ip` is forwarded to all preset-based firewall rules for SSH restriction.

#### Rotate IPs

When an IP is burned (blacklisted, domain seized), rotate all reserved IPs without downtime:

```bash
do-manager campaign rotate-ips --name op-ghost
#  op-ghost-redirector-01  167.99.10.1 -> 45.33.1.10
#  op-ghost-redirector-02  167.99.10.2 -> 45.33.1.11
```

#### Rebuild a single node

Re-image one burned Droplet while the campaign stays live. Reserved IP stays assigned.

```bash
do-manager droplet rebuild op-ghost-c2-01 --image c2-snap --wait
```

---

### Audit

Scan your account for security misconfigurations. Exits with code `1` on CRITICAL or HIGH findings.

```bash
do-manager audit
do-manager audit --max-snapshot-age 30
do-manager audit -o json | jq '.findings[] | select(.severity=="CRITICAL")'
```

| Severity | Check |
|---|---|
| CRITICAL | Droplet with no firewall attached |
| HIGH | Firewall rule exposes port 22/3389/5900 to `0.0.0.0/0` |
| MEDIUM | Reserved IP not assigned to any Droplet (idle billing) |
| INFO | Snapshot older than `--max-snapshot-age` days |

---

### Templates

Built-in cloud-init bash scripts embedded in the binary. Variable substitution via Go's `text/template` (`{{.VarName}}` syntax). Unset variables fall back to declared defaults.

```bash
# List all templates with their variables and defaults
do-manager template list

# Inspect a template before deploying
do-manager template show redirector-nginx

# Render a template with variable overrides and print to stdout (preview before deploy)
do-manager template dump c2-havoc \
  --template-var C2Host=1.2.3.4 \
  --template-var TeamserverPassword=s3cr3t

# Use directly on droplet create
do-manager droplet create --name c2-01 --region fra1 --image ubuntu-22-04-x64 \
  --template c2-havoc \
  --template-var C2Host=1.2.3.4 \
  --template-var TeamserverPassword=s3cr3t \
  --wait

# Use in multi-role campaign (template= token in role spec)
do-manager campaign deploy \
  --name op-phantom --region fra1 --operator-ip 203.0.113.5 --vpc-auto \
  --role "c2:count=1:preset=c2:template=c2-havoc" \
  --role "redirector:count=3:preset=redirector:reserve-ips:template=redirector-nginx" \
  --template-var C2BackendHost=10.10.0.2 \
  --template-var Domain=cdn.example.com
```

| Template | Description | Key variables |
|---|---|---|
| `c2-havoc` | Havoc C2 - build from source, teamserver as systemd service | `C2Port`, `C2Host`, `TeamserverPassword` |
| `c2-sliver` | Sliver C2 - latest release, operator config, systemd service | `C2Port`, `OperatorName` |
| `redirector-nginx` | nginx HTTPS reverse proxy, URI path filtering, certbot/self-signed TLS | `C2BackendHost`, `C2BackendPort`, `Domain`, `C2URIPath` |
| `redirector-apache` | Apache2 mod_rewrite, User-Agent + URI filtering for C2 profiles | `C2BackendHost`, `C2UserAgent`, `C2URIPath` |
| `wireguard-server` | WireGuard server - generates keys, configures wg0, enables IP forwarding | `WGListenPort`, `WGServerNet` |
| `wireguard-client` | WireGuard peer - connects to a WireGuard server | `WGServerEndpoint`, `WGServerPublicKey`, `WGClientNet` |
| `gophish` | GoPhish phishing framework - latest release, admin panel on localhost | `AdminListenPort`, `PhishListenPort` |
| `evilginx2` | evilginx2 reverse proxy phishing - latest release, auto-detect public IP | `Domain`, `ExternalIP`, `RedirectURL` |
| `hardening` | SSH hardening, fail2ban, non-root operator user | `SSHPort`, `OperatorUser`, `OperatorPubKey` |

Templates are stored in `pkg/tmpl/scripts/` and embedded in the binary via `//go:embed`. To add a custom template, add a `.sh` file with the metadata header:

```bash
#!/bin/bash
# @name: my-tool
# @desc: One-line description shown in template list
# @var: Port=443 - Listening port
# @var: Password= - Admin password (required, no default)
```

---

### JSON Output

Pass `-o json` to any command to get machine-readable output:

```bash
do-manager droplet list -o json | jq '.[].id'
do-manager campaign status --name op-ghost -o json
do-manager audit -o json | jq '.findings | length'
```

---

### Shell Completion

```bash
# zsh
do-manager completion zsh > "${fpath[1]}/_do-manager"

# bash
do-manager completion bash > /etc/bash_completion.d/do-manager

# fish
do-manager completion fish > ~/.config/fish/completions/do-manager.fish

# PowerShell
do-manager completion powershell | Out-String | Invoke-Expression
```

---

## Library Usage

All `pkg/` packages are stateless and accept a `context.Context` on every call.

```go
import (
    "context"
    "os"

    "github.com/franckferman/do-manager/pkg/client"
    "github.com/franckferman/do-manager/pkg/droplet"
    "github.com/franckferman/do-manager/pkg/vpc"
    "github.com/franckferman/do-manager/pkg/dns"
    "github.com/franckferman/do-manager/pkg/firewall"
    "github.com/franckferman/do-manager/pkg/reservedip"
    "github.com/franckferman/do-manager/pkg/snapshot"
    "github.com/franckferman/do-manager/pkg/tmpl"
)

func main() {
    c, _ := client.New(os.Getenv("DO_TOKEN"))
    ctx  := context.Background()

    // Droplets
    dropSvc := droplet.New(c)
    list, _ := dropSvc.List(ctx)
    d, _    := dropSvc.Create(ctx, droplet.CreateOptions{
        Name: "worker-01", Region: "fra1",
        Size: "s-1vcpu-1gb", Image: "ubuntu-22-04-x64",
        VPCUUID: "<vpc-id>",
        UserData: "#!/bin/bash\napt-get update -y",
    })
    // Batch: 5 in parallel
    results := dropSvc.CreateBatch(ctx, droplet.CreateOptions{Name: "worker"}, 5)
    // Rebuild a burned node
    action, _ := dropSvc.Rebuild(ctx, d.ID, "ubuntu-22-04-x64")

    // VPC
    vpcSvc := vpc.New(c)
    v, _ := vpcSvc.Create(ctx, vpc.CreateOptions{Name: "lab-net", Region: "fra1"})
    members, _ := vpcSvc.Members(ctx, v.ID, "droplet")

    // DNS
    dnsSvc := dns.New(c)
    dnsSvc.CreateDomain(ctx, "example.com", "1.2.3.4")
    dnsSvc.CreateRecord(ctx, "example.com", "A", "@", "1.2.3.4", 1800)

    // Firewall
    fwSvc := firewall.New(c)
    inRule, _ := firewall.ParseRuleSpec("tcp:443:any")
    fw, _ := fwSvc.Create(ctx, firewall.CreateOptions{
        Name:         "my-fw",
        InboundRules: []firewall.RuleSpec{inRule},
        DropletIDs:   []int{d.ID},
    })

    // Reserved IPs
    ripSvc := reservedip.New(c)
    rip, _ := ripSvc.Reserve(ctx, "fra1", d.ID)
    ripSvc.Unassign(ctx, rip.IP)

    // Snapshots
    snapSvc := snapshot.New(c)
    snaps, _ := snapSvc.List(ctx)
    snapSvc.CreateFromDroplet(ctx, d.ID, "my-snapshot", true) // true = wait

    // Templates
    metas, _ := tmpl.List()
    script, _ := tmpl.Render("redirector-nginx", map[string]string{
        "C2BackendHost": "10.20.0.2",
        "Domain":        "cdn.example.com",
    })
    _ = metas; _ = script

    _ = list; _ = results; _ = action; _ = members; _ = fw; _ = snaps
}
```

---

## License

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).

Any use, modification, or distribution - including over a network - requires the full source code to remain open under the same license.
