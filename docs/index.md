# Gophish-NG

**Gophish-NG** is a fork of the open-source [Gophish](https://github.com/gophish/gophish) phishing simulation framework, extended with additional capabilities for red team engagements.

## What's new in Gophish-NG?

| Feature | Description |
|---------|-------------|
| [Attachment Tracking](features/attachment-tracking.md) | Track when a recipient executes a delivered payload |
| [Hosted Attachments](features/hosted-attachments.md) | Serve the payload directly from the phishing server — bypasses email attachment scanning |
| [Cloudflare Turnstile](features/turnstile.md) | Silent bot protection — blocks scanners before they reach your landing page |
| [Password-Protected ZIP](features/password-zip.md) | Send encrypted ZIP attachments with per-template passwords |
| [IOC Removal](features/ioc-removal.md) | Replace Gophish-specific headers and parameters |
| Script support | `.ps1`, `.bat`, `.pdf` files support placeholder substitution |
| Campaign Results | "Clicked Attachment" and "Email Reported" status in results table |

## Quick Start

```bash
git clone https://github.com/OppressionBreedsResistance/gophish-ng.git
cd gophish-ng
go build
./gophish
```

Open `https://localhost:3333` in your browser. Login credentials are printed in the console on first run.

!!! warning "Authorized Use Only"
    Gophish-NG is intended for authorized security assessments, red team engagements, and security awareness training only.
