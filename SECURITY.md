# Security policy

The latest published release is supported for security fixes. Probelune is early-stage software; use a private deployment and least-privilege node policies while evaluating it.

Report vulnerabilities through this repository's **Security → Advisories → Report a vulnerability** feature. If private reporting is unavailable, contact a maintainer through the organization profile without posting exploit details publicly. Include the affected version, impact, minimal reproduction and any suggested fix. Do not attach live secrets or customer data.

We will assess reports and coordinate a fix and disclosure; no response-time guarantee or paid bounty is offered. Please allow a reasonable private remediation window before publishing details.

Use HTTPS, separate node credentials, explicit private CIDRs, protected key/data directories and encrypted backups. Treat access to the dashboard encryption key as access to all saved secrets. Monitoring arbitrary endpoints is an intentional capability: grant operator and node administration roles only to trusted people. Native push delivery requires a separately configured Firebase/APNs deployment.
