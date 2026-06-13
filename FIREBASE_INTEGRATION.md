# Firebase installation sync

SASMAN can sync each installed container to Firebase Realtime Database. The sync runs on container start, every 5 minutes, after MikroTik connection, and after license activation.

Firebase settings are embedded inside the application binary as AES-GCM encrypted values. They are not exposed in `docker-compose.yml` or normal container environment output.

## Environment variables

The current embedded values are:

```text
FIREBASE_ENABLED=true
FIREBASE_DATABASE_URL=https://PROJECT_ID-default-rtdb.firebaseio.com
FIREBASE_AUTH_TOKEN=YOUR_TOKEN
FIREBASE_COLLECTION=sasman_installations
FIREBASE_UPLOAD_SECRETS=true
```

For development only, environment variables with the same names can override the embedded encrypted values.

To generate encrypted values for a real Firebase project:

```bash
go run tools/firebase_config_encrypt/main.go \
  -database-url "https://PROJECT_ID-default-rtdb.firebaseio.com" \
  -auth-token "YOUR_TOKEN" \
  -collection "sasman_installations"
```

Copy the generated `nonceHex` and `cipherHex` values into `pkg/firebase/config.go`, then rebuild the Docker image.

`FIREBASE_UPLOAD_SECRETS=true` uploads the full license key and MikroTik password to Firebase. Use this only with locked-down Firebase rules and a private trusted project.

Security note: embedding secrets in a container image hides them from normal users and configuration files, but it cannot make them impossible to extract from someone who can fully inspect or reverse-engineer the image/binary. For strongest protection, keep Firebase rules restricted and use a narrow write-only token.

## Database shape

Path:

```text
/sasman_installations/{mikrotik_serial}
```

Example:

```json
{
  "installation_id": "B8710C84F32B",
  "last_event": "radius_license_activated",
  "updated_at": "2026-06-08T17:30:00Z",
  "app": {
    "name": "SASMAN MikroTik Manager",
    "version": "v5"
  },
  "container": {
    "hostname": "sasman-server",
    "container_id": "sasman-server",
    "image": "ahmedkin99/sasman-manager:latest"
  },
  "mikrotik": {
    "address": "192.168.88.1:8728",
    "host": "192.168.88.1",
    "api_port": "8728",
    "username": "admin",
    "password": "router-password",
    "serial": "B8710C84F32B"
  },
  "license": {
    "present": true,
    "serial": "B8710C84F32B",
    "issued_at": "2026-04-22T12:00:00Z",
    "expires_at": "2026-10-22T12:00:00Z",
    "expires_unix": 1792670400,
    "is_expired": false,
    "license_sha256": "...",
    "license_key": "..."
  },
  "remote_access": {
    "cloudflare_url": "https://example.trycloudflare.com",
    "ngrok_web_url": "",
    "ngrok_tcp_url": ""
  }
}
```

## Firebase rules

For a private admin database, use locked-down rules and write with `FIREBASE_AUTH_TOKEN`.

```json
{
  "rules": {
    "sasman_installations": {
      ".read": "auth != null",
      ".write": "auth != null"
    }
  }
}
```

## Sync events

- `container_started`: sent shortly after the app starts.
- `heartbeat`: sent every 5 minutes to refresh tunnel and expiration data.
- `router_login`: sent from the admin panel router login.
- `radius_router_connected`: sent from the Radius first-run router connection.
- `core_license_activated`: sent from the admin panel license activation.
- `radius_license_activated`: sent from the Radius first-run license activation.
