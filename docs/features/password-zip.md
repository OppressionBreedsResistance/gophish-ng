# Password-Protected ZIP Attachments

Gophish-NG supports sending ZIP archives encrypted with **ZipCrypto** as email attachments, with placeholder substitution applied to files inside.

## How It Works

1. You create a ZIP file with ZipCrypto encryption containing your payload
2. Gophish-NG decrypts the archive on send, applies placeholder substitution to supported files, and re-encrypts before attaching to the email
3. Each recipient receives a personalized payload inside a password-protected archive

## Setup

### 1. Create the payload

Create your script with placeholders, e.g. `payload.ps1`:

```powershell
$url = "{{.URL}}"
$name = "{{.FirstName}}"
Invoke-WebRequest -Uri "$url/attachment?keyname={{.RId}}" -UseBasicParsing | Out-Null
```

### 2. Create a password-protected ZIP

Use **ZipCrypto** encryption (default in most tools):

=== "7-Zip GUI"
    1. Right-click the file → 7-Zip → Add to archive
    2. Archive format: `zip`
    3. Encryption method: `ZipCrypto`
    4. Enter password

=== "7-Zip CLI"
    ```bash
    7z a -tzip -p"yourpassword" archive.zip payload.ps1
    ```

!!! warning "Use ZipCrypto, not AES-256"
    Gophish-NG supports **ZipCrypto** encryption only. Do not use AES-256 (`-mem=AES256` in 7-Zip), as it is not supported and will result in a broken attachment.

### 3. Attach in Gophish-NG

1. Go to **Email Templates → New/Edit Template**
2. Attach the `.zip` file
3. A **Password** field appears next to the attachment — enter the ZIP password
4. Save the template

### 4. Result

Each recipient receives the `.zip` with personalized content inside, protected by the password you set.

!!! tip
    The password is the same for all recipients. The personalization happens inside the archive — each recipient's `.ps1` contains their unique `{{.RId}}` and other substituted values.
