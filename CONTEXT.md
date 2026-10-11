# PROVES CI OS

An immutable operating system image whose only job is to be a dedicated CI machine for building and testing the PROVES core reference flight software.

## Language

**Image**:
The bootable, immutable OS artifact published by this repo. Installed on a CI machine and updated in place.

**CI machine**:
A physical x86_64 or aarch64 (Raspberry Pi 4 or 5) machine on wired Ethernet that runs the Image and does nothing except serve as a Runner host. Not a developer workstation.

**Runner**:
The GitHub Actions self-hosted runner service that starts on boot on a CI machine and picks up jobs for the Open-Source-Space-Foundation organization.

**Core reference**:
The `proves-core-reference` repository: the firmware project the CI machine exists to build and test.

**Contributor**:
Someone in the PROVES project who sets up their own CI machine from the Image by following the setup documentation.

**Hardware-in-the-loop (HIL)**:
CI jobs that flash and talk to physical flight boards attached to a CI machine over USB/serial.

**Tailnet**:
The Tailscale network a CI machine joins. Which tailnet is the Contributor's choice; its access rules (ACL) decide who may ssh in.

**Ops account**:
The single shared local account Contributors become when they ssh in over Tailscale. Distinct from the Runner's account, which cannot be logged into.

**Console password**:
The password for the Ops account, accepted only at the physical console and for sudo. Set at install, never baked into the Image. The basis of the documented recovery process.

**Registration token**:
The one-hour token an organization owner mints so a Contributor can register a Runner. Passed to the register command on stdin or from a file, never baked into the Image.

**Seed**:
The pinned runner tarball shipped inside the Image and copied to the runner's writable directory on first boot. After registration the Runner updates itself.

**Rig**:
A CI machine with flight boards and a power supply attached, able to run HIL jobs. Registers its Runner with the `integration` label; a CI machine without boards does not.
