# VPS Quick Setup

`setup_vps.sh` automates a full production deployment of Gophish-NG on a fresh Ubuntu/Debian VPS.

## What it installs

| Component | Details |
|-----------|---------|
| **Go** | Latest stable, downloaded from golang.org |
| **nginx** | Reverse proxy for all phishing domains |
| **acme.sh** | Let's Encrypt client, ECC P-256 certificates |
| **Gophish-NG** | Built from `master`, runs as dedicated system user |
| **systemd** | Service with automatic restart on failure |
| **cron** | Daily certificate renewal at 03:00 |

## Requirements

- Ubuntu 20.04+ or Debian 11+ (root / sudo access)
- DNS A records for your phishing domains pointing to the VPS IP
- Port 80 and 443 open in the firewall

## Usage

```bash
sudo bash setup_vps.sh
```

The script is fully interactive:

1. **How many domains?** — enter the number of phishing domains
2. **Domain names** — enter each domain (e.g. `phish.example.com`)
3. **Email address** — used for Let's Encrypt account registration
4. **Confirmation** — review the summary before anything is installed

## Architecture

```
Internet
   │
   ▼ :443 (TLS terminated by nginx)
[ nginx ]
   │
   ▼ http://127.0.0.1:5555
[ Gophish-NG phish server ]

Admin panel: https://127.0.0.1:3333 (local only)
```

- The phish server listens only on `127.0.0.1:5555` — never exposed directly.
- The admin panel listens on `127.0.0.1:3333` — access via SSH tunnel.
- TLS is handled entirely by nginx using Let's Encrypt certificates.

## Accessing the admin panel

The admin panel is not exposed to the internet. Access it through an SSH local port forward:

```bash
ssh -L 3333:127.0.0.1:3333 user@<VPS_IP>
```

Then open `https://localhost:3333` in your browser.

## Default credentials

Printed once on first run, visible in the systemd journal:

```bash
journalctl -u gophish | grep "Please login"
```

## Managing the service

```bash
# Status
systemctl status gophish

# Restart
systemctl restart gophish

# Live logs
journalctl -u gophish -f
```

## Re-running the script

The script is idempotent — safe to run again on an existing installation:

- Pulls the latest `master` branch and rebuilds the binary
- Skips already-installed packages and existing certificates
- Overwrites `config.json` and the systemd unit file

!!! warning "DNS must resolve before running"
    The Let's Encrypt HTTP-01 challenge requires that each domain already points to the VPS IP in DNS. Certificate issuance will fail if DNS is not propagated yet.
