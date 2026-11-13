# Merchant email worker

Merchant accounts must verify their email address before they can sign in or
appear in the platform onboarding console. Verification and welcome messages
are delivered from a durable PostgreSQL outbox by the backend worker. Delivery
uses the Gmail API over HTTPS rather than SMTP.

## Google Workspace alias

1. Sign in to the Google Admin console.
2. Open **Directory > Users** and select the primary mailbox user.
3. Select **Add Alternate Emails** and add `hello@justloyalty.com`.
4. Sign in to Gmail as the primary user.
5. Open **Settings > Accounts and Import > Send mail as**.
6. Add `Just Loyalty <hello@justloyalty.com>` and complete verification.

The alias does not need to be the default sender. The service account
impersonates the primary mailbox while the email's `From` header uses the
verified alias.

## Gmail API service accounts

Create separate credentials for UAT and production in their respective Google
Cloud projects:

1. Enable the **Gmail API** under **APIs & Services > Library**.
2. Create `just-loyalty-uat-email` or `just-loyalty-prod-email` under
   **IAM & Admin > Service Accounts**.
3. Enable **Google Workspace Domain-Wide Delegation** for the service account.
4. Record its numeric OAuth client ID.
5. Create and download a JSON key. No Google Cloud project role is required.
6. In the Workspace Admin console, open **Security > Access and data control >
   API controls > Manage Domain Wide Delegation**.
7. Add the numeric client ID with only this OAuth scope:

```text
https://www.googleapis.com/auth/gmail.send
```

Store the downloaded key only on its deployment host:

```bash
umask 077
mkdir -p email-credentials
chmod 700 email-credentials
chmod 600 email-credentials/*-gmail-service-account.json
```

Generate the independent application secret used to derive verification PINs:

```bash
openssl rand -base64 32
```

Configure UAT with its own key:

```dotenv
EMAIL_ENABLED=true
GMAIL_API_USER=gary@justloyalty.com
GMAIL_API_CREDENTIALS_SOURCE_FILE=./email-credentials/uat-gmail-service-account.json
EMAIL_FROM_NAME=Just Loyalty
EMAIL_FROM_ADDRESS=hello@justloyalty.com
EMAIL_REPLY_TO=hello@justloyalty.com
EMAIL_VERIFICATION_SECRET=generated-secret-from-openssl
```

Production uses the same delegated mailbox and alias but points
`GMAIL_API_CREDENTIALS_SOURCE_FILE` to its separate production key. Compose
mounts the selected JSON as `/run/secrets/gmail_api_service_account`.

Google documentation:

- https://developers.google.com/workspace/gmail/api/guides/sending
- https://developers.google.com/workspace/gmail/api/auth/scopes
- https://developers.google.com/identity/protocols/oauth2/service-account

## Deploy

No database migration is required for the SMTP-to-Gmail API transport change.
Rebuild the UAT backend with:

```bash
docker compose --env-file .env.uat -f docker-compose.uat.yml \
  up -d --build backend
```

For production, omit the UAT file and environment options. Keep the old app
password until Gmail API delivery is verified, then revoke it from the Google
account and remove its local file.

## Verification

1. Register a new test merchant or request a new verification email.
2. Confirm the `merchant_verification` job in `email_jobs` becomes `sent`.
3. Enter the received six-digit code.
4. Confirm `email_verified_at` is populated, status is `active`, and the
   `merchant_welcome` job is sent.
5. In Gmail, use **Show original** and confirm the sender is
   `Just Loyalty <hello@justloyalty.com>` and SPF, DKIM, and DMARC pass.

Errors such as `unauthorized_client` usually mean domain-wide delegation is
missing or has the wrong client ID or scope. A `403` commonly means the Gmail
API is disabled or the delegated scope has not propagated. Alias-related send
errors mean `hello@justloyalty.com` is not configured as a verified send-as
address for the impersonated mailbox.
