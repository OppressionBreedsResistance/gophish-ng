# SMS Campaigns (Smishing)

Gophish-NG can run **smishing** (SMS phishing) campaigns. Sending the text
messages is left to you — Gophish-NG generates a unique tracking link for every
recipient and exports them to CSV. You send the SMS from your own gateway, and
Gophish-NG tracks the clicks and any data submitted to the landing page, exactly
as it does for email phishing.

## Why This Design

Click and form-submission tracking in Gophish is keyed on the recipient's unique
`keyname` parameter in the URL — it is completely independent of how the message
was delivered. That means the only thing an SMS campaign has to do differently is
**skip the mailer and hand you the per-recipient links**. Everything downstream
(landing page, click event, submitted-data event, timeline, results) works
without any changes.

Email open tracking (the invisible pixel) does not apply to SMS and is simply not
used — clicks and form submits are what you get.

## How It Works

1. Create a campaign and set **Campaign Type** to **SMS (Smishing)**.
2. An SMS campaign requires only a **Landing Page** and a **URL**. The email
   template and sending profile are ignored — no email is ever sent.
3. On launch, Gophish-NG creates a result (with a unique tracking ID) for every
   recipient but **does not queue any messages** for delivery.
4. Open the campaign's **Results** page and export the tracking links to CSV.
5. Send the SMS messages yourself, one personalized link per recipient.
6. Clicks and form submissions are recorded in the campaign timeline just like an
   email campaign.

## Recipients and Phone Numbers

Phone numbers live on the group's targets, next to the e-mail address.

- Add them in the **Users & Groups** editor (the **Phone** column), or import a
  CSV that includes a `Phone` column.
- Always use the **international format with a country code**, e.g.
  `+48500000000`. The number is exported verbatim, so whatever you enter is what
  your SMS gateway receives.
- An **e-mail address is still required** for every recipient (it is used to
  de-duplicate the list). If you only have phone numbers, use a placeholder
  address such as `first.last@example.invalid`.

### CSV import format

```csv
First Name,Last Name,Email,Position,Phone
Jan,Kowalski,jan.kowalski@example.com,Accountant,+48500000001
Anna,Nowak,anna.nowak@example.com,HR,+48500000002
```

The `Phone` column header is matched case-insensitively (also `mobile`,
`telefon`).

## Exporting the Tracking Links

On the campaign **Results** page, open the **Export** menu and choose
**Smishing CSV (with phone & tracking URLs)**. The file contains:

```csv
first_name,last_name,email,phone,tracking_url
Jan,Kowalski,jan.kowalski@example.com,+48500000001,https://phish.example.com/?keyname=aBcD123
```

Feed `phone` and `tracking_url` into your SMS gateway. When a recipient opens the
link, Gophish-NG records a **Clicked Link** event; if they submit the landing-page
form, it records a **Submitted Data** event.

## What to Prepare

| Item | Needed for SMS? |
|------|-----------------|
| Landing Page | **Yes** — this is what tracks clicks and captures form data |
| Campaign URL | **Yes** — the per-recipient link is built from it |
| Email Template | No — ignored |
| Sending Profile (SMTP) | No — ignored |
| Phone numbers on targets | Yes — with country code |
| E-mail on targets | Yes — required (may be a placeholder) |

!!! note
    A scheduled SMS campaign (launch date in the future) stays in the **Queued**
    state until its launch time — the worker intentionally never sends it. For an
    immediate launch it shows as **In progress**. Either way the CSV export and
    click/submit tracking work regardless of the campaign status.
