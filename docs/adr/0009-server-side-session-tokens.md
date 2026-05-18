# Server-side session tokens, not JWT

Authentication uses server-side session tokens: a 256-bit random token stored in the `sessions` table, delivered as an `HttpOnly; SameSite=Strict` cookie. Each request looks up the token in the DB. Sessions expire after 30 days.

The main alternative — JWT — is stateless (no DB lookup) but cannot be invalidated before expiry without a server-side blocklist, which eliminates the stateless advantage. On a home monitoring system where the admin may want to immediately revoke access (e.g., after recognizing a compromised device), hard expiry via `DELETE FROM sessions` is a meaningful property. The `HttpOnly` cookie also prevents the token from being read by JavaScript, which matters if third-party scripts or browser extensions are ever present — a concern for any system with a web dashboard. The 30-day lifetime reflects the usage pattern: a household member checking the dashboard from a phone or tablet should not need to re-authenticate weekly. It is long but the exposure window is bounded by `SameSite=Strict`, which prevents cross-site request forgery, and by the ability to immediately delete specific session rows.

## Considered options

- **JWT with short-lived access tokens + refresh tokens** — rejected: two-token management adds complexity (refresh endpoint, token rotation, clock skew handling) for no benefit over a simple DB lookup; still requires server-side state for refresh token revocation.
- **No authentication** — rejected: the dashboard exposes real-time home occupancy data and controls alert preferences; access must be gated even on a LAN.
