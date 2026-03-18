# Attachment Tracking

Gophish-NG can track when a recipient executes the delivered payload by beaconing back to the phishing server.

## How It Works

1. The payload script sends an HTTP request to `/attachment?keyname={{.RId}}` on execution
2. Gophish-NG records a **Clicked Attachment** event for that recipient
3. If the email had not been marked as opened yet, the open event is automatically inferred

## Setting Up the Beacon

Add the following to your PowerShell payload:

```powershell
Invoke-WebRequest -Uri "{{.URL}}/attachment?keyname={{.RId}}" -UseBasicParsing | Out-Null
```

For batch scripts:

```bat
powershell -Command "Invoke-WebRequest -Uri '{{.URL}}/attachment?keyname={{.RId}}' -UseBasicParsing | Out-Null"
```

## Results

The **Clicked Attachment** event appears:

- As a purple status label in the campaign results table
- As a purple donut chart in the campaign results view
- In the event timeline for the recipient

!!! tip
    The beacon URL uses `{{.URL}}` which already includes the phishing server address. The `{{.RId}}` placeholder is the unique recipient identifier automatically substituted by Gophish-NG.
