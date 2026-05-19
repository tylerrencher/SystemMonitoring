# SMTP via Gmail + app password (AUTH LOGIN)

The alert engine sends email via Gmail SMTP with an app password.

Outlook.com was the original provider. Two successive auth approaches were attempted and rejected by Microsoft:
1. `smtp.PlainAuth` (AUTH PLAIN) — rejected with `504 5.7.4 Unrecognized authentication type`
2. `loginAuth` (AUTH LOGIN) — rejected with `535 5.7.139 Authentication unsuccessful, basic authentication is disabled`

Microsoft has disabled all basic auth for personal Outlook.com accounts. XOAUTH2 was the principled fix but requires an Azure app registration scoped to `SMTP.Send`, which is non-trivial. Gmail was chosen instead — it still supports SMTP with app passwords when 2FA is enabled, and setup is minutes not hours.

The implementation uses a custom `loginAuth` struct implementing `net/smtp.Auth` with the AUTH LOGIN mechanism. Gmail accepts AUTH LOGIN after STARTTLS on port 587.

## Considered options

- **Outlook + XOAUTH2** — correct long-term answer for Outlook; rejected due to setup complexity (Entra app registration, device code flow CLI, token refresh logic)
- **Gmail + app password** — chosen: works today, minimal config (`SMTP_HOST`, `SMTP_USERNAME`, `SMTP_PASSWORD`), revocable app password scoped to one device
