# Security Policy

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, report them privately:

- Use **[GitHub Security Advisories](https://github.com/khanakia/browserctrl/security/advisories/new)** (preferred), or
- Email **khanakia@gmail.com** with the details.

Please include:

- A description of the vulnerability and its impact.
- Steps to reproduce (proof-of-concept if possible).
- Affected version(s) and environment.

We will acknowledge your report within a few days and keep you updated on remediation. We ask that you give us a reasonable window to release a fix before any public disclosure, and we're happy to credit you in the advisory.

## What this tool touches

browserctrl reads browser profile directories and prints profile names, signed-in emails, display names and extension device ids. It never writes to a profile, never sends anything over the network, and never opens a live extension store (each is copied to a temp directory, opened read-only, and the copy removed before exit). A report that shows any of those statements to be false is in scope even if no exploit is attached.

## Supported versions

Security fixes are applied to the latest released version. Older versions may not receive patches; please upgrade to stay supported.

| Version | Supported |
|---|---|
| latest | ✅ |
| older | ❌ |
