# Build on an immutable bootc operating system

- Status: accepted

The Image is a headless `fedora-bootc:44` container image, defined entirely by a `Containerfile`, booted directly, and updated atomically with `bootc`. A CI machine does one job and must be reproducible: anyone who installs the Image gets the same machine, and a bad update rolls back with `bootc rollback`. The base is pinned by digest and Renovate is capped at Fedora 44 until the next release is generally available (see [ADR 0002](0002-automated-dependency-updates.md)).

Derived from [stableOS ADR 0001](https://github.com/nateinaction/stableOS/blob/main/docs/adrs/0001-immutable-operating-system.md), narrowed to a headless CI machine: no desktop, wired networking through NetworkManager only, x86_64 only for the first release.

## Considered Options

- **bootc image on Fedora**: ordinary `Containerfile`, `dnf`, OCI registries and signing. Chosen for mainstream tooling and low authoring cost.
- **NixOS**: stronger end-to-end pinning, but the whole OS must be written in Nix. Nix is still used for the dev shell, not for the OS.
- **A mutable Linux install** (what the current CI machine is): drifts, has no atomic rollback, and cannot be reproduced.

## Consequences

- Good: atomic upgrades, one-command rollback, no drift, updates testable in CI before any machine sees them.
- Good: the only customization path is the `Containerfile`, so every CI machine boots an identical image. Local package layering is not used.
- Bad: RPM installs are only as pinned as the base digest and the repository state at build time.
- Bad: changing a CI machine means rebuilding the Image. Writable state must be routed to `/var` (the runner and its work directory live there).
