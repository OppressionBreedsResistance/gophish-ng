# Hosted Attachments

Gophish-NG can serve the campaign attachment directly from the phishing server instead of embedding it in the email. When a recipient clicks the phishing link, they are automatically redirected to a download URL unique to them.

## How It Works

1. When a campaign is launched with **Host Attachment** enabled, Gophish-NG writes a personalised copy of the attachment (with all template placeholders substituted) to disk under `static/endpoint/attachments/<campaignId>/<RId>/`.
2. When the recipient clicks the phishing link, the server redirects them to `/static/attachments/<campaignId>/<RId>/<filename>`.
3. The file is served directly by the phishing server — no email attachment required.

## Enabling Hosted Attachments

In the campaign settings, check the **Host Attachment** option. The first attachment from the selected email template will be used.

!!! note
    Only the **first attachment** of the email template is hosted. Any additional attachments are ignored when this option is enabled.

## Use Cases

- Deliver payloads without triggering email attachment scanning (AV/sandbox)
- Track the exact moment a recipient downloads/opens the file (separate from the click event)
- Combine with the [Attachment Tracking](attachment-tracking.md) beacon to record execution

## File Path Format

```
/static/attachments/<campaignId>/<RId>/<filename>
```

Each recipient gets their own personalised copy of the file — placeholder substitution (e.g. `{{.FirstName}}`, `{{.URL}}`, `{{.RId}}`) is applied per-recipient at send time.

## Interaction with Cloudflare Turnstile

When [Turnstile](turnstile.md) is enabled, hosted attachment URLs are also protected. The recipient must pass the challenge before the file is served. The Turnstile middleware extracts the RId from the URL path and verifies the session cookie.
