# Email Templates

Email templates work the same as in upstream Gophish, with the addition of attachment support.

## Available Placeholders

| Placeholder | Description |
|-------------|-------------|
| `{{.FirstName}}` | Recipient's first name |
| `{{.LastName}}` | Recipient's last name |
| `{{.Position}}` | Recipient's position/title |
| `{{.Email}}` | Recipient's email address |
| `{{.From}}` | Sender address |
| `{{.URL}}` | Tracking/phishing URL |
| `{{.RId}}` | Unique recipient ID |

These placeholders work in the email body, subject, **and** inside supported attachment files (`.ps1`, `.bat`, `.pdf`, and files inside `.zip` archives).

## Tracking URL

The `{{.URL}}` placeholder expands to the phishing server URL with the recipient's unique identifier:

```
https://your-server.com/?keyname=RECIPIENT_ID
```

!!! note
    Gophish-NG uses `keyname` instead of `rid` as the URL parameter. Update any landing pages or external tooling accordingly.
