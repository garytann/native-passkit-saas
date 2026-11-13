# Email credentials

Store the Google Workspace Gmail API service-account JSON for each environment
in this ignored directory. Configure its host path with
`GMAIL_API_CREDENTIALS_SOURCE_FILE`.

Docker Compose mounts the selected key at
`/run/secrets/gmail_api_service_account`. The backend uses domain-wide
delegation to impersonate `GMAIL_API_USER` with the `gmail.send` scope.

Never commit a real service-account key. `disabled.json` is only a harmless
placeholder for environments where `EMAIL_ENABLED=false`.
