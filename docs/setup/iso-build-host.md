# ISO build host

The [ISO workflow](../../.github/workflows/iso.yml) runs on any Linux x86_64 runner in the `deathstar` group. The workflow carries no host-specific workarounds: prepare the host once with this page, and the job fails early with a pointer here if the host is not ready ([ADR 0008](../adr/0008-installer-iso.md)).

## Requirements

- Labels `Linux` and `X64`, in the `deathstar` runner group, and the group allows this repository (it is public, so the group needs *Allow public repositories*).
- `podman`, run as root by the job.
- Passwordless `sudo` for the runner's user.
- Outbound access to `ghcr.io`, `quay.io` and the Fedora mirrors.
- Nix is installed on first run and reused after that.

## Ubuntu: AppArmor and `bwrap`

`image-builder` runs `osbuild` in a privileged container, and `osbuild` runs its stages under `bwrap`. Ubuntu's `bwrap-userns-restrict` AppArmor profile attaches to every `/usr/bin/bwrap` on the host kernel, containers included, and its `unpriv_bwrap` child profile removes capabilities such as `sys_admin`. The build then fails with `mount: /run/osbuild/containers/storage: permission denied`, and the kernel log shows `apparmor="DENIED"` for `profile="unpriv_bwrap"`.

Put the profile in complain mode on the host. It still logs, nothing is blocked, and it survives reboots:

```sh
sudo ln -s ../bwrap-userns-restrict /etc/apparmor.d/force-complain/bwrap-userns-restrict
sudo apparmor_parser -r -C /etc/apparmor.d/bwrap-userns-restrict
sudo grep bwrap /sys/kernel/security/apparmor/profiles   # unpriv_bwrap should say (complain)
```

This relaxes a sandbox for every `bwrap` user on the machine, such as Flatpak apps, so use a host that is dedicated to CI. To undo it, remove the symlink and run `sudo apparmor_parser -r /etc/apparmor.d/bwrap-userns-restrict`.

Hosts without AppArmor, such as Fedora, need nothing here.

## Docker on the same host

Docker's `FORWARD DROP` firewall policy cuts off podman's default bridge network. The workflow already builds the installer image with `--network=host`, so no host change is needed.
