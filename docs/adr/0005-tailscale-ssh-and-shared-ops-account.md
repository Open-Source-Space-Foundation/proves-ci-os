# Reach the machine over Tailscale SSH with a shared `ops` account, and disable `sshd`

- Status: accepted

Contributors investigate a CI machine over Tailscale SSH (`sudo tailscale up --ssh`). The tailnet identity is the credential: no `authorized_keys`, no key sync, no keys on the machine. They log in as a shared `ops` account. `runner` has a `nologin` shell and no ssh path. `sshd.service` is **disabled** in the Image and a structure test asserts it.

## Why `sshd` is disabled

`fedora-bootc:44` enables `sshd` (Fedora preset), listens on all interfaces, allows password authentication and ships no firewall. A console password for `ops` would therefore also work over ssh from the LAN. Tailscale SSH runs inside `tailscaled` and does not need OpenSSH.

## Consequences

- Good: no key management and no LAN-reachable ssh listener.
- Bad: access depends on the tailnet ACL, which the Image cannot enforce. The sample ACL must name `users: ["ops"]` explicitly and must never use `autogroup:nonroot`, which would let a contributor become `runner` and read CI secrets. The docs warn about this prominently.
- Bad: `ops` can become root with the console password, so the accepted trade is that `ops` can read runner logs and job checkouts through group membership (`systemd-journal`, `runner`) and `UMask=0027`.
- If a machine has not joined a tailnet, the console is the only way in. The console password is set at install, never baked, and is the basis of the documented recovery process.
