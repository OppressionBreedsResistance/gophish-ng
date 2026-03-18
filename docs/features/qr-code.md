# QR Code Placeholder

Gophish-NG can generate a unique QR code per recipient and embed it directly in the email body using the `{{.QR}}` placeholder.

## How It Works

1. Add `{{.QR}}` anywhere in your email template HTML
2. Gophish-NG generates a QR code PNG for each recipient at send time
3. The QR code encodes that recipient's unique phishing URL (including `keyname` parameter)
4. The image is embedded inline as a CID attachment — no external image hosting needed

## Usage

In the CKEditor template editor, type `{{` to open the autocomplete dropdown and select **QR**. Or type it manually:

```html
<p>Scan the QR code below to access the document:</p>
{{.QR}}
```

## What the QR Code Contains

The QR code encodes the full personalized phishing URL:

```
https://your-phishing-server.com/?keyname=RECIPIENT_ID
```

When the recipient scans the QR code and visits the URL, Gophish-NG records a **Clicked Link** event — the same as clicking the link directly.

## Technical Details

| Property | Value |
|----------|-------|
| Format | PNG |
| Size | 256 × 256 px |
| Error correction | Medium |
| Embedding | Inline CID (no external requests) |
| Library | `github.com/skip2/go-qrcode` |

!!! tip
    The QR code works alongside `{{.Tracker}}` — you can use both in the same template. The tracking pixel fires on email open, while the QR code fires on link click when scanned.
