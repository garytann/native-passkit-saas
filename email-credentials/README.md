# Email credentials

Store the Google Workspace SMTP app password in an untracked file in this
directory. Configure its host path with `EMAIL_SMTP_PASSWORD_SOURCE_FILE`.

The backend reads the Docker secret from `/run/secrets/email_smtp_password`.
Never commit a real app password.
