# Security Policy

## Supported versions

docent is pre-1.0; only the latest release receives security fixes.

## Reporting a vulnerability

Report vulnerabilities privately through
[GitHub Security Advisories](https://github.com/matcra587/docent/security/advisories/new)
— do not open a public issue for anything you believe is exploitable.

You can expect an acknowledgment within a week. Once a fix ships, the
advisory is published and credited unless you prefer otherwise.

## Scope

docent is a library with no network access, no credential handling, and no
binary of its own. The most security-relevant surface is skill export, which
writes into host-supplied directories: writes are `os.Root`-scoped so path
components resolving outside the target directory (symlinks, `..`, absolute
paths) are refused at open time. Reports about escaping that boundary are
especially welcome.
