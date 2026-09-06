# Security reporting

Do not put credentials or exploit details in public issues.
Use [GitHub private vulnerability reporting](https://github.com/DiLRandI/confgen/security/advisories/new)
when available. If reporting is not enabled, ask the maintainer for a private
contact in an issue without including vulnerability details.

Include the affected revision, a minimal reproduction with dummy values, and
the impact. No response-time guarantee is offered. Fixes are proposed through
pull requests; release timing remains maintainer-controlled.

Security fixes target the latest release and current development branch. Older
releases have no separate maintenance commitment; upgrade to the latest release
when a fix is published.

Before v1.0.0, generated-code and runtime APIs may change between v0.x
releases. Breaking changes must follow pre-v1 semantic-version expectations
and be described in release notes. Regenerate consumers when upgrading.
Compatibility guarantees and dedicated compatibility testing belong in the
preparation for v1.0.0.

Diagnostics redact configuration values, but secrets in returned configuration
structs remain the application's responsibility. Avoid logging or serializing
those structs. Generated source can contain schema defaults: never put secrets
in defaults, and review inferred values before using `--copy-defaults`.
