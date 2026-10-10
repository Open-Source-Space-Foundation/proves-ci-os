# fedora-bootc:44 sshd defaults

Question (issue #13, parent map #1): does `quay.io/fedora/fedora-bootc:44` ship
and enable sshd, allow password ssh, and expose port 22? Facts only; the
decision belongs to the project owner.

## Method

`docker run --rm quay.io/fedora/fedora-bootc:44 ...` on an Apple Silicon host,
so the inspected image is the **aarch64** variant (digest
`sha256:c24cce42b92975040c0a725c7eef9d6c3bd6f3de4165d947dcd4dff4ab57c3ab`,
fedora-release-common 44-18, systemd 259.9, openssh 10.2p1-14.fc44). Presets,
package set and config files are arch-independent in practice, but the x86_64
image itself was not inspected.

## Findings

| Question | Answer | Evidence in image |
| --- | --- | --- |
| `openssh-server` installed? | Yes | `rpm -q openssh-server` -> `openssh-server-10.2p1-14.fc44` |
| `sshd.service` enabled? | **Yes** | `/usr/lib/systemd/system-preset/90-default.preset:24: enable sshd.service`; symlink `/etc/systemd/system/multi-user.target.wants/sshd.service` present in the image; `systemctl is-enabled sshd.service` -> `enabled` |
| `sshd.socket` enabled? | No | `systemctl is-enabled sshd.socket` -> `disabled` (the unit `Conflicts=sshd.service`) |
| Listens where? | All interfaces, port 22 | `sshd -T`: `port 22`, `listenaddress 0.0.0.0:22`, `listenaddress [::]:22` |
| `PasswordAuthentication`? | **On** (OpenSSH built-in default) | `/etc/ssh/sshd_config` line 65 is the commented `#PasswordAuthentication yes`; no drop-in sets it; `sshd -T` -> `passwordauthentication yes` |
| Other auth settings | `PermitRootLogin prohibit-password`, `KbdInteractiveAuthentication no` (from `/etc/ssh/sshd_config.d/50-redhat.conf`), `UsePAM yes`, `PermitEmptyPasswords no` | `sshd -T` |
| `firewalld` present? | **No** | `rpm -q firewalld` -> not installed; no `/etc/firewalld`. `/usr/lib/firewalld/zones/nm-shared.xml` (from another package) mentions `ssh` but is inert without firewalld. Note `90-default.preset` also says `enable firewalld.service`, which has no effect because the package is absent |
| Any other host firewall active? | No | `nftables` is installed but `systemctl is-enabled nftables` -> `disabled`; no iptables. The sample `/etc/nftables/main.nft` (not loaded) would allow `ssh` and 9090 anyway |
| Root password | Locked in the image (`root:*` in `/etc/shadow`) | Password ssh only works for an account that has a usable password |

Net effect: as shipped, a deployment runs sshd on 0.0.0.0:22 and [::]:22 with
no host firewall in front, and password authentication is accepted for any
account that has a usable password. A password set at the console for a local
account would therefore also work over ssh from the LAN.

## Sources

- Fedora default presets are owned by the `fedora-release` package
  (`90-default.preset`): https://src.fedoraproject.org/rpms/fedora-release ;
  the file header points to https://fedorahosted.org/fesco/ticket/1472 and the
  policy at https://docs.fedoraproject.org/en-US/packaging-guidelines/DefaultServices/
- The bootc-specific presets in the image (`85-bootc.preset`,
  `86-bootc-standard.preset`, `85-bootc-bootloader-update.preset`) do not
  mention ssh (they cover `bootc-fetch-apply-updates.timer`, `dnf-makecache.timer`,
  `systemd-oomd`, bootloader update). Fedora bootc docs:
  https://docs.fedoraproject.org/en-US/bootc/
- Preset semantics (first matching line across files in lexical order wins;
  `99-default-disable.preset` ends with `disable *`):
  https://www.freedesktop.org/software/systemd/man/latest/systemd.preset.html
- `PasswordAuthentication` default `yes`, `PermitRootLogin` default
  `prohibit-password`, and "first obtained value is used" for `Include`d files:
  https://man.openbsd.org/sshd_config
- Fedora's `sshd_config` and `50-redhat.conf` come from the `openssh` package:
  https://src.fedoraproject.org/rpms/openssh
- Tailscale SSH is served by `tailscaled`, not OpenSSH:
  https://tailscale.com/kb/1193/tailscale-ssh

## Minimum change if sshd must not be reachable on the LAN

Any one of these in the Containerfile removes the LAN password path; the first
is the smallest:

1. `RUN systemctl disable sshd.service` (removes the wants symlink; no listener
   at boot). `systemctl mask sshd.service` additionally prevents manual starts.
2. Defense in depth, if sshd stays enabled: ship
   `/etc/ssh/sshd_config.d/10-no-passwords.conf` containing
   `PasswordAuthentication no`. The `10-` prefix sorts before `50-redhat.conf`
   and first value wins. This keeps a console password from becoming an ssh
   password but leaves port 22 listening.
3. Remove the package (`dnf remove openssh-server`): more than the minimum.

Not verified: behavior of a booted (rather than container) deployment, and the
x86_64 build specifically.
