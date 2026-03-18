# Attachments

Gophish-NG extends attachment support with placeholder substitution and password-protected ZIP files.

## Supported File Types

| Extension | Placeholder Substitution | Notes |
|-----------|--------------------------|-------|
| `.txt` | Yes | Plain text |
| `.html` | Yes | HTML files |
| `.ps1` | Yes | PowerShell scripts |
| `.bat` | Yes | Batch scripts |
| `.pdf` | Yes* | *Plain-text streams only — see note below |
| `.zip` | Yes (contents) | Unpacked, substituted, repacked |

!!! warning "PDF Limitation"
    PDF substitution works only if the placeholder text is stored as **plain text** in the PDF content stream. PDFs with compressed streams (zlib/deflate) will not be processed correctly and may become corrupted.

    For best results, export PDFs from Word using default settings, which typically does not compress text streams.

## ZIP Attachments

When a `.zip` file is used as an attachment, Gophish-NG:

1. Decrypts the archive (if password-protected)
2. Applies template substitution to supported file types inside
3. Repacks and re-encrypts the archive before sending

See [Password-Protected ZIP](../features/password-zip.md) for full setup instructions.

## Adding an Attachment

1. Go to **Email Templates → New/Edit Template**
2. Scroll to the **Attachments** section
3. Click **Add Files** and select your file
4. For ZIP files, a **Password** field appears — enter the ZIP password if applicable
5. Save the template
