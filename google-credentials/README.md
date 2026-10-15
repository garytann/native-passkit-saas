# Google Wallet credentials

Place each environment's Google Cloud service-account JSON key in this folder,
then set `GOOGLE_WALLET_CREDENTIALS_FILE` in that server's `.env` file. Real
keys are ignored by Git. The checked-in `disabled.json` placeholder is used only
while `GOOGLE_WALLET_ENABLED=false`.
