# Merchant email worker

Merchant accounts must verify their email address before they can sign in or
appear in the platform onboarding console. Verification and welcome messages
are delivered from a durable PostgreSQL outbox by the backend worker.

## Google Workspace alias

1. Sign in to the Google Admin console.
2. Open **Directory > Users** and select the primary mailbox user.
3. Select **Add Alternate Emails** and add `hello@justloyalty.com`.
4. Sign in to Gmail as the primary user.
5. Open **Settings > Accounts and Import > Send mail as**.
6. Add `Just Loyalty <hello@justloyalty.com>` and complete the verification.

The alias receives mail in the primary inbox. It is not a separate Google
account and cannot be used as the SMTP username.

Google documentation:

- https://knowledge.workspace.google.com/admin/users/add-or-delete-an-alternate-email-address-email-alias
- https://support.google.com/mail/answer/22370

## App password

1. Allow and enable 2-Step Verification for the primary Workspace account.
2. Open https://myaccount.google.com/apppasswords while signed in as that user.
3. Create an app password named `Just Loyalty Email Worker`.
4. Save the displayed 16-character value. Google only shows it once.

The primary Workspace email is the SMTP username. The app password is the SMTP
password. Do not use the alias or the normal Google account password for SMTP
authentication.

On each deployment host, create the secret file without spaces:

```bash
umask 077
mkdir -p email-credentials
printf '%s' 'YOUR_16_CHARACTER_APP_PASSWORD' \
  > email-credentials/google-workspace-app-password
```

Generate the application secret used to derive verification PINs:

```bash
openssl rand -base64 32
```

Add the email settings to `.env` or `.env.uat`:

```dotenv
EMAIL_ENABLED=true
EMAIL_SMTP_HOST=smtp.gmail.com
EMAIL_SMTP_PORT=587
EMAIL_SMTP_USERNAME=gary@justloyalty.com
EMAIL_FROM_NAME=Just Loyalty
EMAIL_FROM_ADDRESS=hello@justloyalty.com
EMAIL_REPLY_TO=hello@justloyalty.com
EMAIL_VERIFICATION_SECRET=generated-secret-from-openssl
EMAIL_SMTP_PASSWORD_SOURCE_FILE=./email-credentials/google-workspace-app-password
```

The Compose file mounts the app-password file at
`/run/secrets/email_smtp_password`. The password and verification secret must
never be committed.

## Deploy

Apply migration `017` before starting the updated backend:

```bash
docker compose --env-file .env.uat -f docker-compose.uat.yml \
  --profile db-admin run --rm db-migrate

docker compose --env-file .env.uat -f docker-compose.uat.yml \
  up -d --build
```

For production, omit the UAT file and environment options.

## Verification

1. Register a test merchant.
2. Confirm the merchant has `pending_verification` status.
3. Confirm a `merchant_verification` row appears in `email_jobs`.
4. Enter the received six-digit code.
5. Confirm `email_verified_at` is populated, status is `active`, and one
   `merchant_welcome` job is sent.
6. In Gmail, use **Show original** on a received message and confirm SPF, DKIM,
   and DMARC pass for `justloyalty.com`.

If app passwords are unavailable, verify that 2-Step Verification is enabled
and that the account is not restricted to security keys or Advanced Protection.

Google SMTP documentation:

- https://knowledge.workspace.google.com/admin/gmail/send-email-from-a-printer-scanner-or-app
- https://support.google.com/accounts/answer/185833
