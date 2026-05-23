# Confide

**A privacy-first end-to-end encrypted form builder**

**Confide** is an intake platform designed to make collecting sensitive data, secure and effortless. It provides the seamless user experience of a traditional form builder while handling all cryptographic key management and client side encryption completely behind-the-scenes.

Learn more at [useconfide.app](https://useconfide.app/).



## ✨ Features

- **🔒 True E2EE:** Forms & responses are encrypted in the browser. The server never sees form schemas or plain text answers.
- **🔑 Passkey-Powered:** Modern, passwordless authentication with native key management. Uses WebAuthn authentication meaning no passwords or email required.
- **🗄️ Invisible Key Management:** Encryption and key distribution happen entirely behind the scenes. Members log in, create, modify, and collaborate on forms normally.
- **📝 Seamless Colloboration:** Workspaces are fully encrypted for all members, teammates can share and access data without ever having to manually manage, copy, or exchange cryptographic keys.
- **👁️ Privacy Controls:** Automatically purge responses after a specific number of days and automatically close forms after a set response limit or a scheduled close date.
- **🌎 Multi-Language Support:** Every field supports independent translations you can even customize the field order per per language.
- **📦 Self-Hostable:** Run an instance of **Confide** on your own hardware.



## Quick start

```bash
mkdir confide && cd confide
curl -fsSLo docker-compose.yml https://raw.githubusercontent.com/confidehq/confide/main/docker-compose.yml
docker compose up -d
```

Open `https://localhost:3000` to create your account.

See the [self-hosting guide](docs/self-hosting.md) for full setup instructions.



## Tech Stack

- **Backend**
  - **[Chi](https://github.com/go-chi/chi)** for the backend API.
  - **[WebAuthn](https://github.com/go-webauthn/webauthn)** for server-side Passkey validation.
- **Frontend**
  - **[Svelte](https://github.com/sveltejs/svelte)** powers the UI and runes manage state
  - **[Svelte Kit](https://github.com/sveltejs/kit)** for routing, static site generation
  - **[SimpleWebAuthn](https://github.com/MasterKale/SimpleWebAuthn)** for browser-level Passkey validation
  - **[WebCryptoAPI](https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto)** for all client side crypto operations and key management
- **Database**
  - **[PostgreSQL](https://www.postgresql.org/)** handles all the data
  - **[SQLc](https://sqlc.dev/)** generates data layer code from SQL queries



## License

This repository's source code is available under the [AGPL-3.0 license](LICENSE).
