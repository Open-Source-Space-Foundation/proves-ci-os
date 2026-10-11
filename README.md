# PROVES CI OS

An immutable operating system image whose only job is to be a dedicated CI machine for building and testing the [PROVES core reference](https://github.com/Open-Source-Space-Foundation/proves-core-reference) flight software.

Every CI machine boots the same signed image, updates itself weekly when idle, and rolls back with one command. There is nothing to configure by hand beyond joining a tailnet and registering a runner.

## Why

A hand-built CI machine drifts, cannot be reproduced and has no rollback. This repository defines the machine as a `Containerfile` (a [bootc](https://containers.github.io/bootc/) image on Fedora), so setting up another one means installing the image, not following a wiki page. See [the decisions](docs/adr/) for the reasoning.

## Quickstart

You need an x86_64 UEFI machine on wired Ethernet, a USB stick, a Tailscale account and an organization owner to mint a registration token. Everything on the machine's disk is erased.

1. **Get the installer ISO** from the latest release and verify its checksum and signature ([install](docs/setup/install.md)).
2. **Install**: write the ISO to a USB stick, boot the machine from it and answer the three prompts (console password, hostname, target disk).
3. **Join your tailnet**: log in at the console as `ops` and run `sudo tailscale up --ssh` ([Tailscale SSH](docs/setup/tailscale-ssh.md)).
4. **Get a registration token** from an organization owner ([runner](docs/setup/runner.md)).
5. **Register the runner**: `sudo proves-runner register --labels <labels>`, with the token on stdin.
6. **Check it is online** in the organization's runner list, then run a job.

## Documentation

| | |
| --- | --- |
| Setup | [install](docs/setup/install.md), [Tailscale SSH](docs/setup/tailscale-ssh.md), [runner](docs/setup/runner.md) |
| Operations | [updates and rollback](docs/operations/updates.md), [recovery](docs/operations/recovery.md) |
| Reference | [security](docs/security.md), [glossary](CONTEXT.md), [decisions](docs/adr/), [contributing](CONTRIBUTING.md) |

## Scope

The Image is published for x86_64 and aarch64. The installer ISO and Raspberry Pi boot are still x86_64 only until the Pi 4 boot is proven (ADR 0009).
