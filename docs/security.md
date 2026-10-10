# Security

## Trust model

A CI machine runs the code of every workflow that targets it, with access to the CI secrets those workflows receive. Anyone who can run a job on it can read those secrets and use the attached hardware. Anyone who controls the Image controls every machine that follows it. The design assumes:

- The registry content is not trusted until its signature verifies against the baked public key ([ADR 0003](adr/0003-verified-signed-updates.md)). A machine rejects an unsigned or wrongly signed image at `bootc upgrade`.
- The signing key is held only as secrets in the `release` GitHub environment, restricted to `main`. PR jobs and forks cannot read it.
- Contributors who can ssh in are trusted with the CI secrets on that machine (see below).

## Accounts

- **`runner`** runs jobs. `nologin`, no password, no ssh.
- **`ops`** is the shared contributor account. `sudo` needs the console password. It is in the `runner` and `systemd-journal` groups, so it can read job checkouts and logs. Treat `ops` access as access to the runner's secrets.
- `sshd` is disabled. Access is Tailscale SSH only, governed by your tailnet ACL, which the Image cannot enforce. Name `users: ["ops"]` explicitly and never use `autogroup:nonroot` ([Tailscale SSH](setup/tailscale-ssh.md)).

## Restrict the runner group

An organization runner can be used by any repository the runner group allows. Put CI machines in a **dedicated runner group**, allow only the repositories that need them (for the core reference, only that repository), and register with `--runnergroup`. Do not leave rigs in the default group.

## Fork pull requests

A pull request from a fork can run arbitrary code on your machine. Do not run fork PRs on these runners without approval:

- Set the repository's *Fork pull request workflows* policy to require approval for all outside collaborators.
- Never use `pull_request_target` to check out and run fork code on a self-hosted runner.
- Keep hardware-in-the-loop (`integration`) jobs off fork PRs entirely: they hold a power supply and flight boards.

## Secrets

- The registration token is minted per registration, lives an hour, and is read from stdin or a file, never from arguments or the Image.
- The console password is set at install and never baked in.
- No secret is stored in this repository. `cosign.pub` and the baked public key are public by design.

## Reporting a problem

Open an issue marked as security-sensitive without exploit details, or contact an organization owner directly.
