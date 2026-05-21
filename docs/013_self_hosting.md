# Self-hosting Confide

## Prerequisites

- Docker 24+ and Docker Compose v2 (`docker compose` command, not `docker-compose`)
- A domain name with an A/AAAA record pointing to your server's public IP
- Ports 80 and 443 open inbound on your server's firewall

## Why HTTPS is required

Confide uses WebAuthn (passkeys) for authentication. The WebAuthn specification requires a **Secure Context** — meaning the page must be served over HTTPS. This is enforced by the browser, not by Confide. Localhost is exempt (for local development), but any publicly accessible deployment must use TLS.

Confide's `docker-compose.yml` includes Caddy as the reverse proxy. Caddy automatically obtains a free TLS certificate from Let's Encrypt with no additional configuration — you only need to point a domain at your server and set `CONFIDE_DOMAIN` in your `.env`.

## Quick start

### 1. Clone the repo

```bash
git clone https://github.com/phantompunk/confide.git
cd confide
```

### 2. Configure

```bash
cp .env.example .env
```

Open `.env` and set the following required values:

| Variable | What to set |
|---|---|
| `CONFIDE_DOMAIN` | Your domain, e.g. `confide.example.com` |
| `CONFIDE_SECRET_KEY` | A random 32-byte base64url key (see below) |
| `DB_PASSWORD` | A strong random password for Postgres |

Generate the HMAC key:

```bash
openssl rand -base64 32 | tr '+/' '-_'
```

Copy the output and set it as `CONFIDE_SECRET_KEY=<output>`.

All other settings have correct defaults for a standard single-domain deployment. The `CONFIDE_RP_ID` and `CONFIDE_RP_ORIGIN` values are derived from `CONFIDE_DOMAIN` automatically in the compose file.

### 3. Start

```bash
docker compose -f deploy/docker-compose.yml up -d
```

Caddy will obtain a TLS certificate on first startup. This may take 30–60 seconds. Check progress with:

```bash
docker compose -f deploy/docker-compose.yml logs caddy
```

### 4. Create your account

Open `https://your-domain.com/register` in your browser. Create your account using a passkey (Touch ID, Face ID, Windows Hello, or a hardware security key). You will be prompted to save recovery codes — store them somewhere safe.

### 5. Lock registration

After creating your account, prevent others from registering:

```bash
# In .env, set:
CONFIDE_REGISTRATION_OPEN=false
```

Then restart the API:

```bash
docker compose -f deploy/docker-compose.yml restart api
```

Registration is now closed. Attempting to register will return a 403 error.

## Upgrading

```bash
git pull
docker compose -f deploy/docker-compose.yml up --build -d
```

Database migrations run automatically on API startup. Downtime is minimal (a few seconds for the container restart).

## Backing up

Confide's data lives in the `pgdata` Docker volume. Back it up with:

```bash
docker compose -f deploy/docker-compose.yml exec db \
  pg_dump -U confide confide | gzip > Confide-backup-$(date +%Y%m%d).sql.gz
```

To restore from a backup:

```bash
# Stop the API to prevent writes during restore
docker compose -f deploy/docker-compose.yml stop api

# Restore
gunzip -c Confide-backup-YYYYMMDD.sql.gz | \
  docker compose -f deploy/docker-compose.yml exec -T db \
  psql -U Confide Confide

# Restart
docker compose -f deploy/docker-compose.yml start api
```

Schedule backups with `cron` or your hosting provider's backup service.

## Environment variable reference

| Variable | Type | Default | Description |
|---|---|---|---|
| `CONFIDE_DATABASE_URL` | string | required | PostgreSQL connection string |
| `CONFIDE_SECRET_KEY` | base64url | required | 32-byte HMAC key for session tokens. Generate with `openssl rand -base64 32 \| tr '+/' '-_'` |
| `CONFIDE_DOMAIN` | string | — | Public domain name. Used by `docker-compose.yml` to configure Caddy and set WebAuthn RP vars |
| `PORT` | string | `:8080` | Address and port the API listens on inside its container |
| `CONFIDE_CORS_ORIGIN` | string | `http://localhost:3000` | Allowed origin for CORS on `/api/*` routes. Set automatically to `https://CONFIDE_DOMAIN` in the compose file |
| `CONFIDE_RP_ID` | string | `localhost` | WebAuthn Relying Party ID. Must match your domain (e.g. `confide.example.com`) |
| `CONFIDE_RP_ORIGIN` | string | `http://localhost:3000` | WebAuthn Relying Party origin. Must be `https://` + your domain |
| `CONFIDE_RP_DISPLAY_NAME` | string | `Confide` | Display name shown in passkey prompts |
| `CONFIDE_ENV` | string | `development` | Set to `production` to enable `Secure` cookies |
| `CONFIDE_RELAY_FLUSH_INTERVAL` | duration | `60s` | How often the relay queue flushes encrypted responses to Postgres |
| `CONFIDE_REGISTRATION_OPEN` | bool | `true` | Set to `false` to disable new account registration |
| `DB_PASSWORD` | string | `changeme` | Postgres password (used by `docker-compose.yml` to build the connection string) |
