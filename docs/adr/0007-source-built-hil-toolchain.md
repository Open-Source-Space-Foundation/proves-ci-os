# Build OpenOCD from source and bake `korad_control` into the Image

- Status: accepted

The core reference's hardware-in-the-loop jobs flash with the Raspberry Pi fork of OpenOCD and power-cycle boards with `korad_control`, at hardcoded paths under the runner's home (`~/openocd`, `~/korad_control`). Neither is a usable Fedora package: distro OpenOCD is upstream and may lack RP2350 support (unverified), and `korad_control` is an unpackaged 2019 upstream script.

## Decision

- A `Containerfile` builder stage builds the Raspberry Pi OpenOCD fork at the pinned ref `sdk-2.2.0-2-gcd4873400` (what CI flashes with today) and installs it to `/usr/local`. A `tmpfiles.d` symlink makes `~runner/openocd/{src/openocd,tcl}` resolve so the workflow works unchanged.
- The same stage clones `korad_control` at the pinned upstream commit `ed806d8` and builds its venv at its final path `/usr/local/lib/korad_control`, using the `python3.13` RPM so the interpreter matches what works today. A `tmpfiles.d` symlink provides `~runner/korad_control`.
- Build-only packages stay in the builder stage. `uv`, Python 3.13 and the Zephyr SDK are not baked; the core reference downloads its own.
- Pins are `ARG`s tracked by Renovate custom managers ([ADR 0002](0002-automated-dependency-updates.md)).
- The udev rules ship in the Image with a `sysusers.d`-created `plugdev` group; `ttyBOARD`, `ttyPWR` and `ttyDBG` are matched by VID:PID. ModemManager is absent because it probes serial ports and can make them busy.

## Consequences

- Good: the core reference workflow runs unchanged and the machine is reproducible.
- Bad: the Image carries two pinned source builds, so they need their own updates. The paths are a hard dependency on the core reference; follow-ups there should remove the hardcoded paths and take ownership of `korad_control`.
- Open: confirm `python3.13` exists on Fedora 44 and that `korad_control` runs on it.
