# Security Policy

## Supported Versions

The latest `main` and the most recent tagged release receive security fixes.

## Reporting a Vulnerability

Do **not** open a public issue for security reports. Use GitHub's private vulnerability
reporting (Security → Report a vulnerability). Expect an acknowledgement within 72 hours.

## Scope

- Auth handshake (JWT validation, expiry)
- Message authorization (membership checks on every SEND/READ)
- Rate limiting and input validation (frame size cap, per-connection token bucket)
- WebSocket `Origin` validation on upgrade
- Dependency vulnerabilities

## Handling

Confirmed issues are fixed on a private branch, released, then disclosed.
