# Confide

**Private forms. End-to-end encrypted.**

Confide is an open-source form builder where responses are encrypted in the respondent's browser before they reach the server. Only the form creator can read them.

## Features

- Passkey (WebAuthn) authentication — no passwords, no email required
- End-to-end encrypted responses using WebCrypto AES-GCM
- No IP logging, no respondent cookies, no tracking
- Multi-language form support
- Self-hostable with a single `docker compose up`

## Quick start

```bash
git clone https://github.com/phantompunk/confide.git
cd wisp
cp .env.example .env
# Edit .env: set CONFIDE_DOMAIN, CONFIDE_HMAC_KEY, DB_PASSWORD
docker compose -f deploy/docker-compose.yml up -d
```

Open `https://your-domain.com/register` to create your account.

See the [self-hosting guide](docs/self-hosting.md) for full setup instructions.

## Architecture

The Go API handles authentication, form storage, and response collection. Responses arrive at a relay endpoint, are queued in memory, and flushed to Postgres in encrypted form — the server never decrypts them. The SvelteKit frontend handles all encryption and decryption in the browser using the WebCrypto API. Caddy provides automatic TLS termination.


## License

This repository's source code is available under the [AGPL 3.0 license](LICENSE)
