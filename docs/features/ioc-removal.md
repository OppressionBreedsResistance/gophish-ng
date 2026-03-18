# IOC Removal

Gophish-NG removes or replaces indicators of compromise (IOCs) that are present in the original Gophish and can be used to fingerprint the framework.

## Changes

| What | Original value | New value |
|------|----------------|-----------|
| Email header | `X-Gophish-Contact` | `X-Contact` |
| Webhook header | `X-Gophish-Signature` | `X-Signature` |
| Server name / X-Mailer | `gophish` | *(omitted)* |
| Recipient URL parameter | `rid` | `keyname` |
| 404 response | Go default page | Custom page |

## URL Parameter Change

Tracking links use `?keyname=...` instead of `?rid=...`.

!!! warning
    If you are using custom landing pages or external tooling that reads the `rid` parameter, update them to use `keyname` instead.
