# Privacy Policy

Version: `2026.5.0`

Last Updated: May 8, 2026



## Overview

We strongly support your right to privacy. Our policy is simple: **Your data is none of our business.**

We make money by selling software, not by mining your or your respondents' personal information. Our core values guide every decision we make:

- **Privacy by Design:** Security is baked into the code, not bolted on.
- **Encryption Made Easy:** We handle the complexity of end-to-end encryption (E2EE) so you don't have to.
- **Minimal Data Retention:** We only keep what is strictly necessary to run the service.
- **Transparency of Purpose:** We are open about what we collect and why.

We only collect what we need. Here's what that means in practice:



## What we collect and why

Our philosophy is simple: **We cannot leak what we do not have.** We minimize data collection as much as possible.

### Anonymous Authentication

We do not require an email address or password to use this platform.

- **Identity:** Your account is bound to a physical authenticator (Passkey) such as a hardware key (YubiKey), TouchID, FaceID, or a secure password manager.
- **Usernames:** You must choose a unique username. This is public and serves as your identifier for team invites and workspace collaboration. We encourage the use of pseudonyms.

### Billing & Subscriptions

Payment processing is handled entirely by **Stripe**.

- **Zero Financial Data:** We never see or store your credit card or billing details.
- **Subscription Linkage:** We only store a reference ID for your Stripe customer profile and subscription status to manage your account limits and features.

### Form Data & Response Privacy

We provide the tools for secure data collection, but privacy is a shared responsibility between the platform and the creator.

- **Encryption:** All form responses are encrypted. Neither the platform nor our hosting providers can read them.
- **Access Control:** Only the form creator and their authorized team members have the keys to decrypt and view responses.
- **Creator Responsibility:** While responses are anonymous by default, a form creator may choose to request personally identifiable information. We urge creators to practice data minimization.

### Operational Analytics

To improve the platform, we collect limited, high-level usage data.

- **What we track:** We monitor aggregate actions (e.g., page visits, clicks, and language preferences).
- **Privacy First:** We strip identifiable markers to focus on general trends. We do not track individual users across the web.

### Cookies and Local Storage

We do not use any third-party tracking cookies, advertising beacons, or "pixels" from platforms like Meta or Google. Our platform is built to respect your privacy by default.

- **Essential Storage Only:** We use local storage and session-based cookies strictly for **authentication and security **purposes. These allow you to remain logged in and ensure that your Passkey-bound session is secure.
- **No Cross-Site Tracking:** We do not track your behavior across other websites or sell your browsing data to third parties.

### Direct Communications

If you contact us for support or feedback via email or our contact forms, we will store that specific communication and your contact details solely to resolve your request and follow up.



## How we secure your data

Our security model is built on the principle of **Zero-Knowledge**. This means your most sensitive data is encrypted before it ever touches our servers, ensuring that only you and your authorized team members hold the keys.

### End-to-End Encryption (E2EE)

- **Client-Side Protection:** Form schemas and configurations are encrypted directly on your device before they are uploaded.
- **The Secure Link:** When a respondent visits your unique form link, their browser decrypts the form locally. When they submit a response, it is encrypted on *their* device before being sent to our infrastructure.
- **Decryption:** Only you and your designated team members possess the cryptographic keys required to decrypt and view these responses.

### Transport & Infrastructure Security

- **Data in Transit:** All communication between your browser and our servers is protected using industry-standard **SSL/TLS** encryption.
- **Data at Rest:** Our PostgreSQL database and all system backups are stored as encrypted blobs on our private, self-hosted infrastructure.

### Metadata Transparency

To manage your forms and ensure reliable delivery, we store a limited amount of unencrypted metadata. This data is strictly used for platform functionality and does not contain the contents of your forms or responses.

**Examples of metadata we store include:**

- **Timestamps:** When a form or response was created or last updated.
- **Form Status:** Whether a form is currently "Open" or "Closed."
- **Logic Flags:** Technical settings, such as response retention periods (TTL) or whether PGP forwarding is enabled.

### Security and Bug Reporting

We take the security of our platform seriously and welcome the assistance of the security research community. If you believe you have discovered a vulnerability or a bug:

- **Contact Us:** Please reach out to us at **security@[yourdomain].com**.
- **Responsible Disclosure:** We ask that you give us a reasonable amount of time to resolve the issue before making any information public. We value your help in keeping our community safe.



## When we access or disclose your info

Our business model and technical architecture are designed to limit our access to your information. We cannot disclose what we cannot see.

### 1. Employee Access

- **Encrypted Data:** No employee or administrator of this platform can access your form schemas, responses, or uploaded files. This data is encrypted client-side; we do not hold the keys.
- **Metadata:** Access to unencrypted metadata (such as account status or billing IDs) is strictly restricted to essential personnel for the purposes of troubleshooting and platform maintenance.

### 2. Legal Requests & Subpoenas

We value the privacy of our users and will only comply with legal requests to the extent required by law.

- **Notification:** Unless legally prohibited by a "gag order," we will notify you of any legal request for your data before disclosure to give you the opportunity to challenge the request.
- **Zero-Knowledge Limitation:** If we are served a valid legal warrant or subpoena, we can only provide the data we have (e.g., your username, subscription status, and account metadata). Because your core content is encrypted with keys held only by you, we cannot provide decrypted form data or responses to any third party, including law enforcement.

### 3. Safety & Protection

We may disclose limited account information only if we have a good-faith belief that disclosure is necessary to:

- Comply with a legal obligation.
- Protect the personal safety of users or the public.
- Protect against legal liability.



## What happens when you delete your account

We believe you should have total control over your data. You may delete your information or your entire account at any time through your account settings.

### 1. Account & Workspace Deletion

- **Ownership:** When you delete your account, any workspaces you own are permanently removed.
- **Transfer of Ownership:** If a workspace has multiple members, you will be prompted to transfer ownership to another member before your account is closed. If no transfer is made, the workspace and its associated data will be purged.
- **Subscriptions:** Upon account deletion, any active paid subscriptions are canceled immediately.

### 2. The Deletion Timeline

We maintain a strict "Clear-to-Empty" policy to balance user error protection with data privacy:

- **Instant Removal:** Once you confirm deletion, your data is immediately removed from our active production database.
- **Backup Retention:** To protect against accidental loss, encrypted snapshots of your data remain in our secure, self-hosted backups for **seven (7) days**.
- **Permanent Purge:** After the 7-day window, the encrypted data is permanently purged from our backup infrastructure and cannot be recovered by you or our team.

### 3. Data Portability and Export

You own your data. We believe in total data portability and provide the tools for you to take your information with you at any time.

- **Manual Export:** You can export your form schemas and collected responses into standard, machine-readable formats such as **JSON** and **CSV**.
- **No Lock-in:** You are never locked into our platform. Your exported data can be used for your own analysis or migrated to other services that support these standard formats.



## Third-Party Vendor Services Used

We use the following third-party providers for infrastructure and administrative tasks prioritize, zero knowledge principles, even when third-party infrastructure is used at your core data, remains encrypted and inaccessible to us and our vendors:

- **Payments:** We use **[Stripe]()** to process subscriptions. We do not store any billing or personal details on our servers. We only store a Stripe customer id and subscription id connected to your workspace.
- **Communications:** Transactional emails (account invites and response notifications) are delivered via **[Resend]()**. 
- **Response Forwards:** For PGP-enabled forwarding, responses are encrypted client-side using your public key before being handed to **[Resend]()** for delivery. Resend acts as a blind carrier and cannot read the content but they can read the subject and recipient email address.
- **Hosting:** Our application servers are hosted by **[Hostinger]()**.
- **Network Security:** We use **[Cloudflare]()** as a Content Delivery Network (CDN) and to protect the platform against DDoS attacks.
- **Primary Database:** We self-host a **[PostgreSQL]()** instance on our private infrastructure. All sensitive data is encrypted before it reaches the database.
- **File Storage & Backups:** All user-uploaded files and database backups are stored as **encrypted blobs** using **[RustFS]()** on our private hardware.

> **The Zero-Knowledge Guarantee:** Because encryption happens at the application level, your form schemas, responses, and file uploads are unreadable to Hostinger, Cloudflare, and even us. We hold the infrastructure, but you hold the keys.



## Location of site and data / residency

- We are based in the **United States** and must comply with U.S. laws. This has two practical implications for your data:

  - **Legal Compliance:** We are subject to U.S. legal processes, such as subpoenas or court orders. If we receive a valid legal request, we must comply.
  - **Infrastructure:** Our application servers, self-hosted database, and encrypted backups are located on infrastructure within the **United States**.

  **The Zero-Knowledge Protection:** While we are subject to U.S. jurisdiction, our technology limits what we can provide. If law enforcement requests your form data or responses, we can only provide the **encrypted blobs**. Because we do not hold your decryption keys, we have no technical way to turn that data into readable text for any third party.



### Global Privacy Rights (EU, California, and Beyond)

While we are a U.S.-based service, we recognize and respect privacy rights globally, including the **GDPR** (Europe) and **CCPA/CPRA** (California). Regardless of your location, we provide the following protections to all users:

- **Right to Access and Portability:** You can view and export your data at any time.
- **Right to Erasure:** You can delete your account and all associated data instantly through your dashboard.
- **Right to Object:** Since we do not sell your data or use it for profiling, your right to object is built into our core architecture.



## Technical Transparency

| **Data Type**    | **Encryption State**                | **Who has the keys?** |
| ---------------- | ----------------------------------- | --------------------- |
| Form Content     | **Client-Side Encrypted**           | You (The User)        |
| Responses        | **Client-Side Encrypted**           | You (The User)        |
| Account Metadata | **Server-Side Encrypted (at rest)** | The Platform (We do)  |
| Payment Info     | **External**                        | Stripe                |



## Questions?

If you have questions about this policy or how your data is handled, please reach out to us at [privacy@useconfide.app](mailto:security@useconfide.app).

Confide LLC