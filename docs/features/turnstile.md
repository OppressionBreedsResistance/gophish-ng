# Cloudflare Turnstile Bot Protection

Gophish-NG supports [Cloudflare Turnstile](https://developers.cloudflare.com/turnstile/) as an optional bot protection layer for the phishing server. When enabled, every visitor must pass a Turnstile challenge before accessing any landing page or hosted attachment.

## Why Use Turnstile?

Automated security scanners and crawlers can prematurely expose phishing infrastructure. Turnstile silently verifies that the visitor is a real browser using behavioural analysis — without showing a CAPTCHA to legitimate users. Bots and scanners that cannot pass the challenge never reach your landing page.

## How It Works

1. A recipient clicks the phishing link: `https://phishing-domain.com/?keyname=<RId>`
2. The Turnstile middleware intercepts the request and checks for a valid session cookie
3. If no valid cookie exists, the visitor is shown a Cloudflare Turnstile challenge page
4. Cloudflare verifies the visitor server-side via the `siteverify` API
5. On success, a signed session cookie (`ts_v`) is set and the visitor is redirected back to the phishing page
6. Subsequent requests carry the cookie — no further challenge is shown for 1 hour

## Setup

### 1. Create a Turnstile site in Cloudflare Dashboard

1. Log in to [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. Go to **Turnstile** → **Add site**
3. Enter your phishing domain
4. Choose **Widget Mode**:
   - **Non-interactive** (recommended) — shows a loading spinner, no user interaction required
   - **Managed** — Cloudflare decides based on risk; most real browsers pass automatically
5. Copy the **Site Key** and **Secret Key**

### 2. Configure `config.json`

```json
{
  "turnstile": {
    "site_key": "0x4AAAAAAA...",
    "secret_key": "0x4AAAAAAA..."
  }
}
```

Leave both fields empty (`""`) to disable Turnstile entirely — the middleware is a no-op when keys are not configured.

## Exempt Endpoints

The following endpoints bypass the Turnstile challenge and are always accessible without a cookie:

| Path | Reason |
|------|--------|
| `/ts-verify` | The verification endpoint itself |
| `/track`, `/{path}/track` | Email open pixel — must work without a browser session |
| `/attachment`, `/{path}/attachment` | Payload execution beacon — called from scripts, not browsers |
| `/robots.txt` | Standard robots file |

## Session Cookie

After a successful challenge, a signed cookie `ts_v` is set:

```
ts_v=<RId>|<timestamp>|<HMAC-SHA256>
```

- Cookie is tied to the specific recipient (`RId`) — a cookie from one campaign link cannot be reused for another
- Expires after **1 hour**
- Signed with HMAC-SHA256 using the Turnstile secret key — cannot be forged

## Hosted Attachments

When [Hosted Attachments](hosted-attachments.md) are used, the Turnstile middleware extracts the `RId` from the URL path (`/static/attachments/<campaignId>/<RId>/<filename>`) and enforces the same session cookie check.
