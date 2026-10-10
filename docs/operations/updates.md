# Updates and rollback

Machines follow the `:latest` Image on `ghcr.io/open-source-space-foundation/proves-ci-os`. CI builds it from `main` and signs it ([ADR 0003](../adr/0003-verified-signed-updates.md)). Updates are idle-gated ([ADR 0006](../adr/0006-idle-gated-image-updates.md)).

## Schedule

A systemd timer runs `proves-update` **Mondays at 09:00 UTC** with up to an hour of random delay, shortly after the weekly image build.

`proves-update`:

1. Stages the new image with `bootc upgrade` (no reboot).
2. If nothing new is staged, exits.
3. Waits until no `Runner.Worker` process is running, checking every minute, for at most **72 hours**.
4. Stops the runner service (a running job gets up to five minutes to finish) and reboots.

After 72 hours of a continuously busy runner the update is **forced**, and a running job can be killed. A missed window costs a week at most, because the update is already staged.

## Update now

```sh
sudo proves-update
```

It follows the same path, including waiting for an idle runner. Check state with `sudo bootc status` and `systemctl list-timers proves-update.timer`.

## Roll back

Rollback is manual. After a bad update:

```sh
sudo bootc rollback
sudo systemctl reboot
```

If the machine does not come up with networking, use the console ([recovery](recovery.md)). Because machines follow `:latest`, the next weekly update will install the newer image again; ask the maintainers to fix or revert it first.

## Signing-key rotation

Images are signed with a cosign key pair. The public key is baked into the Image (`/etc/pki/containers/proves-ci-os.pub`, `cosign.pub` in this repository); the private key and its password are secrets in the `release` environment (`COSIGN_PRIVATE_KEY`, `COSIGN_PASSWORD`).

A machine only accepts an image signed by the key it already trusts, so rotation takes two releases:

1. Generate the new key pair and store it as new `release` secrets.
2. Publish an image, signed with the **old** key, that carries **both** public keys and a policy trusting either.
3. Wait until every machine has taken that update.
4. Sign with the new key and remove the old public key in a later image.

Never remove the old key before every machine has the transitional image: a machine that cannot verify the next image cannot update and needs a [reinstall](../setup/install.md). The multi-key policy has not been exercised yet; test it with `skopeo` and a spare machine first.

## Verify an image yourself

```sh
cosign verify --key cosign.pub ghcr.io/open-source-space-foundation/proves-ci-os:latest
```
