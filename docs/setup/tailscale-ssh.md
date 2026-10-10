# Tailscale SSH

You reach a CI machine over [Tailscale SSH](https://tailscale.com/kb/1193/tailscale-ssh). Your tailnet identity is the credential: there are no `authorized_keys` and no keys on the machine. OpenSSH (`sshd`) is **disabled** in the Image, so nothing listens for ssh on the LAN. See [ADR 0005](../adr/0005-tailscale-ssh-and-shared-ops-account.md).

Which tailnet to use is your choice. Its access rules decide who may log in.

## Join the tailnet

At the machine's console, log in as `ops` with the console password you set at install, then:

```sh
sudo tailscale up --ssh
```

Open the printed URL on another device and approve the machine. Check with `tailscale status`.

## Accounts

Contributors log in as the shared **`ops`** account. It is in `wheel` (sudo with the console password) and in the `systemd-journal` and `runner` groups, so it can read Runner logs and job checkouts. The **`runner`** account has `nologin`, no ssh path and no password.

## Access rules (ACL)

Allow your contributors to ssh to CI machines as `ops`, and only `ops`. Tag the machines when you bring them up (`sudo tailscale up --ssh --advertise-tags=tag:ci-machine`) and use a rule like:

```json
{
  "tagOwners": { "tag:ci-machine": ["group:proves-admins"] },
  "ssh": [
    {
      "action": "accept",
      "src": ["group:proves-contributors"],
      "dst": ["tag:ci-machine"],
      "users": ["ops"]
    }
  ]
}
```

> **Warning:** never use `autogroup:nonroot` in `users`. It lets a contributor log in as `runner`, which can read every CI secret. Always name `ops` explicitly.

## Connect

```sh
ssh ops@<hostname>
```

If the machine is not on a tailnet, the console is the only way in. See [recovery](../operations/recovery.md).
