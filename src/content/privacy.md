# Privacy Policy

**Effective Date:** April 29, 2026

Confide LLC ("we", "us", or "our") operates Confide ("Service"), a privacy-focused, end-to-end encrypted form builder. This Privacy Policy explains how we handle information in connection with the Service.

---

## 1. Our Privacy Philosophy

We designed the Service with privacy as a core principle:

* **End-to-end encryption:** Your form content and responses are encrypted on your device before reaching our servers
* **Zero-knowledge architecture:** We cannot read, access, or decrypt your encrypted data
* **Minimal data collection:** We collect only what is necessary to operate the Service
* **No data sales:** We do not sell, rent, or share your personal information with third parties for marketing purposes

---

## 2. Information We Collect

### 2.1 Account Information

When you create an account, we collect:

* **Authentication credentials:** Passkey public keys (cryptographic keys stored on your device)
* **Email address:** Optional for free accounts. Required if you purchase a subscription (collected by our payment processor, Stripe, for billing purposes)
* **Account metadata:** Account creation date, last login timestamp

**We do not collect or store passwords.** Authentication is handled entirely through passkeys.

### 2.2 Encrypted Content (Zero-Knowledge Data)

The Service stores the following encrypted data that we **cannot access, read, or decrypt**:

* Form structures and questions
* Form responses and submissions
* Workspace names and settings
* User-generated content within forms

**Encryption keys** are generated and stored exclusively on your devices. We store only:

* Encrypted data blobs
* Public keys (used for sharing encrypted data with authorized users)
* Encrypted key fragments (for secure form sharing)

### 2.3 Metadata & Usage Information

We collect limited metadata to operate the Service:

* **Usage metrics:** Number of forms created, submission counts, storage usage
* **Access logs:** IP addresses, timestamps, browser type, device type
* **Billing information:** Payment history, subscription tier (payment details are handled by our payment processors and not stored by us)
* **Email delivery logs:** Timestamps and delivery status for transactional emails

### 2.4 Information from Third Parties

We receive limited information from third-party services:

* **Payment processors (Stripe, Paddle):** Transaction confirmations, payment status (we do not receive full credit card numbers)
* **Email service (Resend):** Email delivery status and bounce notifications

---

## 3. How We Use Information

We use collected information solely to:

* **Operate the Service:** Authenticate users, store encrypted data, deliver forms
* **Process payments:** Manage subscriptions and billing
* **Send transactional emails:** Account notifications, form submission alerts, security updates
* **Improve the Service:** Analyze usage patterns (in aggregate) to identify bugs and improve performance
* **Enforce our Terms:** Detect abuse, respond to legal requests, protect users
* **Provide customer support:** Respond to inquiries and troubleshoot issues

**We do not:**

* Use your data for advertising or marketing (except to communicate about our own Service)
* Sell or rent your personal information
* Access or analyze the content of your encrypted forms or responses
* Share your data with third parties except as described in Section 5

---

## 4. Legal Basis for Processing (GDPR)

For users in the European Economic Area (EEA), United Kingdom, or Switzerland, we process your information under the following legal bases:

* **Contract performance:** Processing necessary to provide the Service you've requested
* **Legitimate interests:** Operating, securing, and improving the Service
* **Legal obligation:** Complying with applicable laws and regulations
* **Consent:** Where explicitly obtained for specific purposes (e.g., optional marketing emails)

---

## 5. Information Sharing & Disclosure

We share information only in the following limited circumstances:

### 5.1 Third-Party Service Providers

We share minimal information with third parties who help us operate the Service:

| Provider | Purpose | Data Shared |
|----------|---------|-------------|
| **Stripe / Paddle** | Payment processing | Email, transaction amounts, payment status |
| **Hostinger** | Server hosting | Encrypted data blobs, metadata (hosted on our VPS) |
| **Cloudflare** | DNS, security, CDN | IP addresses, request metadata |
| **Resend** | Transactional emails | Email addresses, message content (for delivery only) |
| **Namecheap** | Domain registration | No user data shared |

These providers are bound by contract to use data only for providing services to us and to maintain appropriate security measures.

### 5.2 Legal Requirements

We may disclose information if required to:

* Comply with valid legal processes (subpoenas, court orders, warrants)
* Enforce our Terms of Service
* Protect the rights, property, or safety of our users or the public
* Investigate fraud, security issues, or Terms violations

**Important limitation:** Due to end-to-end encryption, we can only provide:

* Account metadata (email, creation date, login timestamps)
* Usage patterns (number of forms, submission counts)
* Access logs (IP addresses, timestamps)
* Encrypted data blobs (which we cannot decrypt)

We cannot provide the plaintext content of forms or responses.

### 5.3 Business Transfers

If we are involved in a merger, acquisition, or sale of assets, your information may be transferred. We will notify you before your information becomes subject to a different privacy policy.

### 5.4 Aggregated & De-identified Data

We may share aggregated or de-identified data that cannot reasonably be used to identify you (e.g., "500 forms were created this month").

---

## 6. Data Retention

### 6.1 Active Accounts

We retain your information for as long as your account is active or as needed to provide the Service.

### 6.2 Account Deletion

When you delete your account:

* **Immediate deletion:** All encrypted data and metadata are removed from active systems
* **Backup retention:** Backups (including encrypted data and metadata) are retained for up to **7 days**
* **Permanent deletion:** After 7 days, backups are permanently deleted and cannot be recovered

### 6.3 Legal Holds

We may retain information longer if required by law or for legitimate legal purposes (e.g., ongoing litigation, regulatory investigations).

---

## 7. Data Security

We implement industry-standard security measures:

### 7.1 Encryption

* **End-to-end encryption:** All form content is encrypted on your device using AES-256
* **Transport security:** All data in transit uses TLS 1.2+
* **At-rest encryption:** Server storage uses encrypted file systems

### 7.2 Access Controls

* **Principle of least privilege:** Staff access to systems is limited to what's necessary
* **No content access:** Our team cannot decrypt or access form content
* **Audit logging:** System access is logged and monitored

### 7.3 Infrastructure Security

* **Secure hosting:** Servers hosted with reputable providers (Hostinger)
* **DDoS protection:** Cloudflare protects against attacks
* **Regular updates:** Security patches applied promptly

**No security is perfect.** While we implement strong safeguards, no method of transmission or storage is 100% secure. You use the Service at your own risk.

---

## 8. Your Privacy Rights

### 8.1 Access & Portability

You have the right to:

* Access your account information and metadata
* Export your form responses in CSV format
* Request a copy of your account data

### 8.2 Correction & Deletion

You can:

* Update your email address and account settings
* Delete your account at any time (see Section 6.2 for deletion timeline)

### 8.3 Additional Rights (GDPR, CCPA)

Depending on your location, you may have additional rights:

**GDPR (EEA, UK, Switzerland):**

* Right to rectification
* Right to erasure ("right to be forgotten")
* Right to restrict processing
* Right to data portability
* Right to object to processing
* Right to withdraw consent
* Right to lodge a complaint with a supervisory authority

**CCPA (California):**

* Right to know what personal information is collected
* Right to delete personal information
* Right to opt-out of sale (note: we do not sell personal information)
* Right to non-discrimination for exercising CCPA rights

**To exercise these rights,** contact us at legal@useconfide.app.

---

## 9. Data Protection Limitations

### 9.1 Encryption Key Loss

Upon account creation, you are issued a one-time recovery code that can be used to regain access if you lose your passkey. Each code can only be used once and can be regenerated from your account settings.

If you lose access to both your passkey and your recovery code:

* **We cannot recover your account or data** — the encryption is designed to prevent this
* **Your data becomes permanently inaccessible**
* We are not liable for data loss due to lost authentication methods or encryption keys

This is a fundamental characteristic of end-to-end encryption and zero-knowledge architecture.

### 9.2 Content Moderation

Due to encryption, we cannot:

* Review the content of forms or responses
* Detect illegal content within encrypted data
* Moderate user-generated content proactively

We rely on metadata, usage patterns, and external reports to detect abuse.

---

## 10. Children's Privacy

The Service is not intended for individuals under 18 years of age.

We do not knowingly collect personal information from children under 13 (or the applicable age in your jurisdiction).

**If you believe a form has collected children's data:**

* Contact us immediately at legal@useconfide.app
* We will work with the form owner to delete the data
* Form owners are responsible for compliance with children's privacy laws (e.g., COPPA)

---

## 11. International Data Transfers

Our servers are located in the United States.

If you access the Service from outside this region, your information may be transferred to, stored, and processed in a different country.

For users in the EEA, UK, or Switzerland:

* We rely on standard contractual clauses or other approved transfer mechanisms
* Your encrypted data is protected by end-to-end encryption regardless of where it's stored

---

## 12. Cookies & Tracking Technologies

### 12.1 Cookies We Use

We use the following types of cookies:

* **Essential cookies:** Required for authentication and core functionality
* **Session cookies:** Temporary cookies deleted when you close your browser
* **Preference cookies:** Remember your settings and preferences

### 12.2 Third-Party Cookies

Our third-party providers may set cookies:

* **Cloudflare:** Security and performance cookies
* **Stripe / Paddle:** Payment processing cookies (on checkout pages only)

### 12.3 Analytics

We do not use third-party analytics tools that track you across websites (e.g., Google Analytics).

We may use privacy-respecting analytics that:

* Do not use cookies
* Do not track users across sites
* Collect only aggregated, anonymous usage data

### 12.4 Do Not Track

We honor Do Not Track (DNT) signals where technically feasible.

---

## 13. Your Choices

You can control your information:

* **Account settings:** Update email, manage authentication methods
* **Email preferences:** Opt out of non-essential emails (you'll still receive critical security and billing notices)
* **Data export:** Export your form responses at any time
* **Account deletion:** Delete your account and all associated data
* **Cookie settings:** Most browsers allow you to control cookies through settings

---

## 14. Open Source & Self-Hosting

The Service is built on open-source software licensed under the GNU Affero General Public License (AGPL).

If you self-host the software:

* This Privacy Policy does not apply to your self-hosted instance
* You are responsible for your own privacy practices and compliance
* You control all data processing and storage

This Privacy Policy applies **only to the hosted version** of the Service operated by us.

---

## 15. Changes to This Privacy Policy

We may update this Privacy Policy from time to time.

When we make material changes, we will:

* Update the "Effective Date" at the top
* Notify you via email (if you've provided one)
* Post a notice in the Service

Continued use of the Service after changes take effect constitutes acceptance of the updated Privacy Policy.

---

## 16. Contact Us

For questions, concerns, or to exercise your privacy rights:

**Confide LLC**  
**Privacy Contact:** legal@useconfide.app

**For GDPR requests:** legal@useconfide.app  
**For CCPA requests:** legal@useconfide.app

**Response time:** We aim to respond to privacy requests within 30 days (or as required by applicable law).

---


## Appendix: Data Processing Summary

| Data Type | How We Process | Can We Access It? | Retention |
|-----------|----------------|-------------------|-----------|
| Passkey public keys | Stored in database | Yes (public keys only) | Until account deletion + 7 days |
| Email address | Stored in database, shared with email provider | Yes | Until account deletion + 7 days |
| Form content | Stored encrypted | **No** (encrypted client-side) | Until account deletion + 7 days |
| Form responses | Stored encrypted | **No** (encrypted client-side) | Until account deletion + 7 days |
| Access logs | Logged for security | Yes | 90 days |
| Payment data | Processed by Stripe/Paddle | No (handled by payment processor) | Transaction history retained per billing requirements |
| Usage metadata | Aggregated for analytics | Yes (aggregated only) | Indefinitely (anonymous) |
