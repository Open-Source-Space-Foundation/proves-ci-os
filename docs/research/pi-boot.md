# How do Pi 4 and Pi 5 boot a fedora-bootc disk and an installer ISO?

Resolves issue #36 (part of map #35). Researched 2026-10-09. Builds on the earlier `research/raspberry-pi-bootc` note (issue #2); claims carried over from it are marked *(carried over)*. Claims marked **[inference]** are mine, not stated by a source. Claims marked **[unverified]** need a spike or hardware.

## Summary

| Question | Pi 4 | Pi 5 |
|---|---|---|
| Boot chain | SPI EEPROM, then either U-Boot (EFI) or EDK2 (UEFI) from the ESP | SPI EEPROM, then U-Boot (EFI), the generic `rpi_arm64` build |
| Kernel | Stock Fedora kernel | Non-stock: a Pi 5 kernel from the Fedora ARM maintainer ("the only thing that's not vanilla Fedora") |
| Extra kernel arg | none known | `cma=256M@0M-1024M` |
| Boot media | SD, USB | **microSD only**; USB and NVMe boot not supported by Fedora |
| bootupd manages boot files | No | No |
| Known broken | none that matter here | Bluetooth (U-Boot hands the kernel a stripped DT), thermal/fan, NVMe, audio, SD flakiness, random boot hangs reported |
| Verdict | Viable with a custom build step | Possible only from microSD with a non-stock kernel; best-effort |

## Firmware, bootloader and kernel

### Common to both
- Both boards load their first-stage bootloader from SPI EEPROM, not from the boot media. It is read and updated with `vcgencmd bootloader_version` and `rpi-eeprom-update`. Pi 5 additionally has A/B EEPROM updates. <https://www.raspberrypi.com/documentation/computers/raspberry-pi.html>
- The GPU firmware reads `config.txt` (`kernel=`, `arm_64bit`, `cmdline`) before Linux starts. <https://www.raspberrypi.com/documentation/computers/config_txt.html>
- bootupd "supports updating GRUB and shim for UEFI firmware on x86_64, aarch64, and riscv64, and GRUB for BIOS firmware on x86_64 and ppc64le." Neither U-Boot nor Raspberry Pi is mentioned. <https://github.com/coreos/bootupd/blob/main/README.md>
  - Consequence **[inference]**: bootupd can manage GRUB/shim on an aarch64 ESP, but it will not touch U-Boot, `start4.elf`, `fixup`, overlays or EDK2 files. Those need another update path (see Open items).

### Pi 4
- Fedora: Pi 4 (4B, 400, CM4, CM4s) is "fully supported in Fedora 37 and later". The wiki says Fedora 31+ boots in ACPI mode on Pi 4 with the Pi Firmware Task Force UEFI. <https://fedoraproject.org/wiki/Architectures/ARM/Raspberry_Pi>
- Closest ostree-style precedent, Fedora CoreOS on Pi 4, offers two paths. Both first need a current EEPROM, because old firmware "had problems reading a FAT16 EFI partition". <https://github.com/coreos/fedora-coreos-docs/blob/main/modules/ROOT/pages/provisioning-raspberry-pi4.adoc>
  - **U-Boot**: disk prepared on another machine; files from RPMs `uboot-images-armv8`, `bcm2711-firmware`, `bcm283x-firmware`, `bcm283x-overlays`; `u-boot.bin` is placed on the ESP as `rpi-u-boot.bin`.
  - **EDK2**: firmware from <https://github.com/pftf/RPi4/releases>, on a separate always-attached SD/USB disk or copied into the ESP of the install disk. 4 GB+ boards may need the default 3 GB RAM limit disabled in the firmware menu; ACPI is the default, DT mode is optional.
- EDK2 for Pi 4 boots from SD, USB and an ESP (FAT16/32 with `0xef` type), needs a recent EEPROM, lacks many drivers, and needs Linux 5.7+ for Ethernet. The firmware states a Broadcom DMA bug is why the 3 GB limit exists. <https://github.com/pftf/RPi4>
- **Pi 4 verdict [inference]**: both paths work with the stock aarch64 Fedora kernel, and wired Ethernet is fine. The stock `fedora-bootc:44` kernel is therefore adequate; the missing piece is putting Pi boot files on the ESP.

### Pi 5
- Fedora 44 images exist for all Pi 5B variants (rev C and D, 1 to 16 GB): serial, microSD, HDMI with acceleration, Ethernet, Wi-Fi and USB ports work. Not working: Pi 500, CM5, NVMe, thermal, audio. <https://nullr0ute.com/2026/03/fedora-44-on-the-raspberry-pi-5/>
- The microSD slot is "the only supported OS disk ATM"; USB ports are "NOT for OS disks". Same source.
- `cma=256M@0M-1024M` must be added to the kernel command line (and `/etc/kernel/cmdline`); "without that accelerated graphics and some other things just won't work". Same source. For a headless CI machine this may not be needed **[unverified]**.
- Announcement thread: Peter Robinson posted the Fedora 44 Pi 5 images on 2026-03-14 asking for feedback before freeze. Reported in the thread: kernel `6.19.9-300.pr1.fc44.aarch64` (the `pr1` suffix marks it as non-stock), U-Boot with EFI, KASLR on, boot hangs at random points, SD card timeouts with some cards, missing fan driver, and a maintainer preference for mainline kernel work over out-of-tree patches. <https://discussion.fedoraproject.org/t/raspberry-pi-5-images-for-fedora-44/183204>
- Post 24 (2026-04-30, user "gooby"): Bluetooth is dead because the Pi 5 image ships `/boot/efi/rpi-u-boot.bin`, identical to the generic `rpi_arm64` build in `uboot-images-armv8` (2026.04-2.fc44). That build uses U-Boot's embedded DT instead of the on-disk DTB, so the Bluetooth node lacks `status = "okay"`, `max-speed` and `shutdown-gpios`. Proposed fix: a Pi 5 specific U-Boot or a patched generic target. <https://discussion.fedoraproject.org/t/raspberry-pi-5-images-for-fedora-44/183204/24> (user report, root cause not confirmed by a maintainer). Bluetooth is out of scope for the map, so this is acceptable.
- Upstream background: the Pi 5 needed out-of-tree patches (SoC pinctrl, mmc, gpio, RP1) because upstream Linux support was incomplete; COPR kernels carried them. <https://lists.fedoraproject.org/archives/list/arm@lists.fedoraproject.org/thread/LDZXN3WZ4O2YD2OCPWGWSRTWBOVVXGVZ/> (found via search; page blocked to fetch, so the snippet only). *(carried over)*
- **UEFI/EDK2 for Pi 5**: `https://github.com/pftf/RPi5` returned 404; I found no first-party EDK2 for Pi 5. U-Boot EFI is the only boot path Fedora ships for it. **[unverified]** whether any other UEFI exists.
- **Does Pi 5 need a non-stock kernel?** Yes, today, per the maintainer. For a bootc image the kernel lives in the container, so the image must carry the Pi 5 kernel. Where that kernel is published (a repo or COPR the build could `dnf install` from) was **not found** in primary sources I could read; the Fedora discourse thread and nullr0ute blog do not link a package source. Ask in the thread or check the Fedora ARM list before building.

## Booting an installed bootc disk

- `fedora-bootc:44` is multi-arch (`arm64`, `amd64`, `ppc64le`, `s390x`). <https://quay.io/repository/fedora/fedora-bootc> *(carried over)*
- `bootc-image-builder` is now inside `osbuild/image-builder`. Its bootc image types (raw, qcow2, ISO and others) have an aarch64 ESP but **no Raspberry Pi definition**. <https://github.com/osbuild/image-builder/blob/main/data/distrodefs/bootc-generic/imagetypes.yaml> *(carried over)*
- AlmaLinux's `bootc-images-rpi` (Pi 5, 4, 3, Zero 2 W) states "bootc-image-builder alone isn't enough to build a raw boot image" because of the firmware. Firmware updates need manual steps or reimaging. <https://github.com/AlmaLinux/bootc-images-rpi>
- **[inference]** A Pi bootc disk therefore needs: the image, the install to a disk with an ESP, then Pi boot files copied onto that ESP (Pi 4: `rpi-u-boot.bin` or EDK2, `start4.elf`, `fixup4.dat`, `bcm2711-rpi-4-b.dtb`, overlays, `config.txt` with `kernel=rpi-u-boot.bin`; Pi 5 similarly with U-Boot and `bcm2712` files). Fedora's U-Boot then finds GRUB EFI on the ESP and boots the bootc deployment. That the whole chain works with bootc's ostree layout is **[unverified]**; nobody has published it for Fedora.

## Booting an installer ISO

- Anaconda aarch64 ISOs boot through GRUB EFI. A Pi only reaches EFI if U-Boot or EDK2 is already present; the stock EEPROM does not launch an EFI binary. **[inference]**
- Therefore a bare ISO written to a USB stick will not boot on a Pi 4 by itself. Options **[inference, all unverified]**:
  1. Put the Pi boot files (U-Boot or EDK2 plus firmware) into the ISO's ESP when building the aarch64 ISO.
  2. Provide a small separate SD card that holds only the EDK2 firmware (the "separate firmware disk" mode of Fedora CoreOS), then boot the stock aarch64 ISO from USB. This is the lowest-effort route for a Pi 4 and is documented for FCOS.
  3. Install by writing a prebuilt raw disk instead of an ISO (the AlmaLinux model), which conflicts with the map's "install artifact is an ISO" premise.
- Fedora does not document an Anaconda ISO install on Pi 4 for current releases; the search results pointed at `arm-image-installer` for Pi 4 and a 2020 PXE-install forum report for a Pi 3B+. <https://discussion.fedoraproject.org/t/fedora-aarch64-pxe-images-on-raspberry-pi-3b-4b/18634>
- Pi 5 ISO boot: the Fedora Pi 5 images are `arm-image-installer` disk images, not an Anaconda ISO. Combined with "SD only, no USB", an ISO on USB cannot boot a Pi 5 today. **[inference]** A Pi 5 would have to boot a microSD that holds U-Boot plus the installer.
- Anaconda bootc installs run `bootc install to-filesystem --bootloader=grub ...` (earlier ISO research, issue #14), so bootc installs GRUB, not Pi files. The Pi files still have to be added after install, in `%post` or by the installer container. **[inference]**

## Boot media

| Media | Pi 4 | Pi 5 |
|---|---|---|
| microSD | works | works (only Fedora-supported OS disk; some cards time out) |
| USB (mass storage) | works (EEPROM-based boot; EDK2/U-Boot on SD or ESP) | not supported by Fedora yet |
| NVMe | no native PCIe; NVMe-over-USB only **[inference]** | hardware can boot it; Fedora does not support it yet |

Sources: Raspberry Pi docs above, the nullr0ute post and the pftf/RPi4 README.

## Can bootupd manage the boot files?

No for Pi-specific files. bootupd's documented scope is GRUB and shim for UEFI (aarch64 included) plus BIOS GRUB. It has no U-Boot or Pi firmware component (README above). A Pi ESP therefore has two owners: bootupd for GRUB/shim and something of ours for `rpi-u-boot.bin`, firmware blobs and overlays. Options for the Firmware updates ticket **[inference]**: a systemd unit that syncs files from the image's `/usr/lib/...` to the ESP on boot; or a custom bootupd component upstream; or re-imaging.

## Open items (need a spike or hardware)

1. Does `fedora-bootc:44` (stock kernel, GRUB EFI via U-Boot) boot on a Pi 4 from USB with Pi files added to the ESP? This is the ticket-critical spike; the owner has a Pi 4.
2. Source and signing of the Pi 5 kernel, and whether it can be installed into a bootc image build.
3. Whether the ISO needs Pi files inside it (option 1) or a firmware SD (option 2).
4. Whether the Fedora docs sites, which blocked automated fetch (Anubis "Access Denied"), say anything newer about Pi bootc, Pi 5 USB/NVMe, or ISO installs. Fedora Discourse thread posts other than 1 and 24 and the Fedora ARM mailing list were not readable here.
5. `pftf/RPi5` 404: confirm there is no Pi 5 EDK2.

## Bearing on map #35 (not decisions)

- Pi 4: proceed; multi-arch image works, boot files are custom.
- Pi 5: stays best-effort. SD-only boot and a non-stock kernel make it a poor CI machine today; the ADR for a non-stock kernel/firmware, as the map anticipated, is needed before any Pi 5 support claim.
