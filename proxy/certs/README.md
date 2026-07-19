# Proxy TLS Certificates

Place your valid domain certificate files here for the Nginx proxy:

- `fullchain.pem` - server certificate plus intermediate chain
- `privkey.pem` - private key for the certificate

These files are mounted read-only into the proxy container at `/etc/nginx/ssl`.

Do not commit real certificate or key files.
