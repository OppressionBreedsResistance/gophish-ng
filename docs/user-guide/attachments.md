# Attachments

Gophish-NG extends attachment support with placeholder substitution and password-protected ZIP files.

## Supported File Types

Extensions are matched case-insensitively, and the same list applies to a standalone
attachment and to a file found inside a `.zip`, so a payload behaves identically whether or
not it is zipped.

| Extension | Placeholder Substitution | Notes |
|-----------|--------------------------|-------|
| `.txt` | Yes | Plain text |
| `.html`, `.htm` | Yes | HTML files |
| `.ics` | Yes | Calendar invitations |
| `.ps1` | Yes | PowerShell scripts |
| `.bat` | Yes | Batch scripts |
| `.js`, `.vbs`, `.hta` | Yes | Script payloads |
| `.xml`, `.rels` | Yes | Raw XML parts |
| `.pdf` | Yes* | *Plain-text streams only — see note below |
| `.docx`, `.docm`, `.pptx`, `.xlsx`, `.xlsm` | Yes (contents) | Unpacked, `.xml`/`.rels` parts substituted, repacked |
| `.zip` | Yes (contents) | Unpacked, substituted, repacked |
| anything else | No | Delivered byte-for-byte |

!!! warning "Substitution is unconditional"
    Every file with one of the extensions above is run through the Go template engine. A `.js`
    payload that legitimately contains `{{` will fail to parse and the attachment will be
    rejected — escape it, or rename the file to an extension that is not templated.

!!! warning "PDF Limitation"
    PDF substitution works only if the placeholder text is stored as **plain text** in the PDF content stream. PDFs with compressed streams (zlib/deflate) will not be processed correctly and may become corrupted.

    For best results, export PDFs from Word using default settings, which typically does not compress text streams.

## ZIP Attachments

When a `.zip` file is used as an attachment, Gophish-NG:

1. Decrypts the archive (if password-protected)
2. Applies template substitution to every supported file type inside (see the table above)
3. Repacks and re-encrypts the archive before sending

Files inside the archive that are not on that list — images, binaries, nested Office
documents — are repacked byte-for-byte.

See [Password-Protected ZIP](../features/password-zip.md) for full setup instructions.

## Attachment Click Tracking

Gophish-NG can track when a recipient executes the delivered payload (e.g. runs a `.ps1` script). To enable tracking, the payload must beacon back to the phishing server. Use the `{{.Attachment}}` placeholder, which expands to a per-recipient URL of the form:

```
<base URL>/attachment?keyname=<RId>
```

for example `http://example.com/attachment?keyname=1234567`.

!!! danger "Do not use `{{.URL}}/attachment?keyname={{.RId}}`"
    Earlier revisions of this page documented that form. It does not work. `{{.URL}}` is the
    full phishing URL and already carries the path **and** the `keyname` query string, so the
    expression expands to something like
    `http://example.com?keyname=1234567/attachment?keyname=1234567`. The `/attachment` segment
    ends up inside the query string, the request never reaches the attachment handler, and the
    recipient ID no longer resolves — you get a 404 and no **Clicked Attachment** event.

    Use `{{.Attachment}}`, or build the URL by hand as `{{.BaseURL}}/attachment?keyname={{.RId}}`
    (`{{.BaseURL}}` has the path and query stripped).

### PowerShell example

```powershell
Invoke-WebRequest -Uri "{{.Attachment}}" -UseBasicParsing | Out-Null
```

### Batch example

```bat
powershell -Command "Invoke-WebRequest -Uri '{{.Attachment}}' -UseBasicParsing | Out-Null"
```

When the beacon request is received, Gophish-NG records a **Clicked Attachment** event for the recipient. If the email had not been marked as opened yet, an **Email Opened** event is also automatically inferred.

See [Attachment Tracking](../features/attachment-tracking.md) for more details.

## Adding an Attachment

1. Go to **Email Templates → New/Edit Template**
2. Scroll to the **Attachments** section
3. Click **Add Files** and select your file
4. For ZIP files, a **Password** field appears — enter the ZIP password if applicable
5. Save the template
