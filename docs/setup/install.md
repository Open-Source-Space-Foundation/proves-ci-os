# Install

Install the Image on a CI machine from a USB stick. The installer wipes the whole internal disk.

> **Warning:** everything on the target disk is erased, with no confirmation beyond choosing the disk. Do not install on a machine whose data you need.

The installer ISO is described in [ADR 0008](../adr/0008-installer-iso.md), which is still *proposed*: the build, size and boot behavior are being proven in a spike. Where this page says what the installer does, that is the intended behavior until the ADR is accepted.

## Requirements

- x86_64 machine with UEFI firmware (no legacy BIOS).
- Wired Ethernet with DHCP. Wi-Fi is not supported and no Wi-Fi packages are installed.
- A USB stick for the ISO.
- A Tailscale account (see [Tailscale SSH](tailscale-ssh.md)).

## Verify the ISO

Download the ISO, its checksum file and its cosign signature from the release page. Check both:

```sh
sha256sum -c proves-ci-os.iso.sha256
cosign verify-blob --key cosign.pub --signature proves-ci-os.iso.sig proves-ci-os.iso
```

`cosign.pub` is in the root of this repository. Do not skip this: the machine will hold CI secrets.

## Boot and install

1. Write the ISO to the USB stick (`dd`, or any tool that writes raw images).
2. Boot the machine from the stick (usually F12 or Esc at power-on).
3. Answer the installer prompts:
   - **Console password** for the `ops` account. It is hashed and never published. It is the only way in if the tailnet is unreachable, so keep it in a password manager ([recovery](../operations/recovery.md)).
   - **Hostname**. It becomes the default Runner name, so pick something that identifies the machine in the runner list.
   - **Target disk**. This disk is wiped.
4. Wait for the install to finish, remove the stick and reboot.

The kickstart fixes everything else: the accounts, wired DHCP, the Image reference, locale and UTC. No network configuration is needed.

## Next

[Join the machine to your tailnet](tailscale-ssh.md), then [register the runner](runner.md).
