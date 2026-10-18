# Apple Wallet pass assets

Create one directory per merchant design alias. Each directory must contain:

```text
pass-assets/luckin-coffee/
  icon.png
  icon@2x.png
  icon@3x.png
  logo.png
  logo@2x.png
  logo@3x.png
```

Use PNG files. Keep the logo at 50 points high and between 50 and 160 points
wide, then export matching 2x and 3x variants. Keep the icon square and export
matching 2x and 3x variants.

The `asset_bundle_ref` stored in the database is only the directory alias, such
as `luckin-coffee`. The backend resolves it beneath `APPLE_PASS_ASSET_BASE_PATH`.
