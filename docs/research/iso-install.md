# ISO build, size, hosting and Mac mini boot

Research for issue #14 (part of map #1, blocks #11). Date of research: 2026-10-09.

Status legend: **Verified** means read in the primary source named. **Unverified** means inferred, reported second-hand, or not testable without hardware or a build; each needs a spike before the install-flow decision relies on it.

Sources were read at these revisions: `osbuild/image-builder` and `osbuild/images` (shallow clones of `main`, `dd0e7d9497643db100682252955faed8d45f9d5d`, 2026-10-09), `rhinstaller/anaconda` `main` (`8cc8edd6a5821ff996874720933cd4dade9aed93`), `pykickstart/pykickstart` `main` (`1568c860e987b73959b10d023e60316649045ab3`).

## Summary

1. The premise of question 1 is out of date. `image-builder` documents **no** `anaconda-iso` type for bootc images; the docs say `anaconda-iso`/`iso` are legacy and unsupported there, and recommend `bootc-generic-iso`. A third type, `bootc-installer`, still exists and is the only one with a first-class way to bake a user kickstart.
2. An Anaconda kickstart may be partial. Omitted required items are prompted for interactively. That covers the `ops` password, hostname and target disk in principle, but the exact prompting behaviour with the `bootc` command is **unverified** until tried in a VM.
3. Expected ISO size is roughly 2 to 3 GiB (estimate, **unverified**); plan on it not fitting a 2 GiB GitHub Release asset. A ghcr OCI artifact pushed with `oras` is the best-supported alternative from the docs read.
4. Checksum plus a cosign keyless signature (`sign-blob --bundle`) is a sound fit and is documented. The signature must be verified by the installing user, so it protects the download, not the boot.
5. The "2013 Mac mini (Macmini6,x)" is the **Late 2012** Mac mini. Apple lists no Late 2013 model. The wired NIC is a Broadcom BCM57766, supported by the in-tree `tg3` driver. Apple-firmware bootloader behaviour is **unverified**.

## 1. Kickstart in a bootc ISO built by image-builder

### Image types available

From `doc/20-advanced/20-bootc/10-isos.md` and `50-migration.md` in `osbuild/image-builder`:

- `bootc-generic-iso` is the documented ISO type. Verified.
- "There was an alternative image type called `anaconda-iso` or `iso` in `bootc-image-builder` but this image type is not available in `image-builder`." (`10-isos.md`). Verified.
- "The `bootc-installer` image type is still available but its use is not recommended." (`50-migration.md`). Verified.
- Counterpoint: `osbuild/images` `data/distrodefs/bootc-generic/imagetypes.yaml` still defines `anaconda-iso` and `iso` (image function `bootc_legacy_iso`), commented "we want to get rid of it (here for compat with bib)". So the type may still resolve in the CLI today, but is slated for removal. Verified in source; whether the CLI exposes it is **unverified**.

### What each type does with a kickstart

`bootc-generic-iso` (recommended):

- Anaconda is not part of `image-builder`'s model. You build your own installer container (the doc's example `Containerfile` installs `anaconda`, `anaconda-dracut`, etc. on `fedora-bootc`) and `image-builder` explodes it into a squashfs plus GRUB config.
- Behaviour is set by files inside that container: `/usr/lib/image-builder/bootc/iso.yaml` (label, GRUB entries and kernel args) and, per the doc example, `/usr/share/anaconda/interactive-defaults.ks` containing a `bootc --source-imgref ... --target-imgref ...` line. Verified (`10-isos.md`, `05-sources-of-configuration.md`).
- The type ignores the blueprint: `manifestForGenericISO` in `osbuild/images` `pkg/distro/defs/bootc_imagetype.go` takes no blueprint argument. So no `customizations.user` or `customizations.installer` here. Verified in source.
- Caveat: Anaconda's own `data/interactive-defaults.ks` header says "This file is deprecated and will be removed in in the future. Use the Anaconda configuration files instead." and "This is not loaded if a kickstart file is provided on the command line." Verified. Risk: the documented mechanism for the payload line is on a deprecation path.
- Alternative to `interactive-defaults.ks`: put `inst.ks=...` in the `iso.yaml` GRUB `linux:` line. `bootc-generic-iso` places no kickstart file on the ISO filesystem itself, so a path would have to come from inside the stage2 squashfs (for example `inst.ks=file:///...`, if Anaconda supports it at that point). **Unverified.**
- Offline install: pass `--bootc-installer-payload-ref`; the payload is copied into `/var/lib/containers/storage` inside the squashfs and the kickstart must use `bootc --source-imgref containers-storage:<ref> ...`. Verified (`10-isos.md`).
- Doc warning: a bootc system installed through Anaconda fails to start `systemd-remount-fs.service` (links to a Fedora atomic-desktops tracker issue and RH bugzilla 2332319). Verified that the warning exists; impact on our image **unverified**.

`bootc-installer` (still present, discouraged):

- Takes a bake-in kickstart via blueprint `customizations.installer.kickstart.contents`. Source: `pkg/customizations/kickstart/kickstart.go` (`options.UserFile = &File{Contents: instCust.Kickstart.Contents}`). Verified.
- When a user file is given, `image-builder` generates a kickstart that handles the payload install and `IncludeRaw`s the user's file for everything else (`pkg/manifest/anaconda_installer_iso_tree.go`, `bootcInstallerKickstartStages`). Verified.
- A user file is incompatible with `unattended`, `sudo_nopasswd`, and blueprint users/groups (`Options.Validate`). So accounts must live in the kickstart text. Verified.
- Without a user file it builds a fully unattended kickstart: root locked, `clearpart --all`, UTC, `en_US.UTF-8`, `us` keyboard, `autopart`, `reboot --eject`. Verified. That is the opposite of what we want (prompting), so we would supply our own file.
- The payload is generated with `ostreecontainer`, not the newer `bootc` kickstart command, plus a `%post` `bootc switch` workaround. Verified in source.
- Both inputs are mandatory: `--bootc-ref` (an Anaconda installer container we build) and `--bootc-installer-payload-ref` (`doc/01-usage.md`). Verified.

### What a kickstart can set

Anaconda/pykickstart docs and source (all Verified unless stated):

- Incomplete files prompt: "Omitting any required item will result in the installation program prompting the user for an answer to the related item, just as the user would be prompted during a typical installation." (pykickstart kickstart docs).
- `rootpw`: `--lock`, `--iscrypted`, `--plaintext`, `--allow-ssh` (`pykickstart/commands/rootpw.py`).
- `user`: `--name` (required), `--password`, `--iscrypted`, `--groups`, `--gecos`, `--lock` (`user.py`).
- `network --hostname`; DHCP via `network --bootproto=dhcp --device=link --activate --onboot=on` (the form `image-builder` itself emits).
- `timezone --utc <zone>`; `clearpart --all [--initlabel] [--drives=...]`; `autopart`; `reboot`.
- `bootc` (Fedora 43+ in pykickstart, `--bootloader` and `--composefs` added at F45): `--source-imgref` required, `--target-imgref`, `--stateroot`. Conflicts with `ostreesetup` and `ostreecontainer`. Anaconda F44 release notes: limitations include "lack of support for partitioning setups spanning multiple disks, arbitrary mount points, or installation from authenticated registries."
- Anaconda runs `bootc install to-filesystem --bootloader=grub --karg=... --stateroot=... --source-imgref=... --target-imgref=...` (`pyanaconda/modules/payloads/payload/rpm_ostree/installation.py`), so bootc, not Anaconda, installs the bootloader.

### Mapping to the three prompts we want

| Want | Kickstart approach | Status |
| --- | --- | --- |
| Prompt for `ops` password | Omit `rootpw`/`user`, or define `user --name=ops` without `--password` | Prompt behaviour for a pre-named user without a password is **unverified**. Omitting is the documented path. |
| Prompt for hostname | Omit `network --hostname` | Documented generically; **unverified** with bootc. |
| Prompt for target disk | Omit `ignoredisk`/`autopart`; whole-disk wipe via `clearpart --all` | Whether `clearpart --all` without `autopart` still shows the destination spoke, and whether the bootc payload demands a partitioning command, is **unverified**. |
| Wired DHCP, UTC | `network --bootproto=dhcp ...`, `timezone --utc UTC` | Commands verified; behaviour on the installed system **unverified**. Anaconda F44 stops creating default profiles for unconfigured NICs, so the kickstart must configure the NIC explicitly (release notes, Verified). |

A virtual-machine spike of the real ISO with the real kickstart is the only way to close the unverified rows.

## 2. ISO size and hosting

Measured (Verified, quay.io registry API, linux/amd64, summing layer sizes in the manifest):

- `quay.io/fedora/fedora-bootc:44`: 65 layers, about 977 MiB compressed.
- `quay.io/fedora/fedora-bootc:43`: about 953 MiB compressed.

Estimate (**Unverified**, no ISO was built): a `bootc-generic-iso` holds a zstd squashfs of the installer environment (fedora-bootc plus Anaconda and dependencies) and, if `--bootc-installer-payload-ref` is used, a second copy of the payload image in containers-storage. At about 1 GiB each before our toolchain layers, expect 2 to 3 GiB. Tool-chain-sized layers push it further over. Build once and measure.

Hosting:

- GitHub Release assets: "Each file included in a release must be under 2 GiB", up to 1000 assets per release, no limit on total size or bandwidth (GitHub Docs, About releases). Verified. An ISO over 2 GiB cannot be a single asset.
- ghcr: "10 GB size limit for each layer", "10 minute timeout limit for uploads", public images pullable anonymously; OCI specs supported (GitHub Docs, Working with the Container registry). Verified. Pricing for public packages was not stated on the page read; **unverified**.
- `oras push <ref> --artifact-type <type> file:media-type` stores a single file as an OCI artifact and `oras pull` fetches it (ORAS docs). Verified. The docs read state no per-file size limit.
- Option without hosting change: split into parts under 2 GiB. Works but hurts the install story; no source needed.
- Object storage was not researched against a primary source and is not recommended without a stated need.

Suggested direction (for #11 to decide): a ghcr OCI artifact via `oras`, with the release page linking to the `oras pull` command; fall back to Release assets only if a measured ISO is under 2 GiB.

## 3. Mac mini quirks

Identification:

- Apple's identify-your-model page lists "Mac mini (Late 2012): Macmini6,1; Macmini6,2" and "Mac mini (Late 2014): Macmini7,1". There is no Late 2013 model. Treat the target as Late 2012 (Macmini6,x). Verified (support.apple.com/en-us/102852). Confirm the actual unit with `Apple menu > About This Mac` or `Macmini6,x` from System Information.
- Specs: Core i5 dual-core or Core i7 quad-core, Intel HD Graphics 4000, up to 16 GB DDR3, Gigabit Ethernet (Apple tech specs SP659). Verified. x86_64 confirmed.

Boot picker:

- Apple: holding Option at startup opens Startup Manager "which allows you to choose other available startup disks or volumes" (support.apple.com/en-us/102603). Verified.
- Fedora wiki FedoraOnMactel: hold the left alt/option key, a boot-device list appears, select the entry with the Fedora logo ("EFI Boot"). Verified that the page says this, but the page is a Fedora 17-era technology preview (2012). Treat as dated.

EFI vs hybrid:

- Fedora wiki states EFI boot support on 64-bit-firmware Macs (post-mid-2007). Verified, dated.
- `bootc-generic-iso` builds an ISO with an EFI GRUB (requires `shimx64.efi`, `gcdx64.efi` etc. in the container) and, on x86_64, BIOS GRUB modules (`/usr/lib/grub/i386-pc`) (`10-isos.md`). Verified from docs. Whether the resulting ISO is a hybrid image that Apple's firmware lists as bootable, and whether shim is acceptable to this firmware, is **unverified**. Late-2012 Macs predate Apple's T2 and Secure Boot; that they enforce no Secure Boot is from general knowledge, **unverified** here.
- Practical: write the ISO to USB as a raw image (for example `dd`), then pick the "EFI Boot" entry. **Unverified** for this ISO.

Wired NIC in the installer:

- Macmini6,2 and some 2012 iMacs use Broadcom BCM57766, PCI ID 14e4:1686; an iPXE mailing-list post (April 2013) reports it unrecognised by iPXE then. Verified that the post says this (secondary: a mailing-list report).
- The mainline Linux `drivers/net/ethernet/broadcom/tg3.c` carries `{PCI_DEVICE(PCI_VENDOR_ID_BROADCOM, TG3PCI_DEVICE_TIGON3_57766)}` in `tg3_pci_tbl` (0x1686). Verified (current `master`). The installer kernel comes from fedora-bootc, so `tg3` support is expected, but whether the `tg3` module is in the Anaconda initramfs/stage2 built from the installer container is **unverified**. `--no-hostonly` dracut build in the doc's example supports inclusion.
- Which Macmini6 variant has which NIC beyond what the sources above state is **unverified**.

Bootloader on Apple firmware:

- Anaconda hands the bootloader to `bootc install to-filesystem --bootloader=grub` for bootc payloads (source above). Bootc's options include `--bootloader` (grub, grub-cc, systemd, none) and `--generic-image` (bootc man page, Verified). Whether bootc/bootupd writes the removable fallback path `\EFI\BOOT\BOOTX64.EFI` that Apple firmware reliably boots, versus relying on NVRAM entries Apple firmware may ignore, was not established from a primary source. **Unverified.** Highest-risk item; test on real hardware (or confirm bootupd's behaviour from its source).
- Dual-boot with macOS was not in scope.

## 4. Signature and trust

- Cosign documents keyless blob signing: `cosign sign-blob <file> --bundle bundle.sigstore.json`; the bundle holds the signature, certificate and transparency-log proof (Sigstore docs, signing blobs). Verified.
- Verification: `cosign verify-blob <file> --bundle artifact.sigstore.json --certificate-identity=<id> --certificate-oidc-issuer=<issuer>` (Sigstore docs, verifying). Verified. In GitHub Actions the identity is the workflow identity and the issuer is GitHub's OIDC issuer; the issuer URL `https://token.actions.githubusercontent.com` was not on the pages read, so confirm before documenting it.
- For an ISO pushed as an OCI artifact, Sigstore docs say to upload the bundle with `oras`, and `cosign sign` works on registry references. Verified in outline; exact flow for arbitrary artifacts **unverified**.
- Conclusion: yes, a checksum (SHA-256 file) plus a cosign keyless bundle covering the ISO is a good fit. Limits: it authenticates the file for someone who runs `cosign verify-blob` before writing the USB; it does not make Apple firmware verify anything at boot. The bootc payload itself should also be signed and verified (out of scope here).

## Open items to spike

1. Build a `bootc-generic-iso` with the real payload; record the ISO size.
2. Boot it in a VM with the partial kickstart; confirm password, hostname and disk prompts, and that `clearpart --all` with no `autopart` works with the `bootc` command.
3. Decide how the kickstart reaches Anaconda (`interactive-defaults.ks`, deprecated, versus `inst.ks=` versus `bootc-installer` blueprint kickstart).
4. On the real Mac mini: Option-key boot of the USB, `tg3` in the installer, and whether the installed system boots (fallback EFI path).
5. Confirm ghcr public-package cost and the GitHub Actions cosign identity values.

## Sources

- https://github.com/osbuild/image-builder: `doc/20-advanced/20-bootc/10-isos.md`, `05-sources-of-configuration.md`, `50-migration.md`, `doc/01-usage.md`
- https://github.com/osbuild/images (now `osbuild/image-builder` `pkg/`): `data/distrodefs/bootc-generic/imagetypes.yaml`, `pkg/distro/defs/bootc_imagetype.go`, `pkg/customizations/kickstart/kickstart.go`, `pkg/manifest/anaconda_installer_iso_tree.go`
- https://github.com/rhinstaller/anaconda: `data/interactive-defaults.ks`, `docs/release-notes/release-notes.rst`, `docs/user-guide/kickstart.rst`, `pyanaconda/modules/payloads/payload/rpm_ostree/installation.py`
- https://github.com/pykickstart/pykickstart: `pykickstart/commands/{bootc,rootpw,user,timezone,clearpart}.py`; https://pykickstart.readthedocs.io/en/latest/kickstart-docs.html
- https://bootc.dev/bootc/man/bootc-install-to-filesystem.8.html
- https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases
- https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry
- https://oras.land/docs/how_to_guides/pushing_and_pulling/
- https://docs.sigstore.dev/cosign/signing/signing_with_blobs/ , https://docs.sigstore.dev/cosign/verifying/verify/ , https://docs.sigstore.dev/cosign/signing/signing_with_containers/
- https://support.apple.com/en-us/102852 , https://support.apple.com/kb/SP659 , https://support.apple.com/en-us/102603
- https://fedoraproject.org/wiki/FedoraOnMactel (dated)
- https://github.com/torvalds/linux/blob/master/drivers/net/ethernet/broadcom/tg3.c
- https://lists.ipxe.org/pipermail/ipxe-devel/2013-April/002444.html (secondary, mailing list)
- https://quay.io/v2/fedora/fedora-bootc/manifests/44 (registry API, sizes)
