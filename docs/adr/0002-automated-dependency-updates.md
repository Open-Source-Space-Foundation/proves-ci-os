# Keep pinned dependencies current with Renovate

- Status: accepted

Everything the Image depends on is pinned: the base image digest, GitHub Actions (by SHA), pre-commit hooks, the flake lock, the OpenOCD ref, the `korad_control` commit, and the runner seed tarball (version and sha256). Pins are reproducible but go stale, and stale pins miss security fixes. Renovate (`renovate.json`) moves them forward with `pinDigests: true`.

Derived from [stableOS ADR 0002](https://github.com/nateinaction/stableOS/blob/main/docs/adrs/0002-automated-updates.md). That ADR covers dependency updates, not image updates on a CI machine; those are in [ADR 0006](0006-idle-gated-image-updates.md).

## Decision

- Managers: `dockerfile` (the `Containerfile`), `github-actions`, `pre-commit`, `nix`, plus custom regex managers for the source-build and seed pins held in `ARG`s.
- The base image is capped with `allowedVersions: "<=44"` so Renovate proposes the next Fedora only after general availability. The cap is raised by hand.
- Non-major updates automerge once CI passes; major updates need a human.
- A CI check (a small Go tool) asserts that every pinned `ARG` is matched by exactly one custom manager, and `renovate-config-validator --strict` gates every PR. A reformatted `Containerfile` otherwise stops pins from being bumped, and nothing would notice.

## Consequences

- Good: pins stay current with little manual effort, and CI gates every Renovate PR like any other.
- Bad: automerge trusts the structure tests. A gap in them can let a bad non-major update land.
- Bad: the Fedora cap is a manual knob; forgetting it stalls base-image updates silently.
