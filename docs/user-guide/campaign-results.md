# Campaign Results

The campaign results view has been extended with two additional status indicators.

## Status Levels

| Status | Color | Description |
|--------|-------|-------------|
| Email Sent | Green | Email successfully delivered |
| Email Opened | Yellow | Recipient opened the email |
| Clicked Link | Orange | Recipient clicked the phishing link |
| Submitted Data | Red | Recipient submitted data on the landing page |
| Clicked Attachment | Purple | Recipient executed the payload |
| Email Reported | Cyan | Recipient reported the email as suspicious |

## Clicked Attachment

Tracked when the payload beacons back to the phishing server. See [Attachment Tracking](../features/attachment-tracking.md).

## Email Reported

Displayed as a label in the results table when a recipient reports the email. This status does not affect the sequential event progression — it is an independent flag shown alongside the primary status.

## Donut Charts

The results page shows one donut chart per status. **Clicked Attachment** appears in purple, matching the status label color.
