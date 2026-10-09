# Can Fedora bootc boot and install on Raspberry Pi 4 and 5?

Resolves issue #2. Researched 2026-10-08. Primary sources only (Fedora, bootc / osbuild / bootupd upstream repos, Raspberry Pi docs, Fedora ARM maintainer posts). Claims marked **[inference]** are mine, not stated by a source.

## Verdict

| Model | Verdict | One line |
|---|---|---|
| Raspberry Pi 4 | **viable-with-caveats** | Fedora supports the hardware (incl. Wi-Fi) and a multi-arch `fedora-bootc` aarch64 image exists, but no Fedora-documented path builds a bootable bootc disk for it; firmware/U-Boot staging is manual. |
| Raspberry Pi 5 | **blocked** (for now) | Not officially supported by Fedora; needs a non-stock kernel, boots from microSD only (no USB/NVMe boot), documented by the maintainer as in-progress. |

"Blocked" for Pi 5 means blocked for a CI-runner OS today (SD-only, non-stock kernel), not impossible; it is moving quickly (see Pi 5 below). Re-check before Fedora 45.

## What is established

### bootc / bootc-image-builder side

- The `quay.io/fedora/fedora-bootc` image is multi-arch. The `44` manifest list contains `arm64`, `amd64`, `ppc64le`, `s390x`. (Quay registry API, checked 2026-10-08; tags 42-45, latest, rawhide exist.) <https://quay.io/repository/fedora/fedora-bootc>
- bootc-image-builder has been merged into `osbuild/image-builder`; the old repo is now only a compatibility container. <https://github.com/osbuild/image-builder/blob/main/bootc-image-builder/README.md>
- The bootc image-type definitions list `raw`, `qcow2`, `ami`, `vmdk`, `vhd`, `gce`, `ova`, `oci`, ISO installers and `pxe-tar-xz`; aarch64 is a supported arch with an EFI system partition (vfat), /boot and root. There is **no Raspberry Pi-specific definition** (no U-Boot, no Pi firmware files). <https://github.com/osbuild/image-builder/blob/main/data/distrodefs/bootc-generic/imagetypes.yaml>
- bootupd (bootloader updater used by bootc) supports "updating GRUB and shim for UEFI firmware on x86_64, aarch64, and riscv64" and GRUB for BIOS; U-Boot / Pi firmware are not mentioned. <https://github.com/coreos/bootupd/blob/main/README.md>
- Cross-arch builds (building aarch64 on x86_64) go through QEMU user-static, are labelled experimental and run roughly "8x-10x slower" than native. Cross-build tests exist for x86_64<->aarch64 (AMI image type). <https://github.com/osbuild/image-builder-cli/blob/main/README.md>, <https://github.com/osbuild/image-builder/blob/main/bootc-image-builder/test/test_build_cross.py>
- The Fedora bootc docs list three install paths: stock Anaconda ISO/PXE, a bootc-image-builder ISO, or `bootc install` from the container. None mentions Raspberry Pi. <https://gitlab.com/fedora/bootc/docs/-/raw/main/modules/ROOT/pages/bare-metal.adoc>
- Closest existing precedent: AlmaLinux's experimental `bootc-images-rpi` (Pi 5 incl. 2/8/16 GB, Pi 4, Pi 3, Zero 2 W). It states: "because of the firmware issue, bootc-image-builder alone isn't enough to build a raw boot image", firmware updates "may need some manual steps or reimaging the pi", and writes are not minimised so cheap flash wears out. Images are pulled from Quay, flashed to M.2 (Pi 5), SD or USB. This is AlmaLinux, not Fedora, but it is the same stack and shows the gap. <https://github.com/AlmaLinux/bootc-images-rpi>

**[inference]** A Fedora bootc disk for Pi would need a custom build step that adds the Pi boot files (U-Boot or EDK2, `bcm283x` firmware/overlays) to the ESP after bootc-image-builder, and a plan for refreshing them since bootupd will not.

### Raspberry Pi 4

- Fedora supports "the Raspberry Pi 4-series (4B, 400, CM4, CM4s)"; Pi 4 is "fully supported in Fedora 37 and later" and a Fedora IoT reference platform. Wi-Fi "on the Raspberry Pi 4 model B works"; on Pi 400 and Zero 2W it is "currently unsupported". <https://fedoraproject.org/wiki/Architectures/ARM/Raspberry_Pi>
- Closest ostree-style precedent is Fedora CoreOS on Pi 4. Requirements: update the Pi's SPI EEPROM first (old firmware "had problems reading a FAT16 EFI partition"); then either U-Boot (files from the `uboot-images-armv8`, `bcm2711-firmware`, `bcm283x-firmware`, `bcm283x-overlays` RPMs copied onto the ESP of a disk prepared on another machine) or EDK2/UEFI (SystemReady ES layer, separate or combined disk). With EDK2, 4 GB+ models may need the 3 GB RAM limit disabled in firmware settings. <https://github.com/coreos/fedora-coreos-docs/blob/main/modules/ROOT/pages/provisioning-raspberry-pi4.adoc>
- Boot media per Raspberry Pi docs: SD, USB mass storage, network (and HTTP) boot; bootloader lives in SPI EEPROM, updated with `rpi-eeprom-update`. Pi 4 has no native PCIe/NVMe boot (NVMe-over-USB only, **[inference]**). <https://www.raspberrypi.com/documentation/computers/raspberry-pi.html>
- Install media: SD or USB. For CI runners, USB SSD is the sensible choice (SD wear; see AlmaLinux note above).

### Raspberry Pi 5

- The Fedora ARM wiki lists only the Pi 3 and Pi 4 series; Pi 5 is not mentioned there. <https://fedoraproject.org/wiki/Architectures/ARM/Raspberry_Pi>
- Fedora ARM maintainer Peter Robinson (nullr0ute) announced the first minimal Fedora 43 Pi 5 image on 2025-09-08 using a custom kernel, built in "personal spare time". Then: rev c0 SoC only (4/8 GB); no rev d0 (2/16 GB, Pi 500, CM5); no early-boot display or GPU acceleration; **no boot from USB or NVMe**; microSD, wired and wireless networking work. <https://discussion.fedoraproject.org/t/initial-minimal-fedora-image-for-raspberry-pi-5/163809>
- Update, 2026-03-14 (Fedora 44): all Pi 5B variants (rev C and D, 1-16 GB) work, with serial, microSD, HDMI + accelerated graphics, ethernet, wireless, USB ports (not for the OS disk). Still unsupported: Pi 500, CM5, **NVMe**, thermal, audio. Requires adding `cma=256M@0M-1024M` to the kernel command line and persisting it in `/etc/kernel/cmdline`. Devices **boot only from microSD**, via standard UEFI/U-Boot. The Pi 5 kernel is "the only thing that's not vanilla Fedora". <https://nullr0ute.com/2026/03/fedora-44-on-the-raspberry-pi-5/>
- Community thread: upstream kernel support was the long-standing blocker (SoC pinctrl, mmc, gpio); users run COPR/custom kernels (e.g. `6.12.x ... rpi5 ... fc42`). Not an official source; used only as context. <https://discussion.fedoraproject.org/t/aarch64-support-for-raspberry-pi-5/134857>
- Hardware itself can boot NVMe (single-lane PCIe connector, M.2 HAT+) and supports A/B bootloader updates, but Fedora does not boot from it yet. <https://www.raspberrypi.com/documentation/computers/raspberry-pi.html>

**Why this is a blocker for bootc specifically [inference]:** a bootc image carries its own kernel in the container. The stock `fedora-bootc` kernel is the vanilla Fedora kernel, which per the maintainer does not suffice on Pi 5; we would have to rebuild the image with the Pi 5 kernel and track it ourselves.

## Per-topic answers to the issue

| Topic | Pi 4 | Pi 5 |
|---|---|---|
| Boot firmware | SPI EEPROM (update first) + U-Boot or EDK2 on the ESP | SPI EEPROM + UEFI/U-Boot per Fedora Pi 5 images |
| Kernel | Stock Fedora kernel | Non-stock Pi 5 kernel; `cma=256M@0M-1024M` karg |
| Disk image production | `fedora-bootc` arm64 + bootc-image-builder `raw`, then add Pi boot files manually (no Pi support in BIB) | Same, plus custom kernel |
| Install media | SD, USB (no NVMe boot) | microSD only today; USB/NVMe boot not working in Fedora |
| Wi-Fi | Works on 4B (not Pi 400) | Reported working (Fedora 43/44 images) |
| Known blockers | No documented bootc-on-Pi path; firmware not updated by bootupd | Not officially supported; SD-only boot; custom kernel; NVMe/thermal/audio missing |

## Not verified / gaps

- No primary source shows `fedora-bootc` actually booting on a Pi 4 or Pi 5. The Fedora docs site (docs.fedoraproject.org) blocked automated fetch, so Fedora IoT and Fedora CoreOS docs were read from their source repositories instead.
- Whether Pi 4 Wi-Fi firmware survives an immutable root (blobs come from `linux-firmware` in the image) is untested here.
- A spike is the next step: build `fedora-bootc:44` raw for aarch64, add U-Boot/Pi firmware to the ESP, boot a Pi 4 from USB. This unblocks the dependent issues (#10, #11).
