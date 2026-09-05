# Attachment Tracking

Gophish-NG can track when a recipient executes the delivered payload by beaconing back to the phishing server.

## How It Works

1. The payload script sends an HTTP request to `{{.Attachment}}` on execution — the placeholder expands to `<base URL>/attachment?keyname=<RId>`, unique per recipient
2. Gophish-NG records a **Clicked Attachment** event for that recipient
3. If the email had not been marked as opened yet, the open event is automatically inferred

## Setting Up the Beacon

Add the following to your PowerShell payload:

```powershell
Invoke-WebRequest -Uri "{{.Attachment}}" -UseBasicParsing | Out-Null
```

For batch scripts:

```bat
powershell -Command "Invoke-WebRequest -Uri '{{.Attachment}}' -UseBasicParsing | Out-Null"
```

!!! danger "Do not use `{{.URL}}/attachment?keyname={{.RId}}`"
    Earlier revisions of this page documented that form. It does not work. `{{.URL}}` is the
    full phishing URL and already carries the path **and** the `keyname` query string, so the
    expression expands to something like
    `http://example.com?keyname=1234567/attachment?keyname=1234567`. The `/attachment` segment
    ends up inside the query string, the request is routed to the landing page handler instead
    of the attachment handler, and the recipient ID no longer resolves — you get a 404 and no
    **Clicked Attachment** event.

    Use `{{.Attachment}}`, or build the URL by hand as `{{.BaseURL}}/attachment?keyname={{.RId}}`
    (`{{.BaseURL}}` has the path and query stripped).

## Results

The **Clicked Attachment** event appears:

- As a purple status label in the campaign results table
- As a purple donut chart in the campaign results view
- In the event timeline for the recipient

!!! tip
    `{{.Attachment}}` already includes the phishing server address and the unique recipient
    identifier, so nothing else needs to be appended to it. It is available in every templated
    context — email bodies, landing pages and attachments — and appears in the CKEditor
    autocomplete dropdown.
