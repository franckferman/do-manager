<div align="center">

```
    ██████╗  ██████╗       ███╗   ███╗ █████╗ ███╗   ██╗ █████╗  ██████╗ ███████╗██████╗
    ██╔══██╗██╔═══██╗      ████╗ ████║██╔══██╗████╗  ██║██╔══██╗██╔════╝ ██╔════╝██╔══██╗
    ██║  ██║██║   ██║█████╗██╔████╔██║███████║██╔██╗ ██║███████║██║  ███╗█████╗  ██████╔╝
    ██║  ██║██║   ██║╚════╝██║╚██╔╝██║██╔══██║██║╚██╗██║██╔══██║██║   ██║██╔══╝  ██╔══██╗
    ██████╔╝╚██████╔╝      ██║ ╚═╝ ██║██║  ██║██║ ╚████║██║  ██║╚██████╔╝███████╗██║  ██║
    ╚═════╝  ╚═════╝       ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═╝
```

**A modular Go CLI and library for managing DigitalOcean infrastructure.**

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-AGPL--3.0-blue?style=flat-square&label=license&message=GNU+Affero+v3)](LICENSE)
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
  - [Regions](#regions)
  - [Sizes](#sizes)
  - [Images](#images)
- [Library Usage](#library-usage)
- [Roadmap](#roadmap)

---

## Overview

`do-manager` is built on top of [`godo`](https://github.com/digitalocean/godo), the official DigitalOcean Go client. It is designed as a **dual-purpose project**:

- A **CLI tool** for interacting with your DigitalOcean account from the terminal.
- A **Go library** (`pkg/`) that can be imported by other projects to programmatically manage cloud resources.

Unlike a wrapper around `doctl`, `do-manager` talks directly to the DigitalOcean API via `godo`. This means no dependency on an external binary, proper Go types throughout, and full composability.

---

## Why do-manager?

Several tools already exist to interact with DigitalOcean. Here is where `do-manager` fits:

| | doctl | Terraform / Pulumi | do-manager |
|---|---|---|---|
| **Type** | CLI binary | Infrastructure-as-Code | CLI + Go library |
| **Importable as Go lib** | No | No | **Yes** |
| **Requires external binary** | Yes (doctl installed) | Yes (terraform/pulumi) | No |
| **State management** | No | Yes (statefile) | No |
| **Parallel batch ops** | No | Partial | **Yes** |
| **API coverage** | Full DO API | Full DO API | Focused (Droplets, keys, regions) |
| **Learning curve** | Low | High (HCL / SDK) | Low |
| **Embed in another Go project** | Subprocess + parsing | SDK only | `import "pkg/droplet"` |

### vs doctl

`doctl` is the official DigitalOcean CLI. It covers the entire API surface and is the right tool for manual operations from the terminal.

`do-manager` targets a different use case: **embedding cloud provisioning inside another Go program**. Instead of shelling out to `doctl` and parsing its output, you import `pkg/droplet` and call typed Go functions directly. No binary dependency, no version mismatch, no fragile string parsing.

### vs Terraform / Pulumi

Terraform and Pulumi are full infrastructure-as-code platforms. They manage state, handle drift detection, and orchestrate dozens of providers. That power comes with significant overhead: HCL or SDK boilerplate, a statefile to manage, and a heavy binary to ship.

`do-manager` has no concept of state. It is a thin, focused layer over the DigitalOcean API for programs that need to **provision and destroy Droplets programmatically** without pulling in an IaC framework.

### The one-liner

> Use `doctl` when you work from the terminal. Use `do-manager` when you build a Go tool that needs to manage Droplets.

---

## Features

| Area | Supported operations |
|---|---|
| **Droplets** | List, Get, Create (with `--wait`), Delete, Power on/off, Reboot |
| **SSH Keys** | List, Get, Add (from string or file), Delete |
| **Regions** | List all regions with availability |
| **Sizes** | List all size slugs with specs and pricing |
| **Images** | List by type (distribution / application / user) |

- Colored, aligned table output via `tablewriter` + `color`
- Automatic pagination for all list operations
- Token from env var, `--token` flag, or `~/.do-manager.yaml`
- `--wait` on `droplet create` polls until the Droplet is active and prints its IP

---

## Project Structure

```
do-manager/
├── main.go                   # entrypoint
├── go.mod
│
├── cmd/                      # Cobra CLI commands (not imported externally)
│   ├── root.go               # root command, config init
│   ├── version.go
│   ├── droplet.go
│   ├── sshkey.go
│   └── region.go
│
├── internal/
│   └── config/
│       └── config.go         # token resolution (env / flag / file)
│
└── pkg/                      # importable library packages
    ├── client/
    │   └── client.go         # authenticated godo.Client factory
    ├── droplet/
    │   └── droplet.go        # Droplet CRUD + power actions
    ├── sshkey/
    │   └── sshkey.go         # SSH key management
    └── region/
        └── region.go         # Regions, Sizes, Images
```

`cmd/` contains CLI-only code. `pkg/` is stable API surface, safe to import.

---

## Installation

**Prerequisites:** Go 1.23+

```bash
git clone https://github.com/franckferman/do-manager.git
cd do-manager
go mod tidy
go build -o do-manager .
```

Or install directly:

```bash
go install github.com/franckferman/do-manager@latest
```

---

## Configuration

`do-manager` resolves your API token in this priority order:

| Method | Example |
|---|---|
| `--token` flag | `do-manager --token dop_v1_xxx droplet list` |
| `DO_TOKEN` env var | `export DO_TOKEN=dop_v1_xxx` |
| Config file | `~/.do-manager.yaml` |

**Config file format** (`~/.do-manager.yaml`):

```yaml
token: dop_v1_your_token_here
```

Generate a token at: https://cloud.digitalocean.com/account/api/tokens

---

## CLI Usage

```
do-manager [command] [subcommand] [flags]
```

Global flag available for all commands:

```
--token string   DigitalOcean API token (overrides DO_TOKEN env var)
```

---

### Droplets

#### List all Droplets

```bash
do-manager droplet list
# aliases: do-manager d ls
```

```
>> 3 Droplet(s)

 ID         | Name       | Status | Region | Size          | IPv4           | IPv6 | Tags
------------|------------|--------|--------|---------------|----------------|------|-------
 123456789  | web-01     | active | fra1   | s-1vcpu-1gb   | 167.99.12.34   | -    | web
 123456790  | db-01      | active | fra1   | s-2vcpu-4gb   | 167.99.12.35   | -    | db
 123456791  | bastion    | off    | nyc1   | s-1vcpu-1gb   | 138.68.0.1     | -    |
```

#### Get a Droplet

```bash
do-manager droplet get 123456789
```

```
>> Droplet web-01

  ID:            123456789
  Status:        active
  Region:        Frankfurt 1 (fra1)
  Size:          s-1vcpu-1gb
  Specs:         1 vCPU  /  1024 MB RAM  /  25 GB disk
  IPv4:          167.99.12.34
  Tags:          web
  Created:       2024-01-15T10:30:00Z
```

#### Create a Droplet

```bash
# Minimal
do-manager droplet create --name web-02 --region fra1

# Full options
do-manager droplet create \
  --name pentest-lab \
  --region nyc1 \
  --size s-2vcpu-4gb \
  --image ubuntu-22-04-x64 \
  --ssh-keys 42000001,42000002 \
  --tags lab,pentest \
  --ipv6 \
  --wait
```

| Flag | Default | Description |
|---|---|---|
| `--name`, `-n` | (required) | Droplet name (or base name for batch) |
| `--count`, `-c` | `1` | Number of Droplets to provision in parallel |
| `--region`, `-r` | `nyc1` | Region slug |
| `--size`, `-s` | `s-1vcpu-1gb` | Size slug |
| `--image`, `-i` | `ubuntu-22-04-x64` | Image slug |
| `--ssh-keys` | none | SSH key IDs (comma-separated) |
| `--tags` | none | Tags (comma-separated) |
| `--user-data` | none | Cloud-init script |
| `--ipv6` | false | Enable IPv6 |
| `--backups` | false | Enable automatic backups |
| `--wait`, `-w` | false | Wait until active, print IP(s) |

**Batch example** - 5 Droplets in parallel, named `lab-01` to `lab-05`:

```bash
do-manager droplet create \
  --name lab \
  --count 5 \
  --region fra1 \
  --size s-1vcpu-1gb \
  --tags lab,pentest \
  --wait
```

#### Delete Droplet(s)

```bash
# Single
do-manager droplet delete 123456789

# Multiple IDs in parallel
do-manager droplet delete 111 222 333 444 555

# All Droplets with a tag (single API call)
do-manager droplet delete --tag lab

# Skip confirmation prompt
do-manager droplet delete --tag lab --force
```

#### Power actions

```bash
do-manager droplet power on 123456789
do-manager droplet power off 123456789
do-manager droplet power reboot 123456789
```

---

### SSH Keys

```bash
# List
do-manager ssh-key list

# Add from file
do-manager ssh-key add --name homelab --file ~/.ssh/id_ed25519.pub

# Add from string
do-manager ssh-key add --name ci-key --public-key "ssh-ed25519 AAAA..."

# Get by fingerprint
do-manager ssh-key get "xx:xx:xx:..."

# Delete
do-manager ssh-key delete "xx:xx:xx:..."
# alias: do-manager key rm "xx:xx:xx:..."
```

---

### Regions

```bash
do-manager region list
```

```
>> 14 region(s)

 Slug  | Name               | Available
-------|--------------------|----------
 nyc1  | New York 1         | yes
 ams3  | Amsterdam 3        | yes
 fra1  | Frankfurt 1        | yes
 sgp1  | Singapore 1        | yes
 ...
```

---

### Sizes

```bash
do-manager size list
```

```
>> 72 size(s)

 Slug             | vCPUs | Memory (MB) | Disk (GB) | Transfer (TB) | Price/mo | Regions
------------------|-------|-------------|-----------|---------------|----------|--------
 s-1vcpu-1gb      | 1     | 1024        | 25        | 1.0           | $6.00    | 12
 s-1vcpu-2gb      | 1     | 2048        | 50        | 2.0           | $12.00   | 12
 s-2vcpu-4gb      | 2     | 4096        | 80        | 4.0           | $24.00   | 12
 ...
```

---

### Images

```bash
# Distribution images (default)
do-manager image list

# Application images
do-manager image list --type application

# All images
do-manager image list --type ""
```

---

## Library Usage

`pkg/` packages are designed to be imported in other Go projects.

```go
import (
    "context"

    "github.com/franckferman/do-manager/pkg/client"
    "github.com/franckferman/do-manager/pkg/droplet"
    "github.com/franckferman/do-manager/pkg/sshkey"
    "github.com/franckferman/do-manager/pkg/region"
)

func main() {
    // Build an authenticated client
    c, err := client.New(os.Getenv("DO_TOKEN"))
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // List Droplets
    svc := droplet.New(c)
    droplets, err := svc.List(ctx)

    // Create a Droplet
    d, err := svc.Create(ctx, droplet.CreateOptions{
        Name:   "worker-01",
        Region: "fra1",
        Size:   "s-1vcpu-1gb",
        Image:  "ubuntu-22-04-x64",
    })

    // Manage SSH keys
    keySvc := sshkey.New(c)
    keys, err := keySvc.List(ctx)

    // List regions / sizes / images
    regSvc := region.New(c)
    regions, err := regSvc.ListRegions(ctx)
    sizes,   err := regSvc.ListSizes(ctx)
    images,  err := regSvc.ListImages(ctx, "distribution")
}
```

Each package is stateless and takes a `context.Context` on every call, making it straightforward to set deadlines and cancellation in larger programs.

---

## Roadmap

- [ ] `droplet create --count N` for batch provisioning
- [ ] `droplet ssh <id>` shortcut (wraps `ssh root@<ipv4>`)
- [ ] VPC / firewall management
- [ ] Output as JSON (`--output json`)
- [ ] Config profile switching (`--profile prod`)
- [ ] Volumes and Snapshots

---

## License

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).

Any use, modification, or distribution — including over a network — requires the full source code to remain open under the same license.
