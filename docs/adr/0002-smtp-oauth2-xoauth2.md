# SMTP authentication via app password

> **Superseded note:** This ADR originally documented a decision to use OAuth2 XOAUTH2. That implementation was reverted when Azure app registration for personal Outlook.com accounts required an Office 365 Management subscription to access the `SMTP.Send` permission. The implementation now uses an app password with `smtp.PlainAuth`. The original reasoning is preserved below for context.

---

# (Original) SMTP authentication via OAuth2 XOAUTH2, not app password

The alert engine sends email via Outlook.com SMTP. The account has MFA enabled, which requires either an app password or OAuth2 for programmatic SMTP access. We use OAuth2 with the XOAUTH2 SASL mechanism.

An app password grants broad account access if leaked and cannot be scoped. A refresh token obtained via OAuth2 device code flow is scoped to `https://outlook.office.com/SMTP.Send` only — if leaked, it can send email but cannot access calendar, contacts, or account settings, and can be revoked from Microsoft Entra without changing the account password.

The one-time setup cost is a `monitor oauth2-setup` CLI command (device code flow — prints a URL, user completes auth in a browser, command prints a refresh token to store in `.env`). At runtime, `golang.org/x/oauth2` auto-refreshes the short-lived access token transparently. The XOAUTH2 SASL mechanism is ~15 lines implementing `net/smtp.Auth`.

## Considered options

- **App password + PlainAuth** — simpler, but full account exposure on leak; Microsoft is also deprecating Basic Auth for SMTP on Exchange Online (does not yet apply to personal Outlook.com accounts, but trajectory is clear)
- **OAuth2 XOAUTH2** — chosen: scope-limited, revocable, aligns with Microsoft's authentication direction
