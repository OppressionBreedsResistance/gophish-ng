# Configuration

Gophish-NG uses a `config.json` file in the root directory.

```json
{
  "admin_server": {
    "listen_url": "0.0.0.0:3333",
    "use_tls": true,
    "cert_path": "gophish_admin.crt",
    "key_path": "gophish_admin.key"
  },
  "phish_server": {
    "listen_url": "0.0.0.0:80",
    "use_tls": false
  },
  "db_name": "sqlite3",
  "db_path": "gophish.db",
  "migrations_prefix": "db/db_",
  "contact_address": "",
  "logging": {
    "filename": "",
    "level": ""
  },
  "turnstile": {
    "site_key": "",
    "secret_key": ""
  }
}
```

## Key Settings

| Key | Description |
|-----|-------------|
| `admin_server.listen_url` | Address and port for the admin panel |
| `phish_server.listen_url` | Address and port for the phishing server (landing pages, tracking) |
| `db_name` | Database driver: `sqlite3` or `mysql` |
| `db_path` | Path to the SQLite database file |
| `contact_address` | Email address embedded in outgoing emails as `X-Contact` header |
| `turnstile.site_key` | Cloudflare Turnstile site key — leave empty to disable bot protection |
| `turnstile.secret_key` | Cloudflare Turnstile secret key — leave empty to disable bot protection |
