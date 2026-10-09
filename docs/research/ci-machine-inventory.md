# CI machine and core-reference CI tooling inventory

Resolves proves-ci-os#4. Part of map #1.

Inspected 2026-10-08, read-only, over SSH to `spacelab@100.70.111.57`
(runner name `spacelab-ubuntu-macmini`). No credentials, tokens or env values
were copied; for the runner `.env` only variable names are recorded.

Sources for the "needed" column are `proves-core-reference` at its current
checkout: `.github/workflows/ci.yaml`, `.github/actions/flash-firmware/action.yml`,
`Makefile`, `makelib/*.mk`, `tools/ci/detect-board-tty.sh`,
`docs-site/additional-resources/self-hosted-runner-setup.md`.

## Summary

- Two self-hosted machines are involved. Only one was inspected:
  - `spacelab-ubuntu-macmini` (this machine), label `integration`: jobs `integration-uart` and `integration-radio`. Confirmed from job metadata of a successful `main` run (`gh api repos/Open-Source-Space-Foundation/proves-core-reference/actions/runs/37793981632/jobs`).
  - `northland-mac-mini`, label `deathstar`: job `build`. Not inspected. Everything the `build` job needs (Zephyr SDK, west workspace, uv, toolchain) is a separate inventory.
- `lint`, `unit-test` and `yamcs-build` run on GitHub-hosted `ubuntu-latest`; they need nothing from either machine.
- Runner labels cannot be read from the machine (labels live server-side; `.runner` holds none) and the org runners API returned 403 for the current gh login. Label `integration` and runner group `proves` are taken from the setup doc and job metadata.
- The runner is not a systemd service. `run.sh` is running under an interactive `pts/0` shell (runner 2.337.0, no `actions.runner.*` unit, no crontab). It will not survive a reboot. The setup doc says to ensure the runner restarts after reboot, but nothing does.
- The machine is a general-purpose Ubuntu 24.04.5 desktop (GDM, GNOME, snapd, CUPS, RustDesk, Tailscale, kernel 6.14.0-37-generic) on a Mac mini. Much of what is installed is unrelated to CI.

## Tool table

"Needed" refers to the `integration` jobs on this machine unless stated.

| Tool | Version on machine | Provenance | Needed by repo? | Fedora equivalent |
|---|---|---|---|---|
| Ubuntu | 24.04.5 LTS, kernel 6.14.0-37 | OS | n/a | Fedora (target) |
| GitHub Actions runner | 2.337.0 (`bin.2.337.0`; tarball 2.333.1 still in dir; self-updated) | GitHub runner tarball, `config.sh`, started by hand with `run.sh` | Yes | Same tarball (not packaged); needs a systemd unit (`svc.sh` or own unit) |
| openocd | `0.12.0+dev-00002-gcd4873400 (2026-03-17)`; `git describe`: `sdk-2.2.0-2-gcd4873400`; source in `~/openocd`, binary `~/openocd/src/openocd` (not on PATH) | Built from `https://github.com/raspberrypi/openocd`, `./configure --enable-cmsis-dap` (per config.log), built Mar 16 2026 | Yes. The `flash-firmware` action hardcodes `~/openocd/src/openocd -s ~/openocd/tcl -f interface/cmsis-dap.cfg -f target/rp2350.cfg` | Build from source. Distro `openocd` is upstream OpenOCD and may lack RP2350 support from the RPi fork (unverified). Build deps: `autoconf automake libtool pkgconf-pkg-config texinfo libusb1-devel hidapi-devel gcc make` |
| picotool | `v2.2.0-a4 (Linux, GNU-11.4.0, Release)` at `/usr/local/bin/picotool` plus `enc_bootloader*.elf`, `xip_ram_perms.elf`, `rp2350_otp_contents.json`, cmake config files | Prebuilt `pico-sdk-tools` v2.2.0-2 tarball `picotool-2.2.0-a4-x86_64-lin.tar.gz` (still in `~/Downloads`), copied to `/usr/local/bin` per the setup doc. Dated Sep 12 2025 | No. The setup doc calls it "needed for flashing", but no workflow, action or Makefile target calls it (grep of the repo outside that doc finds nothing). Flashing is OpenOCD only | `picotool` if Fedora packages it (check `dnf info picotool`; unverified), else the same prebuilt tarball. Low priority |
| Zephyr SDK | 0.17.4 at `~/zephyr-sdk-0.17.4`; `arm-zephyr-eabi-gcc` 12.2.0; `~/.cmake/packages/{Zephyr,Zephyr-sdk,ZephyrUnittest}` present | `west sdk install` output (dir dated Jan 3 2026), registered via the cmake package registry | Not by the `integration` jobs. Needed by the `build` job (on `deathstar`) via `make zephyr-sdk`, which installs `~/zephyr-sdk-$(cat zephyr/SDK_VERSION)` with `--toolchains arm-zephyr-eabi`. Likely a leftover here | Same upstream mechanism (`west sdk install`); not a distro package |
| Core-reference checkout | `~/actions-runner/_work/proves-core-reference/proves-core-reference`, plus `~/code/proves-core-reference` | git clone (setup doc step 5) | Jobs use `actions/checkout`; the pre-clone is only a warm cache | n/a |
| uv | 0.9.21 in `~/.local/bin` (`uv`, `uvx`); repo pins 0.8.13 | Standalone installer for the user. The repo's `make uv` separately downloads 0.8.13 into `bin/` from `astral.sh` | Repo fetches its own copy; the machine copy is incidental (it created the korad venv) | `uv` (in Fedora archive, unverified) or let `make uv` fetch |
| Python (system) | 3.12.3 | Ubuntu | Repo `.python-version` is 3.13, supplied by uv, not the system | `python3`; uv supplies 3.13 |
| Python 3.13.7 | uv-managed CPython under `~/.local/share/uv/python` | uv | Used by `~/korad_control/.venv` | via uv |
| pytest, fprime-gds, yamcs-client, west, pre-commit | not on PATH | Installed into the repo's `fprime-venv` by `make fprime-venv` (`requirements.txt`) | Yes, from the venv | via uv |
| Java | `openjdk-21-jre-headless` 21.0.12 (apt, pulled by ca-certificates-java); Temurin 17 in `_work/_tool/Java_Temurin-Hotspot_jdk` | apt; Temurin from `actions/setup-java@v4` (`temurin`, 17) in `integration-uart` | YAMCS needs Java 11+ (`make yamcs`). The job installs Temurin 17 itself, so system Java is not needed | `java-21-openjdk-headless` (optional; setup-java supplies 17) |
| Docker / compose | not installed | n/a | Only `yamcs-build`, `make yamcs-server`, `make yamcs-build-check`. These run on `ubuntu-latest`, not here | `moby-engine` + compose, or `podman` + `podman-compose`, if ever needed locally |
| git | 2.43.0 | apt | Yes (checkout, submodules) | `git` |
| make | GNU Make 4.3 | apt (`build-essential`) | Yes | `make` |
| cmake | 3.28.3 | apt | Not by `integration`. Used by `make test-unit` and Zephyr builds elsewhere | `cmake` |
| gcc / g++ | 13.3.0 | apt (`build-essential`) | Only to build openocd and Python wheels | `gcc gcc-c++` |
| autoconf, automake, libtool | 2.71 / 1.16.5 / 2.4.7 | apt, manually installed | Only to build openocd | `autoconf automake libtool` |
| ninja, dtc, gperf, ccache, clang, node, protoc, west on PATH | not installed | n/a | Not needed here | n/a |
| libusb | `libusb-1.0-0-dev` 1.0.27 (manual) | apt | openocd and picotool | `libusb1-devel` |
| hidapi | `libhidapi-dev`, `-libusb0`, `-hidraw0` 0.14.0 (dev and libusb0 manual) | apt | openocd CMSIS-DAP | `hidapi-devel` |
| libudev-dev | 255.4 | apt | openocd build | `systemd-devel` |
| libmbedtls-dev | not installed | n/a | `unit-test` only, on ubuntu-latest | `mbedtls-devel` if unit tests ever move here |
| curl, wget | present | apt | curl: yes (`make uv`, YAMCS readiness loop). wget: setup doc only | `curl wget` |
| jq | 1.7 | apt | Only `make yamcs-build-check` (ubuntu-latest) | `jq` |
| lsof, psmisc, procps | lsof 4.95, psmisc 23.7 | apt | Yes: steps call `fuser -k /dev/ttyBOARD`, `lsof /dev/ttyBOARD`, `pkill -9 python` | `lsof psmisc procps-ng` |
| usbutils (`lsusb`) | present | apt | Yes: diagnostics in `detect-board-tty.sh` and the radio job | `usbutils` |
| bash | Ubuntu default | apt | Yes: the `.env` loader step uses `[[ ]]` | `bash` |
| korad_control | commit `ed806d8` (2019-03-28), `~/korad_control`; no local changes recorded | `git clone https://github.com/dj-on-github/korad_control`; `.venv` is uv-created (Python 3.13.7) with `pyserial 3.5`, `pyyaml 6.0.3`, `future 1.0.0`, `iso8601 2.1.0` (no pip in the venv) | Yes. Hard-coded `~/korad_control/.venv/bin/python ~/korad_control/korad_control.py -d /dev/ttyPWR --output {0,1}` | Same clone, then `uv venv` and `uv pip install pyserial pyyaml future iso8601` |
| tailscale | apt, manually installed | Tailscale apt repo | Access path to the machine, not CI | `tailscale` from Tailscale's Fedora repo |
| rustdesk, lazygit, UxPlay, create_ap, snap firefox, GNOME desktop, CUPS | present | various | No | Do not carry over. Disable ModemManager on Fedora: it probes CDC ACM ports (running here today) |

## Device access, udev, groups

User `spacelab` (uid 1000) groups: `adm dialout cdrom sudo dip plugdev users lpadmin`. The relevant ones are `dialout` (ttyACM) and `plugdev` (USB / CMSIS-DAP), matching setup doc step 2. Fedora has `dialout`; `plugdev` must be created if the rules are kept as-is.

Rules in `/etc/udev/rules.d` that matter:

| File | Content | Provenance |
|---|---|---|
| `60-picotool.rules` | usb `2e8a` pids `0003 0009 000a 000f`, `MODE=660 GROUP=plugdev TAG+=uaccess` | Upstream `raspberrypi/picotool` `udev/60-picotool.rules` (setup doc) |
| `99-openocd.rules` | List of debug-probe VID/PIDs with `MODE=664 GROUP=plugdev`, plus `ATTRS{product}=="*CMSIS-DAP*"` | Matches upstream OpenOCD's `60-openocd.rules`, renamed. A source build needs this file installed by hand |
| `99-custom.rules` | Stable symlinks: `ttyBOARD` (0028:000f, Zephyr CDC ACM), `ttyDBG` (2e8a:000c, RPi Debug Probe), `ttyPWR` (0416:5011), `ttyGDS` (one specific device matched by USB serial number, interface 02) | Hand-written; not documented in core-reference |

Notes on the symlinks as used by CI:

- `ttyPWR` is required by every power-cycle step (0416:5011 Winbond Virtual Com Port, presumably the Korad PSU). `ttyBOARD` is required by the YAMCS phase (`UART_DEVICE=/dev/ttyBOARD`, wait loop, `fuser -k`). `ttyGDS` and `ttyDBG` are not referenced by the repo.
- `GDS_PASSTHROUGH` in the runner `.env` is the optional override for the LoRa passthrough tty (`${GDS_PASSTHROUGH:-$LORA_PASSTHROUGH_TTY}` in the radio job).
- At inspection time `/dev/ttyBOARD` did not exist (no 0028:000f device on the bus) and `ttyPWR -> ttyACM1`. USB devices seen: RPi Debug Probe `2e8a:000c`, Winbond VCP `0416:5011`, "PROVESKit RP2350 V5d" `1209:e004` (likely the CircuitPython LoRa passthrough board), and Apple Bluetooth/IR.
- `detect-board-tty.sh` and the radio job read sysfs under `/sys/bus/usb/devices` directly; no extra tooling.

## Runner configuration

- Install dir `~/actions-runner`, work folder `_work`, org-level registration at `https://github.com/Open-Source-Space-Foundation`, agent name `spacelab-ubuntu-macmini`. Runner group `proves` and label `integration` per the setup doc; not independently verifiable (403 on the runners API).
- `.env` in the runner dir defines three variable names: `LANG`, `KORAD_CONTROL`, `GDS_PASSTHROUGH`. Values are not recorded here. The workflow reads this file by absolute path (`~/actions-runner/.env`) in `Load .env file`, so its location is a hard dependency. The repo does not reference `KORAD_CONTROL`.
- `.path` is the stock PATH plus `~/.local/bin`. `~/openocd` is not on it, hence the hardcoded paths.
- Tool cache `_work/_tool` holds only Temurin JDK 17.
- `AUTH_KEY` comes from GitHub Secrets at run time and is not stored on the machine. The runner's own `.credentials*` files were not read.

## Drift and risks

1. OpenOCD pin mismatch: the repo Makefile pins `OPENOCD_REF=acff23f`, builds into `tools/openocd` with `--enable-cmsis-dap --enable-cmsis-dap-v2`. The machine has a build at `cd4873400` in `~/openocd` with only `--enable-cmsis-dap`, and CI flashing uses that one. `make debug` and CI flashing are different binaries.
2. picotool is installed and documented but unused by CI.
3. The runner is a foreground shell process with no autostart.
4. `integration` jobs depend on absolute home-relative paths `~/openocd`, `~/korad_control`, `~/actions-runner/.env`, and on udev symlinks `/dev/ttyPWR` and `/dev/ttyBOARD`. A Fedora rebuild must reproduce these, or the repo must change.
5. This machine needs no Zephyr SDK or west for `integration`; SDK 0.17.4 and `~/.cmake/packages` look like leftovers from when it also built.
6. korad_control is an unmodified 2019 upstream commit, not pinned by the repo.

## Minimum Fedora set for the `integration` role

Packages (names from knowledge of the Fedora archive, not verified on a Fedora host):
`git make curl psmisc lsof procps-ng usbutils jq gcc gcc-c++ autoconf automake libtool pkgconf-pkg-config texinfo libusb1-devel hidapi-devel systemd-devel uv`.

Then: build the RPi openocd fork into `~/openocd`; clone korad_control to `~/korad_control` with a uv venv; install the three udev files; add the user to `dialout` (and `plugdev`); install the Actions runner with label `integration` and group `proves`, run as a systemd service; recreate the runner `.env`. Optional: `picotool`, `java-21-openjdk-headless`.
