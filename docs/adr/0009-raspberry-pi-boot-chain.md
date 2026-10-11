# Boot Raspberry Pi CI machines with U-Boot from microSD, Pi 4 verified and Pi 5 best-effort

- Status: proposed

The Image is published as one multi-arch manifest (x86_64 and aarch64). A Raspberry Pi 4 or Pi 5 on wired Ethernet is a CI machine like any other, and updates stay `bootc upgrade`. Pi boot differs from PC boot, so this ADR fixes the boot chain.

## Decision

- **Firmware and bootloader**: both boards boot U-Boot (EFI) from the ESP, using the Fedora RPM files (`uboot-images-armv8`, `bcm2711-firmware`, `bcm283x-firmware`, `bcm283x-overlays`), which U-Boot hands to GRUB EFI and the bootc deployment. One mechanism for both boards, no third-party blobs.
- **EEPROM**: the SPI EEPROM is a one-time manual prerequisite, brought current with Raspberry Pi Imager before install and documented on the setup page. The Image does not flash it.
- **Boot media**: microSD for both boards. Pi 5 supports nothing else in Fedora; Pi 4 uses it for uniformity.
- **Boot-file updates**: bootupd does not manage U-Boot or Pi firmware. The Image carries these files under `/usr/lib/` and a systemd unit syncs them to the ESP when their hashes differ, writing a temporary file and renaming it so a re-run is safe. A bad `rpi-u-boot.bin` leaves the board unbootable until its SD is reflashed; Renovate bumps and the structure tests are the guard.
- **Install**: Pi 4 installs from the ISO written to a USB stick, with the U-Boot and firmware files embedded in the ISO's ESP, onto its microSD. Pi 5 has one boot slot, so it cannot use the ISO; its install path (likely a raw disk image) is left to the Pi 5 follow-up.
- **Pi 5 is best-effort**: it needs a non-stock kernel (Fedora 44 images ship `6.19.9-300.pr1.fc44`, source unpublished, and RP1 Ethernet and USB support was missing upstream in the stock kernel at last report) and no Fedora Atomic or bootc image exists for it. The Image carries the stock kernel. Pi 5 gets a support claim only after the kernel source is known and someone with a board validates it, recorded in a later ADR. Wi-Fi and Bluetooth are out of scope on every board.

## Considered Options

- **EDK2 (pftf/RPi4) on Pi 4**: real UEFI and ACPI, but a third-party release rather than an RPM, a 3 GB RAM limit quirk, and no Pi 5 equivalent.
- **Automated EEPROM flashing**: removes a manual step but adds a brick risk on unattended machines.
- **USB SSD boot on Pi 4**: more durable under CI writes, but a different path from Pi 5; deferred.
- **Raw disk image for both boards**: one flow, but drops the ISO prompts (ADR 0008).

## Status: proposed

Nobody has published fedora-bootc on a Pi. That U-Boot finds GRUB and the bootc ostree layout, that the ISO boots with the Pi files embedded, and that the sync unit works are unproven until the Pi 4 boot task finishes. Accepted then, or superseded.
