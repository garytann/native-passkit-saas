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
- `POST /api/v1/public/merchants/{merchantSlug}/members/register` - registers a customer from a merchant-owned public signup link.
- `PUT /api/v1/customers/members/{memberID}/stamps` - updates stamp progress for a member owned by the authenticated merchant.
- `GET /api/v1/apple/passes/example` - returns a sample pass URL.
- `GET /api/v1/apple/passes/{memberID}/preview` - renders an unsigned browser preview.
- `GET /api/v1/apple/passes/{memberID}/qr.png` - renders the preview QR code as a PNG.
- `GET /api/v1/apple/passes/{memberID}.pkpass` - generates and returns a signed Apple Wallet stamp card.

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

Run these once when provisioning a new UAT/prod environment:

```sh
docker compose up -d db
docker compose --profile db-admin run --rm db-init
docker compose --profile db-admin run --rm db-migrate
docker compose up -d --build
```

For normal application deploys after the database already exists:

```sh
docker compose up -d --build
```

When a future release includes schema changes, run the migration job deliberately during the deployment:

```sh
docker compose --profile db-admin run --rm db-migrate
docker compose up -d --build
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

## Docker Compose

The easiest way to run the full POC stack is Docker Compose:

```sh
docker compose up --build
```

Compose reads required values from `.env`. Start from the example file if you have not created one yet:

```sh
cp .env.example .env
```

This starts:

- `db` - Postgres 16, with a persistent named volume.
- `backend` - Go API inside the Compose network.
- `web` - React admin console inside the Compose network.
- `proxy` - Nginx reverse proxy on HTTP and HTTPS.

Use the proxy URL for both app and API access:

```text
https://your-domain.example/
https://your-domain.example/api/v1/healthz
```

The proxy redirects HTTP to HTTPS, routes `/api/v1/...` to the Go backend, and routes all other paths to the React web app. This keeps the browser on one origin, which is closer to the shape you would deploy behind a public load balancer later.

Set your public domain in `.env`:

```text
PUBLIC_BASE_URL=https://your-domain.example
```

Then place your valid TLS files in:

```text
proxy/certs/fullchain.pem
proxy/certs/privkey.pem
```

Make sure Docker Desktop is running before starting Compose. If you already have a local Postgres using port `5432`, either stop that local instance or change `POSTGRES_HOST_PORT` in `.env`. If ports `80` or `443` are already used locally, change `PROXY_HOST_PORT` or `PROXY_HTTPS_HOST_PORT` in `.env` and update `PUBLIC_BASE_URL` to match.

For Compose, the backend container gets its `DATABASE_URL` from `POSTGRES_USER`, `POSTGRES_PASSWORD`, and `APP_DATABASE_NAME`, using the internal Docker hostname `db`. The `DATABASE_URL` field in `.env` is kept for the separate case where you run the backend directly from your host with `go run ./cmd/server`.

`POSTGRES_ADMIN_DB` is the database used by the Postgres container healthcheck and admin commands. `APP_DATABASE_NAME` is the application database. Keeping them separate prevents rebuilds from depending on Postgres first-run initialization behavior.

If an existing UAT/prod volume is missing the application database, preserve the volume and run:

```sh
docker compose up -d db
docker compose --profile db-admin run --rm db-init
docker compose --profile db-admin run --rm db-migrate
docker compose up -d --build
```

The backend container mounts `./certs` read-only at `/app/certs`, so your Apple Wallet certificate paths can stay as:

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

## Member Registration API

```http
POST /api/v1/customers/members/register
Content-Type: application/json
```

Request:

```json
{
  "name": "Taylor Example",
  "email": "taylor@example.com",
  "merchant": "Daily Brew"
}
```

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
    "status": "active"
  },
  "pass_url": "http://localhost:8080/api/v1/apple/passes/MBR-8K4J2QPA.pkpass",
  "preview_url": "http://localhost:8080/api/v1/apple/passes/MBR-8K4J2QPA/preview",
  "qr_code_url": "http://localhost:8080/api/v1/apple/passes/MBR-8K4J2QPA/qr.png"
}
```

The returned pass links render from the registered member record. `customer_name` is shown as the member name, `merchant` is shown as the pass brand, and `stamp_progress` controls the displayed stamps.

## Stamp Scanner API

The merchant admin console has a `Customers > Stamp Scanner` tab for the demo stamp workflow. A logged-in merchant can scan the QR code on a customer's Wallet pass, or manually enter the readable member ID if camera scanning is unavailable.

The pass QR payload currently uses this shape:

```text
stamp-card:MBR-8K4J2QPA
```

The web app extracts the `MBR-...` value and calls:

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
    "status": "active"
  },
  "event": {
    "stamp_delta": 1,
    "previous_stamp_progress": 6,
    "new_stamp_progress": 7
  }
}
```

The backend reads the merchant name from the login token, not from the request body. The repository locks the member row, verifies the member belongs to that merchant, clamps stamp progress between `0` and `10`, and returns `403 forbidden` if another merchant attempts the update.

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
VITE_DEFAULT_MERCHANT="Saga Loyalty" npm run dev -- --port 5173
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

## Next Steps

1. Replace the sample member data with database-backed member records.
2. Replace generated placeholder PNGs with branded pass assets.
3. Add Wallet web service endpoints for pass updates.
4. Add APNs push support for Wallet update notifications.
5. Add the Swift client flow from `docs/ios-passkit-client.md`.
