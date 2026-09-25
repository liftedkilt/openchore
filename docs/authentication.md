# Signing in

OpenChore is built for a shared tablet on the wall first and personal phones
second. Sign-in follows that: **tap your profile**, then prove it's you only
if the profile asks for it.

| Profile has… | Tapping it… |
|---|---|
| Nothing (typical for young kids) | Signs straight in |
| A PIN | Shows the PIN pad |
| A linked account (Pocket ID, Google, …) | Shows **Continue with …** |
| Both | Shows the PIN pad *and* **Continue with …** |

Parents are ordinary profiles with the admin role. They sign in the same way
and get a **Manage** button on their own dashboard; there is no separate
household passcode. Parents take part like everyone else: they can be
assigned chores, earn points, redeem rewards and pick a theme.

Every **parent profile must have a PIN or a linked account**. The server
refuses to create a parent without a PIN, to promote someone without one, or
to remove a parent's last way to sign in.

## Sessions

Signing in gets you a server-issued, signed session in an `HttpOnly` cookie.
The API does not trust a client-supplied user ID.

| Signed in with | Lasts | On idle |
|---|---|---|
| Tap or PIN | 12 hours | Signed out after 5 minutes idle; the tablet returns to the wall display |
| Linked account (OIDC) | 30 days | Stays signed in, so your phone just opens to your chores |

Change these under **Manage → Settings → Sign-in → Session length**. New
sign-ins use the new length. `auth.kiosk_session_ttl` and
`auth.personal_session_ttl` in `config.yaml` (e.g. `12h`, `720h`) take
precedence and make the fields read-only.

**Sign out on all devices** (in *Linked accounts*) revokes every session for
your profile. Unlinking an account or changing someone's role does the same
for that person.

Five wrong PINs in a row lock that profile for 30 seconds, and the lockout
doubles with each further miss (up to 15 minutes).

The wall display (`/ambient`) needs no sign-in. The per-person chores, points
and streak endpoints it reads are public and read-only.

## Linking an account (OIDC)

OpenChore works with any OpenID Connect provider: Pocket ID, Authelia,
Authentik, Keycloak, Zitadel, Kanidm, Google, Microsoft and so on. You can
configure several at once.

Accounts are never created from a provider. Each person:

1. Signs in to their profile as usual (tap or PIN).
2. Opens **Linked accounts** (the link icon on their dashboard) and chooses
   **Link Pocket ID**, or whichever provider you set up.
3. Signs in at the provider and is sent back, linked.

From then on, tapping their profile offers **Continue with Pocket ID**, and on
their own phone they stay signed in. If someone taps a profile and then signs
in at the provider as somebody else, the sign-in is refused.

Parents can see and unlink anyone's linked accounts from **Manage → People**.
Only the person themselves can link a new one.

### Configuring a provider

Register OpenChore as an OIDC client at your provider:

- **Redirect URI:** `https://<your OpenChore URL>/api/auth/oidc/<id>/callback`
- **Scopes:** `openid profile email`
- **Client type:** confidential (client secret). PKCE (S256) is always used as well.

Then add it under **Manage → Settings → Sign-in → Add provider**: a button
label, an ID (it becomes part of the redirect URI, which the form shows you,
and can't be changed later), the issuer URL, the client ID and secret, and
whether to ask people to sign in every time. It's available on the login
screen straight away. **Test** checks that OpenChore can reach the issuer.

Client secrets are write-only: editing a provider shows that a secret is
saved, and leaving the field blank keeps it. A provider can't be removed while
someone has no PIN and no other linked account, since they'd be locked out;
give them a PIN first. Removing a provider keeps people's links, so adding it
back with the same ID restores them.

Changing a provider's issuer disconnects the accounts linked through it
(account IDs are specific to an issuer), so people need to link again.

#### In config.yaml or the environment

Providers can also live in `config/config.yaml`. Unlike the seed sections,
the `auth` section is read on **every** start, so you can add it to an
existing install and restart. These providers appear under Settings marked
*config.yaml* and are read-only there; one with the same ID as a provider
added in Settings takes its place.

```yaml
auth:
  public_url: "https://chores.example.com"   # used to build the redirect URI
  oidc:
    - id: pocketid             # appears in the callback URL; don't change it once people have linked
      name: "Pocket ID"        # button label: "Continue with Pocket ID"
      issuer: "https://id.example.com"
      client_id: "openchore"
      client_secret: "${POCKET_ID_CLIENT_SECRET}"   # ${VAR} expands from the environment
      prompt: login            # optional: re-authenticate every time (recommended on shared tablets)
```

Or configure a single provider with environment variables only:

| Variable | Meaning |
|---|---|
| `OIDC_ISSUER` | Issuer URL (enables the provider) |
| `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | Client credentials |
| `OIDC_PROVIDER_ID` | Provider ID, default `oidc` |
| `OIDC_PROVIDER_NAME` | Button label, default `Single sign-on` |
| `OIDC_SCOPES` | Space- or comma-separated, default `openid profile email` |
| `OIDC_PROMPT` | Passed through as the `prompt` parameter, e.g. `login` |
| `OPENCHORE_PUBLIC_URL` | Overrides `auth.public_url` |
| `OPENCHORE_SESSION_SECRET` | Optional; at least 32 characters. Otherwise a secret is generated and stored in the database |

**Set `public_url`** (or the *System Base URL* under Manage → Settings) when
OpenChore sits behind a reverse proxy. Otherwise the redirect URI is guessed
from the request's `Host` and `X-Forwarded-Proto` headers, which proxies don't
always pass through.

Browser requests that change something are also checked against their
`Origin`. This stops another app on a sibling subdomain (say
`other.home.lan`) from acting with your cookie. If OpenChore sits behind a
proxy that rewrites `Host` and you see "cross-origin request blocked", set
`public_url`.

If your provider uses a certificate from a private CA, point `SSL_CERT_FILE`
at a bundle that includes it.

### Pocket ID

In Pocket ID, go to **OIDC Clients → Add OIDC Client**, set the callback URL
above, and copy the client ID and secret into the config. Pocket ID signs in
with passkeys. On a shared wall tablet that usually means scanning a QR code
with your phone, so parents often keep a PIN for the tablet and use Pocket ID
on their own phones. Setting `prompt: login` makes Pocket ID ask every time
rather than silently reusing a session left open on the tablet.

### Google and Microsoft

Both are standard OIDC providers:

| Provider | Issuer |
|---|---|
| Google | `https://accounts.google.com` |
| Microsoft (one tenant) | `https://login.microsoftonline.com/<tenant-id>/v2.0` |
| Microsoft (personal accounts) | `https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0` |

Microsoft's multi-tenant `common` and `organizations` endpoints don't publish
a fixed issuer, so they aren't supported. Use a tenant-specific issuer.

### "Social login" with GitHub, Discord, Apple and others

GitHub, Discord and Facebook use plain OAuth 2.0, not OpenID Connect, so
OpenChore can't talk to them directly. If you want them, put an identity
broker in front: Authentik, Keycloak, Zitadel or Dex can each sign people in
with those services and present them to OpenChore as a single OIDC provider.
Pocket ID and Authelia don't federate upstream logins.

## Upgrading from the household passcode

Earlier versions had one shared admin passcode and no real sign-in. After
upgrading:

- **Parent profiles that already have a PIN** just use it.
- **Parent profiles without a PIN** are asked, the first time they're tapped,
  for the old admin passcode (default `0000`) and a new PIN of their own. Once
  every parent has a PIN or a linked account, the old passcode is deleted.
- **Kids** are unaffected: no PIN means tap and go, same as before.
- **API tokens** keep working unchanged.
- **Scripts that sent `X-User-ID`** must switch to an API token
  (Manage → Settings → API Tokens) or sign in with `POST /api/auth/login` and
  send the returned session as `Authorization: Bearer ocs1.…`.
- The `auth.admin_passcode.*` webhook events now fire only for that one-time
  upgrade step. Use `auth.profile_pin.*`, `auth.oidc.login` and
  `auth.identity.*` for sign-in auditing.

Photo QR codes now carry a short-lived token that can only upload a photo for
that one chore. The phone that scans the code doesn't need to be signed in.
