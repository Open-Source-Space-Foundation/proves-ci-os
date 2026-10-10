# Contributing

This repository defines the PROVES CI machine Image. The decisions behind it are in [docs/adr](docs/adr/) and the vocabulary is in [CONTEXT.md](CONTEXT.md); read the relevant ADR before changing what it covers, and add or supersede an ADR for a new decision.

## Dev shell

Dependencies come from Nix. With [Nix](https://nixos.org/download/) and [direnv](https://direnv.net/) installed, `cd` into the repository and run `direnv allow`; otherwise run `nix develop`. The shell provides `gnumake`, `hadolint`, `container-structure-test`, `pre-commit`, `go`, `golangci-lint`, `cosign`, `skopeo`, `oras`, `renovate`, `rumdl` and `codespell`, and installs the git hooks on entry. `podman` is installed separately (a Linux VM on macOS).

Commit from inside the shell: the hooks run tools it provides.

## Make targets

| Target | What it does |
| --- | --- |
| `make help` | List targets. |
| `make fmt` | Run every pre-commit hook on all files. |
| `make test` | `go test ./...`. |
| `make build` | Build the Image with podman. CI is the authoritative build. |
| `make check-renovate` | Check every pinned `ARG` has exactly one Renovate manager and validate `renovate.json`. |
| `make iso` | Build the installer ISO. |
| `make all` | All of the above except `iso`. |

`make fmt` must pass on a clean clone before you open a pull request.

## Layout

- `Containerfile`: the Image. Pins are `ARG`s tracked by Renovate ([ADR 0002](docs/adr/0002-automated-dependency-updates.md)).
- `files/`: mirrors the Image's filesystem and is copied into it (units, udev rules, sysusers, policy).
- `cmd/` and `internal/`: Go programs baked into the Image (`proves-runner`, `proves-update`) and the pin check.
- `tests/structure/`: container-structure-test files the built Image must pass.
- `iso/`: installer ISO configuration.

## Code

Go, static binaries (`CGO_ENABLED=0`). No discarded errors, secrets on stdin only, every loop bounded. Tests are `go test`. Match the surrounding style.

## Dependencies

Do not bump pins by hand; Renovate does it and CI gates every PR. Non-major updates automerge once CI passes. The Fedora base is capped at the next general-availability release; raise the cap by hand.

## Issues

Work is tracked in GitHub Issues with the labels `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human` and `wontfix`.
