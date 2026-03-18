![gophish-ng logo](static/images/gophish-ng_logo.png)

# Gophish-NG

Gophish-NG is a fork of the open-source [Gophish](https://github.com/gophish/gophish) phishing toolkit, extended with additional capabilities for red team engagements.

## Building From Source

**Requires Go v1.10 or above.**

```bash
git clone https://github.com/gophish/gophish-ng.git
cd gophish-ng
go build
```

## Setup

Run the binary and open a browser at `https://localhost:3333`. Login credentials are printed on first run:

```
time="2020-07-29T01:24:08Z" level=info msg="Please login with the username admin and the password 4304d5255378177d"
```

## Modifications

This fork includes the following changes on top of the upstream Gophish codebase:

### Attachment Template Support

- **`.ps1` and `.bat` files** — PowerShell and batch script attachments support placeholder substitution (`{{.URL}}`, `{{.FirstName}}`, etc.), the same way `.txt` and `.html` files do.
- **`.pdf` files** — PDF attachments also support placeholder substitution. Note: this works only if the placeholder text is stored as **plain text** in the PDF content stream. PDFs using compressed streams (zlib/deflate) will not be processed correctly and may become corrupted. For best results, export PDFs from tools that do not compress text streams (e.g. Word → Export to PDF with default settings).
- **`.zip` files containing `.ps1`, `.bat`, or `.pdf`** — When a `.zip` archive is used as an attachment, Gophish-NG unpacks it in memory, applies template substitution to any `.ps1`, `.bat`, `.pdf` (and `.xml`/`.rels`) files inside, and repacks it before sending.
- **Password-protected `.zip` attachments** — ZIP archives encrypted with ZipCrypto are fully supported. Gophish-NG decrypts the archive, applies placeholder substitution, and re-encrypts before sending. The password is stored per-attachment in the database and can be set in the template UI.

#### How to use password-protected ZIP attachments

1. Create your payload script, e.g. `payload.ps1`, with any placeholders:
   ```powershell
   $url = "{{.URL}}"
   $name = "{{.FirstName}}"
   ```
2. Compress it into a password-protected ZIP using ZipCrypto encryption (default in most tools, including 7-Zip without `-mem=AES256`).
3. In Gophish-NG, go to **Email Templates → New/Edit Template** and attach the `.zip` file.
4. A **Password** field appears in the attachment row — enter the ZIP password there.
5. Save the template. Each recipient will receive a `.zip` with a personalized `.ps1` inside, protected by the same password.

---

### QR Code Placeholder

Use `{{.QR}}` in any email template to embed a per-recipient QR code that links to the phishing URL.

- Generated server-side and embedded as an inline image (CID) — no external requests needed.
- Each recipient gets a unique QR code pointing to their personalized phishing URL (with `keyname` parameter).
- Available in the CKEditor autocomplete dropdown.

Example:
```html
<p>Scan the QR code below to access the document:</p>
{{.QR}}
```

---

### Attachment Click Tracking

A new event type **"Clicked Attachment"** tracks when a recipient executes the delivered payload.

- The payload script should beacon back to `{{.URL}}/attachment?keyname={{.RId}}` on execution.
- Gophish-NG records a **Clicked Attachment** event, visible in the campaign results table and donut chart (purple).
- If the email had not been marked as opened yet, the open event is automatically inferred.

Example beacon in PowerShell:
```powershell
Invoke-WebRequest -Uri "{{.URL}}/attachment?keyname={{.RId}}" -UseBasicParsing | Out-Null
```

---

### IOC Removal

The following Gophish-specific indicators of compromise have been removed or replaced:

| What | Original value | New value |
|------|----------------|-----------|
| Email header | `X-Gophish-Contact` | `X-Contact` |
| Webhook header | `X-Gophish-Signature` | `X-Signature` |
| Server name / X-Mailer | `gophish` | *(omitted)* |
| Recipient URL parameter | `rid` | `keyname` |
| 404 response | Go default | Custom page |

> **Note:** Tracking links use `?keyname=...` instead of `?rid=...`. Update landing pages and any external tooling accordingly.

---

### Campaign Results Enhancements

- **Email Reported** — displayed as a status label in the results table when a recipient reports the email, without affecting the sequential event progression.
- **Clicked Attachment** — displayed as a 5th status level (purple) in both the results table and the donut chart.

---

## License

```
Gophish - Open-Source Phishing Framework

The MIT License (MIT)

Copyright (c) 2013 - 2020 Jordan Wright

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software ("Gophish Community Edition") and associated documentation
files (the "Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish, distribute,
sublicense, and/or sell copies of the Software, and to permit persons to whom
the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
