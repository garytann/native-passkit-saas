# Native iOS PassKit WIP

Go proof of concept for issuing Apple Wallet F&B loyalty stamp cards without the PassKit.com SDK.

This project uses:

- Go for the backend API and pass package generation.
- Apple Wallet pass package format: `pass.json`, images, `manifest.json`, `signature`, ZIP.
- OpenSSL for the PKCS #7 detached signature Apple Wallet requires.
- Native iOS `PassKit.framework` only on the client side, to present the add-to-Wallet UI.

## Current Routes

Routes are mounted through a single API version group in `internal/httpapi/routes.go`. The current public API prefix is:

```text
/api/v1
```

- `GET /api/v1/healthz` - service health check.
- `POST /api/v1/merchants/register` - registers a SaaS merchant account.
- `POST /api/v1/merchants/login` - validates merchant email/password and returns a merchant token.
- `GET /api/v1/customers/members` - returns members scoped to the authenticated merchant.
- `GET /api/v1/customers/members/{memberID}` - loads a merchant-scoped member and reward programme state.
- `POST /api/v1/public/merchants/{merchantSlug}/members/register` - registers a customer from a merchant-owned public signup link.
- `PUT /api/v1/customers/members/{memberID}/stamps` - updates stamp progress for a member owned by the authenticated merchant.
- `POST /api/v1/customers/members/{memberID}/rewards/issue` - converts a completed stamp card into an available reward.
- `POST /api/v1/customers/members/{memberID}/rewards/redeem` - redeems one available reward.
- `GET /api/v1/apple/passes/example` - returns a sample pass URL.
- `GET /api/v1/apple/passes/{memberID}/preview` - renders an unsigned browser preview.
- `GET /api/v1/apple/passes/{memberID}/qr.png` - renders the preview QR code as a PNG.
- `GET /api/v1/apple/passes/{memberID}.pkpass` - generates and returns a signed Apple Wallet stamp card.
- `GET /api/v1/google-wallet/passes/{memberID}/save` - lazily provisions a Google Wallet loyalty object and redirects to Google's signed save URL.
- `GET /api/v1/merchants/me/loyalty-program` - returns the authenticated merchant's shared Apple/Google loyalty programme.
- `PUT /api/v1/merchants/me/loyalty-program` - publishes shared programme content and queues active wallet updates.

## Backend Architecture

The backend follows a small controller-service-repository shape while keeping Go packages under `internal`, as recommended for server implementations that are not intended to expose reusable public packages.

- `cmd/server` - binary entry point and dependency wiring.
- `internal/httpapi` - HTTP server assembly, middleware, and versioned route grouping.
- `internal/controller` - HTTP controllers for request parsing and response writing.
- `internal/service` - business logic for member registration and pass generation.
- `internal/repository` - GORM/Postgres persistence.
- `internal/domain` - core domain models and domain-level helpers.

When adding new API features, prefer this flow:

```text
controller -> service -> repository
```

Controllers should parse requests and write HTTP responses. Services should validate business inputs, compose domain behavior, and call repositories. Repositories should stay focused on database reads/writes.

## Apple Credentials Required

To install a generated pass into Apple Wallet, you need Apple Developer Program credentials:

- Pass Type ID, for example `pass.com.yourcompany.membership`
- Team ID
- Pass Type ID certificate
- Private key associated with the certificate request
- Apple WWDR intermediate certificate

The code can compile without those files, but `/api/v1/apple/passes/{memberID}.pkpass` cannot produce an installable pass until the certs are present.

## Configure

Copy `.env.example` to `.env` for local development:

```sh
cp .env.example .env
```

Docker Compose reads `.env` automatically for container configuration, including database credentials, exposed host ports, auth token secret, and Apple pass settings. The backend also reads `.env` automatically when you run it directly with `go run`; real environment variables still take precedence, which is useful for deployment.

You can also export values directly:

```sh
export PORT=8080
export PUBLIC_BASE_URL=http://localhost
export AUTH_TOKEN_SECRET=change-me-for-local-dev
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/passkit_wip?sslmode=disable"
export APPLE_PASS_TYPE_ID=pass.com.yourcompany.membership
export APPLE_TEAM_ID=YOURTEAMID
export APPLE_ORGANIZATION_NAME="Your Company"
export APPLE_PASS_CERT_PATH=certs/pass-cert.pem
export APPLE_PASS_KEY_PATH=certs/pass-key.pem
export APPLE_WWDR_CERT_PATH=certs/wwdr.pem
```

Put local PEM files in `certs/`. Do not commit real certificates or private keys.

## Database

This project uses Postgres for member registration. The Go database layer uses GORM with the Postgres driver to build and execute SQL.

The backend does not create the database and does not run schema migrations during server startup. It only opens the configured `DATABASE_URL` and pings the database. This keeps database lifecycle concerns outside the application process, which is the safer model for UAT and production.

For Docker deployments, database lifecycle is split into explicit one-off jobs:

- `db` starts Postgres and checks the admin database only.
- `db-init` creates the application database once if it does not already exist.
- `db-migrate` applies SQL files from `migrations/` to the application database.

Run these once when provisioning a new production environment:

```sh
docker compose up -d db
docker compose --profile db-admin run --rm db-init
docker compose --profile db-admin run --rm db-migrate
docker compose up -d --build
```

For a new UAT environment, select the UAT Compose entry point and environment
file explicitly:

```sh
docker compose --env-file .env.uat -f docker-compose.uat.yml up -d db
docker compose --env-file .env.uat -f docker-compose.uat.yml --profile db-admin run --rm db-init
docker compose --env-file .env.uat -f docker-compose.uat.yml --profile db-admin run --rm db-migrate
docker compose --env-file .env.uat -f docker-compose.uat.yml up -d --build
```

For normal production deploys after the database already exists:

```sh
docker compose up -d --build
```

For normal UAT deploys:

```sh
docker compose --env-file .env.uat -f docker-compose.uat.yml up -d --build
```

When switching an existing UAT server from the former combined stack, append
`--remove-orphans` once so Docker removes the old landing container. This does
not remove the Postgres volume:

```sh
docker compose --env-file .env.uat -f docker-compose.uat.yml up -d --build --remove-orphans
```

When a future production release includes schema changes, run the migration
job deliberately during the deployment:

```sh
docker compose --profile db-admin run --rm db-migrate
docker compose up -d --build
```

For a UAT release with schema changes:

```sh
docker compose --env-file .env.uat -f docker-compose.uat.yml --profile db-admin run --rm db-migrate
docker compose --env-file .env.uat -f docker-compose.uat.yml up -d --build
```

Do not use `docker compose down -v` in UAT/prod unless you intentionally want to destroy database data.

For local development without Docker, create the database first and apply the SQL migrations manually:

```sh
createdb passkit_wip
for file in migrations/*.sql; do psql "$DATABASE_URL" -f "$file"; done
```

The migration creates `store_members` with:

- `member_id`
- `creation_date`
- `customer_name`
- `customer_email`
- `merchant`
- `stamp_progress`
- `available_rewards`
- `status`

`creation_date`, `stamp_progress`, and `status` are Postgres defaults. The API generates `member_id` as a readable support-friendly ID, for example `MBR-8K4J2QPA`, instead of exposing a long UUID to customers.

The second migration creates `merchants` for SaaS merchant onboarding:

- `merchant_id`
- `creation_date`
- `first_name`
- `last_name`
- `merchant`
- `merchant_slug`
- `merchant_email`
- `password_hash`
- `status`
- `payment_status`

`merchants.merchant` is unique and is referenced by `store_members.merchant`, so existing member registration can keep sending the merchant name while the platform maintains a dedicated merchant table.

Merchant registration accepts a plain `password` in the API request, hashes it with bcrypt in the service layer, and stores only `password_hash`. New merchants start with `payment_status` set to `inactive`; payment activation can be added later.

Campaign and Wallet update tracking is kept out of `store_members`. The campaign migration creates:

- `campaigns` - merchant-owned campaign definitions, message content, status, and timing.
- `campaign_deliveries` - per-member campaign targeting/delivery state.
- `wallet_passes` - per-pass authentication tokens and update timestamps.
- `wallet_registrations` - Apple Wallet device/pass push tokens registered through the Wallet pass web service.

This keeps member profile data separate from promotional state and gives the platform a delivery history for future campaign analytics.

## Docker Compose

Production is the default Compose environment:

```sh
docker compose up --build
```

Production reads required values from `.env`. Start from the example file if
you have not created one yet:

```sh
cp .env.example .env
```

The Compose files are organized as follows:

- `docker-compose.common.yml` contains the shared database, migration, backend,
  and web service definitions. Do not run this file directly.
- `docker-compose.yml` is the production entry point and adds the landing site
  plus `proxy/nginx.prod.conf`.
- `docker-compose.uat.yml` is the UAT entry point and adds only
  `proxy/nginx.uat.conf`; it does not define or build the landing site.

The production entry point starts:

- `db` - Postgres 16, with a persistent named volume.
- `backend` - Go API inside the Compose network.
- `web` - React admin console inside the Compose network.
- `landing` - Static Just Loyalty marketing site inside the Compose network.
- `proxy` - Nginx reverse proxy on HTTP and HTTPS.

The UAT entry point starts `db`, `backend`, `web`, and `proxy`. Create its
ignored environment file from the tracked template:

```sh
cp .env.uat.example .env.uat
docker compose --env-file .env.uat -f docker-compose.uat.yml up -d --build
```

Use the proxy URL for both app and API access:

```text
https://your-domain.example/
https://your-domain.example/api/v1/healthz
```

The proxy redirects HTTP to HTTPS and separates the public website from the
merchant application by hostname:

- `www.just-loyalty.com` serves the landing site.
- `just-loyalty.com` redirects to `www.just-loyalty.com`.
- `app.just-loyalty.com` serves the React merchant and business consoles.
- `/api/v1/...` on the app hostname routes to the Go backend.

UAT uses a single hostname:

- `demo.grayworks.dev` serves the merchant portal, customer registration pages,
  protected platform console at `/admin`, and `/api/v1/...` backend routes.
- The public marketing landing site is not part of the UAT service graph.

Create DNS `A` or `AAAA` records for the apex, `www`, and `app` hostnames that
point to the deployment server. The TLS certificate mounted into the proxy must
cover `just-loyalty.com`, `www.just-loyalty.com`, and `app.just-loyalty.com`.
The landing page calls to action already link to the merchant registration and
login pages on `app.just-loyalty.com`.

For UAT, create an `A` or `AAAA` record for `demo.grayworks.dev` pointing to the
UAT server. Its TLS certificate only needs to cover `demo.grayworks.dev`.

Set your public domain in `.env`:

```text
PUBLIC_BASE_URL=https://your-domain.example
```

Use `PUBLIC_BASE_URL=https://app.just-loyalty.com` in production and
`PUBLIC_BASE_URL=https://demo.grayworks.dev` in UAT. This value is embedded in
Apple Wallet web-service URLs, so it must identify the application/API host,
not the production landing-site hostname.

Then place your valid TLS files in:

```text
proxy/certs/fullchain.pem
proxy/certs/privkey.pem
```

Because UAT and production are separate servers, each checkout keeps its own
environment-specific certificate files at these same paths.

Make sure Docker Desktop is running before starting Compose. If you already have a local Postgres using port `5432`, either stop that local instance or change `POSTGRES_HOST_PORT` in `.env`. If ports `80` or `443` are already used locally, change `PROXY_HOST_PORT` or `PROXY_HTTPS_HOST_PORT` in `.env` and update `PUBLIC_BASE_URL` to match.

For Compose, the backend container gets its `DATABASE_URL` from `POSTGRES_USER`, `POSTGRES_PASSWORD`, and `APP_DATABASE_NAME`, using the internal Docker hostname `db`. The `DATABASE_URL` field in `.env` is kept for the separate case where you run the backend directly from your host with `go run ./cmd/server`.

`POSTGRES_ADMIN_DB` is the database used by the Postgres container healthcheck and admin commands. `APP_DATABASE_NAME` is the application database. Keeping them separate prevents rebuilds from depending on Postgres first-run initialization behavior.

### Deployment rollback

Application rollback uses the same environment-specific Compose command after
checking out the previous known-good release:

```sh
# Production
git checkout <previous-release>
docker compose up -d --build

# UAT
git checkout <previous-release>
docker compose --env-file .env.uat -f docker-compose.uat.yml up -d --build
```

Do not run `docker compose down -v` during rollback. SQL migrations are treated
as forward-only; restore a tested database backup only when a release explicitly
requires a database rollback.

If an existing UAT/prod volume is missing the application database, preserve the volume and run:

```sh
docker compose up -d db
docker compose --profile db-admin run --rm db-init
docker compose --profile db-admin run --rm db-migrate
docker compose up -d --build
```

The backend container mounts `./certs` at `/app/certs`. It must be writable for
admin-console credential uploads and should be backed up as sensitive
persistent data. Apple Wallet certificate paths can stay as:

```text
certs/pass-cert.pem
certs/pass-key.pem
certs/wwdr.pem
```

## Merchant Registration API

```http
POST /api/v1/merchants/register
Content-Type: application/json
```

Request:

```json
{
  "first_name": "Gary",
  "last_name": "Lee",
  "company": "Daily Brew",
  "merchant_email": "owner@dailybrew.example",
  "password": "password123"
}
```

Response:

```json
{
  "merchant": {
    "merchant_id": "MCH-8K4J2QPA",
    "creation_date": "2026-07-16T10:00:00Z",
    "first_name": "Gary",
    "last_name": "Lee",
    "merchant": "Daily Brew",
    "merchant_slug": "daily-brew",
    "merchant_email": "owner@dailybrew.example",
    "status": "active",
    "payment_status": "inactive"
  }
}
```

## Merchant Login API

```http
POST /api/v1/merchants/login
Content-Type: application/json
```

Request:

```json
{
  "email": "owner@dailybrew.example",
  "password": "password123"
}
```

Response:

```json
{
  "merchant": {
    "merchant_id": "MCH-8K4J2QPA",
    "merchant": "Daily Brew",
    "merchant_email": "owner@dailybrew.example",
    "payment_status": "inactive",
    "status": "active"
  },
  "token": "signed-merchant-token"
}
```

Use the token to load merchant-scoped members:

```http
GET /api/v1/customers/members
Authorization: Bearer signed-merchant-token
```

Each merchant also receives a public signup link based on `merchant_slug`:

```text
http://127.0.0.1:5173/m/daily-brew/join
```

That page asks customers only for name and email. It calls:

```http
POST /api/v1/public/merchants/daily-brew/members/register
Content-Type: application/json
```

Request:

```json
{
  "name": "Taylor Example",
  "email": "taylor@example.com"
}
```

The backend resolves `daily-brew` to the merchant record and creates the member under that merchant.

## Campaigns and Wallet Updates

Merchants can create campaigns with:

```http
POST /api/v1/campaigns
Authorization: Bearer <merchant-token>
Content-Type: application/json

{
  "title": "Double stamps weekend",
  "message": "Get double stamps this Saturday and Sunday."
}
```

The API creates a `campaigns` row, queues `campaign_deliveries` for the merchant's current members, and marks any existing `wallet_passes` as updated. If a member has a Wallet registration, the backend sends an APNs pass-update push to the stored `pushToken`. Campaign titles are limited to 40 characters. The active campaign message is placed in the `latest_offer` Pass Details field with its `changeMessage`; merchants may also display the short title on the pass front.

Apple Wallet populates `wallet_registrations` after a customer adds a pass. The pass contains `webServiceURL` and `authenticationToken`, then Wallet calls:

```http
POST /api/v1/apple/passes/v1/devices/{deviceLibraryIdentifier}/registrations/{passTypeIdentifier}/{serialNumber}
Authorization: ApplePass <authenticationToken>
Content-Type: application/json

{
  "pushToken": "<apple-wallet-push-token>"
}
```

The same web service also supports unregistering, listing changed serial numbers, and returning the updated `.pkpass` to Wallet. The APNs endpoint defaults to `https://api.push.apple.com` and can be overridden with `APPLE_APNS_ENDPOINT`.

## Member Registration API

```http
POST /api/v1/customers/members/register
Authorization: Bearer <merchant-token>
Content-Type: application/json
```

Request:

```json
{
  "name": "Taylor Example",
  "email": "taylor@example.com"
}
```

This endpoint is for the authenticated merchant dashboard. The backend derives the merchant from the signed bearer token and does not trust a merchant name supplied by the client.

Customers using a merchant's dedicated signup link call the public endpoint instead:

```http
POST /api/v1/public/merchants/{merchant_slug}/members/register
Content-Type: application/json

{
  "name": "Taylor Example",
  "email": "taylor@example.com"
}
```

The merchant slug must resolve to an existing registered merchant. Member registration never creates a merchant account.

Response:

```json
{
  "member": {
    "member_id": "MBR-8K4J2QPA",
    "creation_date": "2026-07-06T10:00:00Z",
    "customer_name": "Taylor Example",
    "customer_email": "taylor@example.com",
    "merchant": "Daily Brew",
    "stamp_progress": 0,
    "available_rewards": 0,
    "status": "active"
  },
  "pass_url": "http://localhost:8080/api/v1/apple/passes/MBR-8K4J2QPA.pkpass",
  "preview_url": "http://localhost:8080/api/v1/apple/passes/MBR-8K4J2QPA/preview",
  "qr_code_url": "http://localhost:8080/api/v1/apple/passes/MBR-8K4J2QPA/qr.png",
  "wallet_links": {
    "apple": { "url": "https://app.example/api/v1/apple/passes/MBR-8K4J2QPA.pkpass", "label": "Add to Apple Wallet" },
    "google": { "url": "https://app.example/api/v1/google-wallet/passes/MBR-8K4J2QPA/save", "label": "Add to Google Wallet" }
  }
}
```

The returned pass links render from the registered member record. `customer_name` is shown as the member name, `merchant` is shown as the pass brand, and `stamp_progress` controls the displayed stamps.

`wallet_links` contains only platforms activated for that merchant. `pass_url`
remains as the Apple compatibility field and is empty for a Google-only
programme. The registration page orders Google first on Android, Apple first on
iOS, and shows both on desktop when both platforms are active.

## Google Wallet

Google Wallet uses one Just Loyalty issuer per environment, one merchant class,
and one member object. It does not use Apple-style per-merchant certificates.
The environment's Google Cloud service account authenticates REST calls and
signs short-lived save-link JWTs.

Before enabling it:

1. Create separate UAT and production Google Cloud projects and Wallet issuer accounts.
2. Enable the Google Wallet API in each project.
3. Create a service account and add its email as a Developer in the matching Wallet issuer.
4. Place the downloaded JSON key in `google-credentials/` on that server.
5. Set `GOOGLE_WALLET_ISSUER_ID`, `GOOGLE_WALLET_CREDENTIALS_FILE`, and `GOOGLE_WALLET_ENABLED=true` in `.env` or `.env.uat`.
6. Apply migration `016`, rebuild the backend, then complete the Google Wallet step in `/admin`.

Real JSON keys are ignored by Git. Compose mounts the selected key read-only at
`/run/secrets/google_wallet_service_account`; the private key is never stored in
Postgres or returned by an API. See Google's [issuer onboarding](https://developers.google.com/wallet/retail/loyalty-cards/getting-started/issuer-onboarding)
and [REST authentication](https://developers.google.com/wallet/retail/loyalty-cards/getting-started/auth/rest)
guides.

Google downloads programme and hero images from the public URLs stored in each
Wallet resource. `PUBLIC_BASE_URL` is the single origin used for Apple Wallet,
Google Wallet artwork, pass links, and API callbacks, so Google Wallet must be
configured with a publicly reachable HTTPS application/API hostname. Use
`PUBLIC_BASE_URL=https://app.just-loyalty.com` in production and
`PUBLIC_BASE_URL=https://demo.grayworks.dev` in UAT.

Google Wallet provisioning intentionally rejects localhost, private IP
addresses, and plain HTTP origins because Google's servers cannot retrieve the
artwork from them. For a future tunnel-based test, point the tunnel at the
normal Nginx proxy, set `PUBLIC_BASE_URL` to the tunnel's HTTPS origin, and
recreate the backend before provisioning or synchronizing the class. This
exposes the same application routes as the normal proxy; there is no separate
Google Wallet artwork gateway.

Migration `016_add_google_wallet_loyalty_programs.sql` creates:

- `loyalty_programs` and `loyalty_program_links` for platform-neutral programme content.
- `apple_pass_programs` to map existing Apple pass types to the shared programme.
- `google_wallet_classes` and `google_wallet_objects` for Google resources and sync state.
- `wallet_sync_jobs` for durable, retryable class and object updates.

Existing Apple designs and links are backfilled and remain writable for one
compatibility release. Database triggers mirror existing Apple design writes to
the shared tables, so installed Apple passes continue unchanged.

Platform administrators use these authenticated routes:

```text
GET  /api/v1/admin/merchants/{merchantID}/google-wallet
PUT  /api/v1/admin/merchants/{merchantID}/google-wallet/assets
POST /api/v1/admin/merchants/{merchantID}/google-wallet/provision
POST /api/v1/admin/merchants/{merchantID}/google-wallet/activate
```

The square programme logo must be a PNG of at least `660 x 660`. The optional
wide logo must be a PNG of at least `1280 x 400`. Uploads are decoded and
re-encoded before storage. Provisioning submits the merchant class for review;
local activation succeeds only after Google reports `APPROVED`. Demo Mode cards
display `[TEST ONLY]` until Google grants publishing access.

The first visit to a member's Google save route idempotently creates that
member's `LoyaltyObject`, then redirects to
`https://pay.google.com/gp/v/save/<jwt>`. Stamp, reward, programme, and campaign
changes update Google asynchronously through `wallet_sync_jobs`. Google errors
therefore do not roll back registration or the authoritative member transaction.

## Stamp Scanner API

The merchant admin console has a `Customers > Stamp Scanner` tab. Scanning a Wallet QR code or entering a member ID loads the customer without changing their balance. The merchant then chooses **Add Stamp**, **Issue Reward**, or **Redeem Reward**.

The pass QR payload currently uses this shape:

```text
stamp-card:MBR-8K4J2QPA
```

The web app extracts the `MBR-...` value and first calls:

```http
GET /api/v1/customers/members/MBR-8K4J2QPA
Authorization: Bearer signed-merchant-token
```

Adding a stamp calls:

```http
PUT /api/v1/customers/members/MBR-8K4J2QPA/stamps
Authorization: Bearer signed-merchant-token
Content-Type: application/json
```

Request:

```json
{
  "stamp_delta": 1
}
```

Response:

```json
{
  "member": {
    "member_id": "MBR-8K4J2QPA",
    "customer_name": "Taylor Example",
    "customer_email": "taylor@example.com",
    "merchant": "Daily Brew",
    "stamp_progress": 7,
    "available_rewards": 0,
    "status": "active"
  },
  "event": {
    "stamp_delta": 1,
    "previous_stamp_progress": 6,
    "new_stamp_progress": 7,
    "stamp_goal": 10
  }
}
```

At the configured stamp goal, issue one reward with a unique idempotency key:

```http
POST /api/v1/customers/members/MBR-8K4J2QPA/rewards/issue
Authorization: Bearer signed-merchant-token
Idempotency-Key: issue:MBR-8K4J2QPA:unique-request-id
```

Redeem one available reward with the same header requirement:

```http
POST /api/v1/customers/members/MBR-8K4J2QPA/rewards/redeem
Authorization: Bearer signed-merchant-token
Idempotency-Key: redeem:MBR-8K4J2QPA:unique-request-id
```

Issue and redemption run in locked database transactions and append to `member_reward_transactions`. Issuing subtracts the configured stamp goal, increments `available_rewards`, and returns the member to `active`; redemption decrements the reward balance without changing current stamps. Reusing the same idempotency key returns the original transaction and does not send a second Wallet push.

All member mutations derive merchant identity from the login token and verify ownership. Successful stamp, issue, and redemption actions mark the pass as updated and use the existing APNs flow to refresh registered Wallet passes. The pass information view displays the current balance as `Rewards Earned`.

## Run

```sh
go run ./cmd/server
```

Then test:

```sh
curl http://localhost:8080/api/v1/healthz
open http://localhost:8080/api/v1/apple/passes/member-1001/preview
curl -OJ http://localhost:8080/api/v1/apple/passes/member-1001.pkpass
```

Without Apple certs, the pass endpoint returns a clear error explaining which credential is missing.

The preview endpoint does not need Apple certificates because it renders from the same pass data as plain HTML. The current sample shows a Daily Brew loyalty card with 6 of 10 stamps filled. Treat it as a design/content preview only; Apple Wallet performs the final native rendering after a signed pass is opened on an iPhone.

The actual `.pkpass` uses Apple's `PKBarcodeFormatQR` barcode field, so Wallet renders the QR natively. The HTML preview uses `/api/v1/apple/passes/{memberID}/qr.png` to show a real scannable QR code for the same payload.

## Admin Console

The React admin console lives in `web/`.

Start the Go API first:

```sh
go run ./cmd/server
```

Then start the frontend:

```sh
cd web
npm install
npm run dev -- --port 5173
```

Open:

```text
http://127.0.0.1:5173/
```

The current POC includes a sidebar, dashboard summary cards, placeholder activity panels, and a customer registration form that calls `/api/v1/customers/members/register` and embeds the generated Apple Wallet preview URL.

The stamp scanner is available under `Customers > Stamp Scanner`. Browser QR scanning uses the `BarcodeDetector` API when supported, and the manual member ID field remains available for desktop demos or browsers without camera QR support.

The customer-facing registration POC is available at:

```text
http://127.0.0.1:5173/members/register
```

It only asks for name and email, then submits the configured merchant name behind the scenes. Set `VITE_DEFAULT_MERCHANT` for the frontend if you want to change the merchant used by that page:

```sh
VITE_DEFAULT_MERCHANT="Just Loyalty" npm run dev -- --port 5173
```

After registration, the page shows a direct `.pkpass` link for testing. In a later phase, that same response can be used to email the pass link instead.

## Certificate Conversion Notes

Apple usually gives you a `.cer` file, and Keychain can export a `.p12` containing the certificate/private key.

Typical local conversion commands:

```sh
openssl x509 -inform DER -in pass.cer -out certs/pass-cert.pem
openssl pkcs12 -in pass.p12 -nocerts -nodes -out certs/pass-key.pem
```

Download the current Apple WWDR intermediate certificate from Apple and convert it to PEM if needed:

```sh
openssl x509 -inform DER -in AppleWWDRCAG*.cer -out certs/wwdr.pem
```

## Per-Merchant Apple Wallet Provisioning

New merchant registration creates an `apple_wallet_issuers` record with a
`pending` status. Pass issuance remains disabled until payment, Apple identity,
certificate, assets, and design are complete and an administrator performs the
final activation.

Set an administrator token and mount merchant credentials and pass assets
beneath their separate shared directories:

```env
ADMIN_API_TOKEN=replace-with-a-long-random-value
APPLE_CERTIFICATE_BASE_PATH=certs
APPLE_PASS_ASSET_BASE_PATH=pass-assets
```

```text
certs/luckin-coffee/pass-cert.pem
certs/luckin-coffee/pass-key.pem
certs/wwdr.pem

pass-assets/luckin-coffee/icon.png
pass-assets/luckin-coffee/icon@2x.png
pass-assets/luckin-coffee/icon@3x.png
pass-assets/luckin-coffee/logo.png
pass-assets/luckin-coffee/logo@2x.png
pass-assets/luckin-coffee/logo@3x.png
```

Provision an existing or newly registered merchant in four steps. Replace the
merchant ID and administrator token with your own values.

```bash
curl -X POST https://loyalty.example.com/api/v1/admin/merchants/MCH-EXAMPLE/apple-wallet/issuer \
  -H 'Authorization: Bearer your-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "ownership_mode": "platform_managed",
    "team_identifier": "FVNA87HWN5",
    "organization_name": "Luckin Coffee"
  }'
```

```bash
curl -X POST https://loyalty.example.com/api/v1/admin/merchants/MCH-EXAMPLE/apple-wallet/pass-types \
  -H 'Authorization: Bearer your-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "pass_type_identifier": "pass.com.justloyalty.luckincoffee",
    "pass_style": "storeCard"
  }'
```

```bash
curl -X POST https://loyalty.example.com/api/v1/admin/merchants/MCH-EXAMPLE/apple-wallet/certificates \
  -H 'Authorization: Bearer your-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "pass_type_identifier": "pass.com.justloyalty.luckincoffee",
    "credential_alias": "luckin-coffee"
  }'
```

The last request validates the certificate/private-key match, Pass Type ID,
Apple Team ID, validity period, and WWDR signature. The certificate remains in
`validated` status while onboarding is incomplete. Then create the
merchant-specific pass design:

```bash
curl -X PUT https://loyalty.example.com/api/v1/admin/merchants/MCH-EXAMPLE/apple-wallet/pass-design \
  -H 'Authorization: Bearer your-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "pass_type_identifier": "pass.com.justloyalty.luckincoffee",
    "program_name": "Luckin Coffee Loyalty Card",
    "pass_description": "Collect stamps and enjoy rewards at Luckin Coffee.",
    "stamp_goal": 10,
    "reward_title": "Free Coffee",
    "background_color": "#5434F5",
    "foreground_color": "#FFFFFF",
    "label_color": "#E1DCFF",
    "asset_bundle_ref": "luckin-coffee",
    "stamp_instructions": "Present this pass when making an eligible purchase to collect a stamp.",
    "reward_details": "Complete your stamp card to receive one complimentary drink.",
    "support_text": "Questions? Contact hello@luckin.example.",
    "terms_text": "One stamp per eligible transaction. Rewards cannot be exchanged for cash.",
    "links": [
      {"link_type":"website","label":"Website","url":"https://luckin.example"},
      {"link_type":"instagram","label":"Instagram","url":"https://instagram.com/luckin.example"}
    ],
    "campaign_display_mode": "details"
  }'
```

`campaign_display_mode` supports the following values:

- `details` keeps the full active campaign message in the Pass Details field
  `latest_offer` and does not display campaign copy on the front.
- `front_and_details` adds the campaign title to the front auxiliary field
  `latest_offer_title` and keeps the full message in Pass Details.
- `none` hides both campaign fields and remains available for API compatibility.

Only `latest_offer` has a `changeMessage`, so Wallet can show the full campaign
notification without producing a duplicate notification from the front field.

### Dynamic stamp artwork

Migration `014_add_stamp_icon_key.sql` adds the merchant-level
`apple_pass_designs.stamp_icon_key` setting. Supported values are `none`,
`logo`, `coffee`, `star`, `gift`, `heart`, `shopping_bag`, and `custom` after
migration `015_default_physical_stamp_logo.sql` is applied. Migration `015`
defaults new designs to `logo` without changing existing rows. The built-in
keys remain API-compatible, while the merchant interfaces expose only the
merchant logo and a custom transparent icon.

For goals from 1 to 10, the backend renders the member's current progress into
`strip.png`, `strip@2x.png`, and `strip@3x.png` while generating the signed
pass. Each position is a white physical stamp circle; completed positions show
the selected artwork and empty positions remain blank. Goals up to 5 use one
centered row; goals from 6 to 10 use two balanced rows so each stamp remains
readable. `logo` uses the merchant's existing `icon@3x.png`. Goals above 10,
`none`, and unavailable artwork safely fall back to the remaining-stamps field.
Stamp updates and reward resets already mark the pass as updated, so the next
Wallet refresh receives newly rendered artwork.

Use the following standard Apple Wallet PNG dimensions. A transparent icon
background is recommended when the same icon is used inside the white physical
stamp circles:

```text
icon.png       38 x 38 px
icon@2x.png    76 x 76 px
icon@3x.png   114 x 114 px

logo.png       50-160 x 50 px
logo@2x.png   100-320 x 100 px
logo@3x.png   150-480 x 150 px
```

The programme name is omitted from the pass front and is used as Apple Wallet's
required top-level description, such as `Coffee Brew Loyalty Card`. It is not
repeated as a separate `Programme` field in Pass Details. The stored
`pass_description` field remains in the API for backwards compatibility and is
populated from the programme name by the web consoles.

Merchants publish stamp artwork through the authenticated multipart endpoint:

```text
PUT /api/v1/merchants/me/apple-wallet/stamp-icon
stamp_icon_key=logo
```

The authenticated `GET` form of the same endpoint returns the source artwork
for an accurate merchant preview:

```text
GET /api/v1/merchants/me/apple-wallet/stamp-icon?source=logo
```

For `custom`, include a `stamp_icon` file containing a transparent square PNG
between 64 and 512 pixels and no larger than 1 MB. The server decodes and
re-encodes the image before storing it as `stamp.png` in the merchant's asset
bundle. The admin onboarding asset upload accepts the same image using the
multipart field name `stamp.png`.

After activation, the merchant can maintain the customer-facing information
from the **Passes** section of the merchant portal. The portal uses the
authenticated endpoint below; callers cannot select another merchant ID:

```text
GET /api/v1/merchants/me/apple-wallet/pass-design
PUT /api/v1/merchants/me/apple-wallet/pass-design
```

Publishing increments the design version, marks that merchant's issued passes
as updated, and sends an empty APNs Wallet update notification to registered
devices. Wallet then requests the newly signed pass. The active campaign is
still managed from **Campaigns**, stamp progress is calculated per member, and
the `Powered by Just Loyalty` link is platform-controlled.

Apply the pass-information schema migration before deploying a backend that
uses these fields:

```bash
docker compose --profile db-admin run --rm db-migrate
docker compose up -d --build
```

Inspect the resulting configuration with:

```bash
curl https://loyalty.example.com/api/v1/admin/merchants/MCH-EXAMPLE/apple-wallet \
  -H 'Authorization: Bearer your-admin-token'
```

The protected business console is available at `/admin`. Configure its login
without exposing the credentials in the frontend bundle:

```env
PLATFORM_ADMIN_EMAIL=admin@justloyalty.example
PLATFORM_ADMIN_PASSWORD=replace-with-a-long-unique-password
```

The console calls the administrator APIs with an eight-hour, signed
`platform_admin` token. `ADMIN_API_TOKEN` remains supported for scripts.
Certificate and asset uploads use immutable server-generated aliases beneath
`APPLE_CERTIFICATE_BASE_PATH` and `APPLE_PASS_ASSET_BASE_PATH`; both mounts must
be writable and persistent.

The final wizard action calls:

```bash
curl -X POST https://loyalty.example.com/api/v1/admin/merchants/MCH-EXAMPLE/apple-wallet/activate \
  -H 'Authorization: Bearer your-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{"pass_type_identifier":"pass.com.justloyalty.luckincoffee"}'
```

Activation is rejected unless the merchant payment is active and all Wallet
configuration checks pass. Runtime pass credential resolution also enforces
active merchant and payment status.

## Next Steps

1. Replace local certificate references with a production secret manager.
2. Add certificate expiry monitoring and rotation controls.
3. Add Apple Wallet provisioning status to the administrator UI.
4. Add the Swift client flow from `docs/ios-passkit-client.md`.
