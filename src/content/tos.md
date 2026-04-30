# Terms of Service

**Effective Date:** April 29, 2026

Welcome to Confide ("Service"), operated by Confide LLC ("we", "us", or "our"). By using the Service, you agree to these Terms.

---

## 1. Eligibility

You must be at least 18 years old and legally capable of entering into a binding contract to use the Service.

If you are using the Service on behalf of an organization, you represent that you have the authority to bind that organization to these Terms, and "you" refers to both you and that organization.

By using the Service, you represent that you meet these requirements.

---

## 2. Overview of the Service

The Service is a privacy-focused, encrypted form builder that allows users to create, share, and manage forms and responses using end-to-end encryption.

We offer:

* A hosted (managed) version of the Service
* Open-source software licensed under the GNU Affero General Public License (AGPL), which users may self-host independently

These Terms apply only to the hosted Service.

Specific features and limits depend on the plan you subscribe to and may change from time to time.

---

## 3. Accounts & Authentication

Authentication is performed using passkeys (e.g., device credentials, password managers, or hardware keys). We do not store passwords.

Upon account creation, you will be issued a one-time recovery code. This code can be used to regain access to your account if you lose your passkey. Each recovery code can only be used once and is invalidated after use. You may regenerate a new recovery code at any time from your account settings.

You are responsible for:

* Maintaining access to your authentication methods
* Storing your recovery code securely
* Securing your devices and credentials

If you lose both your passkey and your recovery code, we cannot recover access to your account.

---

## 4. Encryption & Data Access

The Service is designed to be end-to-end encrypted:

* Encryption keys are generated and stored on your device
* We store only encrypted data and public keys
* We cannot access, read, or decrypt your content

### Team Workspaces

* Workspace data is encrypted using shared workspace keys
* Workspace keys are encrypted for each authorized member

### Public Forms

* When a form is shared, the share URL includes an encryption key fragment
* This fragment allows respondents to decrypt the form questions and encrypt their responses
* Only authorized workspace members with the corresponding keys can decrypt submitted responses

### Important Limitation

If you lose access to your passkey, your recovery code, or your encryption keys:

* Your data may be permanently unrecoverable
* We cannot restore or decrypt your data

You are responsible for storing your recovery code securely. Losing both your passkey and your recovery code will result in permanent loss of access to your account and data.

---

## 5. User Responsibilities

You are solely responsible for:

* The content you create, collect, or distribute
* Compliance with applicable laws and regulations
* Managing access to your data and encryption keys
* Ensuring that forms do not collect data from children under 13 without proper parental consent

If you believe a form may have inadvertently collected children's data, contact us immediately at legal@useconfide.app and we will assist with deletion.

---

## 6. Acceptable Use & Abuse

You agree not to use the Service to:

* Violate any laws or regulations
* Distribute illegal, harmful, or abusive content
* Engage in phishing, fraud, or malicious activity
* Engage in spam or coordinated inauthentic behavior
* Interfere with the security or integrity of the Service
* Circumvent any technical restrictions or security measures of the Service
* Infringe on intellectual property rights of others

Due to the end-to-end encrypted nature of the Service:

* We do not have access to the contents of forms or responses
* We are unable to review or moderate encrypted user data

As a result:

* Abuse investigations may be limited
* We may not be able to verify or resolve complaints involving encrypted content

We will respond to valid legal requests where technically feasible. However, due to encryption, only limited metadata (such as account information, usage patterns, and timestamps) may be available.

We may take action based on available signals, including:

* Account metadata
* Usage patterns
* External reports

This may include suspending or terminating accounts at our sole discretion.

---

## 7. Copyright & Intellectual Property

We respect intellectual property rights. If you believe content accessible through the Service infringes your copyright, you may submit a notice to our designated DMCA agent.

### Submitting a DMCA Notice

Send your notice to: legal@useconfide.app

Your notice must include:

* Description of the copyrighted work you claim has been infringed
* Location or description of the allegedly infringing material
* Your contact information (name, address, email, phone number)
* A statement that you have a good faith belief that the use is not authorized
* A statement, under penalty of perjury, that the information in your notice is accurate and that you are authorized to act on behalf of the copyright owner
* Your physical or electronic signature

### Limitations

Due to end-to-end encryption:

* We cannot access or review encrypted form content
* Copyright claims may only be actionable for publicly accessible forms or metadata
* We will investigate based on available information and may remove access or terminate accounts where appropriate

### Designated DMCA Agent

Confide LLC  
legal@useconfide.app

---

## 8. Data Export & Portability

You may export your data at any time:

* Form responses can be exported in CSV format
* Exports include all accessible form data for which you have decryption keys

---

## 9. Fees & Billing

Some features require payment.

* Fees are billed as described at purchase
* Subscriptions automatically renew at the end of each billing period unless cancelled beforehand
* Payments are processed through third-party payment processors (currently Stripe, may include Paddle in the future)
* Payments are non-refundable except where required by law
* We may change pricing with at least 30 days' notice

### Service Limitations

Depending on your plan, the following limitations may apply:

* Storage capacity limits
* Email forwarding rate limits
* API rate limiting
* Monthly usage caps

Specific limits are detailed in your plan documentation and may be updated with reasonable notice.

---

## 10. Data Deletion

Upon account deletion:

* Encrypted data and metadata are deleted immediately from active systems
* Backups (including both encrypted data and metadata) are retained for up to 7 days before permanent deletion
* After 7 days, all backups are permanently deleted

Deleted data cannot be recovered.

---

## 11. Third-Party Services

The Service uses third-party providers for certain functions:

* **Payment processing:** Stripe (and potentially Paddle)
* **Hosting infrastructure:** Hostinger
* **Domain registration:** Namecheap
* **DNS and security:** Cloudflare
* **Transactional emails:** Resend

These providers operate under their own terms of service and privacy policies. We select providers that maintain appropriate security and privacy standards.

---

## 12. Service Availability

The Service is provided on an "as is" and "as available" basis.

We do not guarantee:

* Continuous availability
* Error-free operation
* Data accessibility

We may modify or discontinue the Service at any time.

---

## 13. Termination

You may stop using the Service at any time.

We may suspend or terminate access if:

* You violate these Terms
* Required by law
* Necessary to protect the Service or other users

Upon termination, you will immediately lose access to your account and encrypted data. The data deletion policy described in Section 10 applies — encrypted data and metadata will be deleted from active systems immediately, with backups permanently deleted within 7 days. We recommend exporting your data before closing your account.

---

## 14. Indemnification

You agree to indemnify and hold harmless [Your Company Name], its officers, directors, employees, and agents from any claims, damages, losses, liabilities, and expenses (including reasonable legal fees) arising from:

* Your use of the Service
* Your violation of these Terms
* Your violation of any law or third-party rights
* Content you create, collect, or distribute through the Service
* Any breach of your representations and warranties

---

## 15. Disclaimer of Warranties

To the fullest extent permitted by law, the Service is provided without warranties of any kind, including:

* Fitness for a particular purpose
* Reliability or availability
* Data accessibility or recovery
* Merchantability
* Non-infringement

---

## 16. Limitation of Liability

To the fullest extent permitted by law, we are not liable for:

* Loss of data (including loss due to lost encryption keys)
* Indirect, incidental, or consequential damages
* Loss of business, revenue, or profits
* Service interruptions or data breaches
* Any damages exceeding the amount you paid us in the 12 months preceding the claim

---

## 17. Governing Law & Disputes

These Terms are governed by the laws of the Commonwealth of Virginia, without regard to its conflict of law provisions.

Any disputes arising from these Terms or your use of the Service shall be resolved in the state or federal courts located in Virginia. You consent to the personal jurisdiction of such courts.

---

## 18. Survival

The following sections survive termination of these Terms:

* Section 5 (User Responsibilities)
* Section 7 (Copyright & Intellectual Property)
* Section 14 (Indemnification)
* Section 15 (Disclaimer of Warranties)
* Section 16 (Limitation of Liability)
* Section 17 (Governing Law & Disputes)

---

## 19. Changes to These Terms

We may update these Terms from time to time. We will notify you of material changes by:

* Posting the updated Terms with a new effective date
* Sending notice to your registered email address (if applicable)

Continued use of the Service after changes take effect constitutes acceptance of the updated Terms.

---

## 20. Contact

For questions about these Terms, contact:

**Confide LLC**  
legal@useconfide.app

For DMCA notices, use the contact information in Section 7.
