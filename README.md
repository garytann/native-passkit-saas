# Native iOS PassKit WIP

Go proof of concept for issuing Apple Wallet F&B loyalty stamp cards without the PassKit.com SDK.

This project uses:

- Go for the backend API and pass package generation.
- Apple Wallet pass package format: `pass.json`, images, `manifest.json`, `signature`, ZIP.
- OpenSSL for the PKCS #7 detached signature Apple Wallet requires.
- Native iOS `PassKit.framework` only on the client side, to present the add-to-Wallet UI.

## Current Routes

- `GET /healthz` - service health check.
- `GET /apple/passes/example` - returns a sample pass URL.
- `GET /apple/passes/{memberID}/preview` - renders an unsigned browser preview.
- `GET /apple/passes/{memberID}/qr.png` - renders the preview QR code as a PNG.
- `GET /apple/passes/{memberID}.pkpass` - generates and returns a signed Apple Wallet stamp card.

## Apple Credentials Required

To install a generated pass into Apple Wallet, you need Apple Developer Program credentials:

- Pass Type ID, for example `pass.com.yourcompany.membership`
- Team ID
- Pass Type ID certificate
- Private key associated with the certificate request
- Apple WWDR intermediate certificate

The code can compile without those files, but `/apple/passes/{memberID}.pkpass` cannot produce an installable pass until the certs are present.

## Configure

Copy `.env.example` values into your shell environment, or export only the values you need:

```sh
export PORT=8080
export PUBLIC_BASE_URL=http://localhost:8080
export APPLE_PASS_TYPE_ID=pass.com.yourcompany.membership
export APPLE_TEAM_ID=YOURTEAMID
export APPLE_ORGANIZATION_NAME="Your Company"
export APPLE_PASS_CERT_PATH=certs/pass-cert.pem
export APPLE_PASS_KEY_PATH=certs/pass-key.pem
export APPLE_WWDR_CERT_PATH=certs/wwdr.pem
```

Put local PEM files in `certs/`. Do not commit real certificates or private keys.

## Run

```sh
go run ./cmd/server
```

Then test:

```sh
curl http://localhost:8080/healthz
open http://localhost:8080/apple/passes/member-1001/preview
curl -OJ http://localhost:8080/apple/passes/member-1001.pkpass
```

Without Apple certs, the pass endpoint returns a clear error explaining which credential is missing.

The preview endpoint does not need Apple certificates because it renders from the same pass data as plain HTML. The current sample shows a Daily Brew loyalty card with 6 of 10 stamps filled. Treat it as a design/content preview only; Apple Wallet performs the final native rendering after a signed pass is opened on an iPhone.

The actual `.pkpass` uses Apple's `PKBarcodeFormatQR` barcode field, so Wallet renders the QR natively. The HTML preview uses `/apple/passes/{memberID}/qr.png` to show a real scannable QR code for the same payload.

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

The current POC includes a sidebar, dashboard summary cards, placeholder activity panels, and a pass generator that calls `/apple/passes/example` and embeds the generated preview URL.

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
