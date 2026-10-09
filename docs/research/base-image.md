# Research: base image, Wi-Fi, and Broadcom `wl`

Resolves issue #3 (map: #1). Researched 2026-10-08.

Question: is headless `quay.io/fedora/fedora-bootc` available for x86_64 and
aarch64, which tags can Renovate track, how does headless Wi-Fi work with
NetworkManager, is the Broadcom `wl` akmod needed and buildable for the Intel
Mac mini CI box, and is a desktop genuinely required for Wi-Fi setup?

Evidence labels: **[registry]** queried quay.io directly; **[image]** ran the
image (`podman run`) and inspected it; **[box]** read-only `ssh` inspection of
the Mac mini; **[doc]** first-party documentation (URL given).

## Recommendation

1. **Base image: `quay.io/fedora/fedora-bootc:44`, pinned by digest, headless.**
   No desktop. Matches the stableOS pin style (`:44@sha256:...`).
2. **A desktop is not required for Wi-Fi setup.** NetworkManager's `nmtui` is
   already in the base image and runs on a plain text console. The one thing the
   base image is missing is the Wi-Fi plugin: **add `NetworkManager-wifi`**
   (pulls `wpa_supplicant`, `iw`, `wireless-regdb`).
3. **Prefer Ethernet for CI boxes.** The Mac mini is on wired Ethernet today
   (in-kernel `tg3`). Wi-Fi is a convenience, not a dependency.
4. **Broadcom `wl` is needed only for Wi-Fi on the Mac mini's BCM4360**, and is
   not needed for the Mac mini to boot, get a network, or run CI. Keep it out of
   the default image; if wanted, ship it as an opt-in variant (separate tag or
   build target) so the Pi/PC image stays free of proprietary, tainted-kernel
   code. Decision for the image-layout ticket.
5. **Raspberry Pi 4/5 Wi-Fi works from the stock image** once
   `NetworkManager-wifi` is added; no extra driver or firmware package.

## 1. Availability and tags

- `quay.io/fedora/fedora-bootc` is the Fedora "full" bootc image: kernel,
  systemd, bootc, NetworkManager, podman, filesystem tools; no cloud agents.
  [doc: https://gitlab.com/fedora/bootc/docs/-/raw/main/modules/ROOT/pages/base-images.adoc]
- Multi-arch manifests exist for `44`, `43`, `42`, `latest`, `rawhide`: each
  lists `amd64`, `arm64`, `ppc64le`, `s390x`. [registry:
  `GET https://quay.io/v2/fedora/fedora-bootc/manifests/<tag>`]
- Tag families [registry: `https://quay.io/api/v1/repository/fedora/fedora-bootc/tag/`]:
  - Floating per release: `42`, `43`, `44`, `45`, `46`; plus per-arch
    `44-x86_64`, `44-aarch64`. `latest` = `44` (same digest today). `rawhide` =
    `46`. `45` is the in-development branch (pre-GA as of today).
  - Immutable date tags (`43.20250717.0`) exist **only** from July 2025; no
    date tags for 44. Do not rely on them; pin by digest.
  - `42` last rebuilt 2026-05-13 (EOL, no longer rebuilt); `43` and `44` are
    rebuilt continuously.
- Fedora lifecycle: release N is maintained until four weeks after N+2 ships
  (about 13 months). F43 and F44 are listed as supported, F42 as EOL.
  [doc: https://docs.fedoraproject.org/en-US/releases/lifecycle/]
- **Renovate**: track the floating tag with `pinDigests: true` (digest PRs on
  every rebuild). Cap the major so Renovate does not jump to the pre-GA `45`/
  `46`/`rawhide` tags: `allowedVersions: "<=44"`, bumped by hand after GA. This
  is exactly the pattern in stableOS `renovate.json` (reference impl, checked
  against its Containerfile `FROM ...:44@sha256:...`). Fedora rebuilds the
  floating tags daily-ish (all of `43`, `44`, `45`, `46` rebuilt 2026-10-08), so
  expect frequent digest PRs; `automerge` after CI is sensible.
- Alternatives considered: the "minimal" bootc demo image has only
  `kernel systemd bootc` (no networking) and is not on Quay, so it's unsuitable
  [doc: base-images.adoc]. Atomic desktops (`cosmic-atomic` etc.) carry a
  desktop we explicitly do not want. No alternative beats `fedora-bootc` here.

## 2. What is in the base image (inspected)

Ran `quay.io/fedora/fedora-bootc:44` on both `linux/amd64` and `linux/arm64`
**[image]**. Fedora 44, kernel `7.2.9-200.fc44`, `bootc 1.16.13`,
`NetworkManager 1.56.1`.

| Item | State |
|---|---|
| `NetworkManager`, `NetworkManager-tui` (`nmtui`) | present |
| `NetworkManager-wifi`, `wpa_supplicant`, `iw` | **absent**; `dnf5 install NetworkManager-wifi` adds `wpa_supplicant`, `iw`, `wireless-regdb` (~7 MiB) |
| `linux-firmware` split packages | `brcmfmac-firmware`, `atheros-`, `mt7xxx-`, `realtek-`, `intel-gpu/audio/npu-`, `amd-` ... present. No `iwlwifi` firmware package is installed (Intel Wi-Fi cards would need it added) |
| Kernel modules | `brcmfmac`, `brcmsmac`, `b43`, `bcma`, `iwlwifi`, `tg3`, `cfg80211` all shipped |

Consequence: without `NetworkManager-wifi` the stock image cannot do Wi-Fi at
all. This is the single required addition for Wi-Fi.

## 3. Broadcom `wl` on the Intel Mac mini

**[box]** (Ubuntu 24.04, read-only):

- Wi-Fi: `02:00.0 Broadcom BCM4360 802.11ac [14e4:43a0]`.
- Ethernet: `03:00.0 Broadcom NetXtreme BCM57766 [14e4:1686]`, interface
  `enp3s0f0` is **UP**; this is the link CI uses.
- The Ubuntu box loads the proprietary `wl` module; `wlp2s0` is `NO-CARRIER`
  (Wi-Fi unused). Bluetooth is a separate USB device (`05ac:8289`).

**[image]** driver coverage in Fedora's 7.2.9 x86_64 kernel for `14e4:43a0`:

- `brcmfmac`: no match. `brcmsmac`: no match. `b43`: no match.
- Only `bcma` (the bus layer) claims the device. So **no in-kernel Wi-Fi driver
  binds the BCM4360**; `wl` is the only working driver.
- `tg3` has the alias for `14e4:1686`, so the wired NIC works with the stock
  kernel.

So: `wl` is needed **only** to use the Mac mini's Wi-Fi, and Ethernet makes
that optional.

**Is `wl` buildable on Fedora 44?** RPM Fusion nonfree provides, for F44,
`akmod-wl 6.30.223.271-63.fc44`, `kmod-wl` (same version) and
`broadcom-wl 6.30.223.271-28.fc44` **[image: `dnf5 repoquery` after
installing `rpmfusion-nonfree-release-44`]**.

Reference build: stableOS `Containerfile` installs RPM Fusion release RPMs,
`akmods` + `kernel-devel-<image kernel>`, installs `broadcom-wl` with
`--setopt=tsflags=noscripts` (akmods refuses to run as root; the `%post`
scriptlet fails in a container), then runs `akmods --force --kernels <ver>` and
asserts `modinfo -k <ver> wl`. Its latest CI build on `cosmic-atomic:44` passed
on 2026-10-08 (`gh run list -R nateinaction/stableOS`). Caveats:

- That proves the recipe on `cosmic-atomic:44`, a sibling Fedora 44 image. I did
  not confirm it shares the exact kernel with `fedora-bootc:44`.
- My own attempt to repeat the build for `fedora-bootc:44` x86_64 ran under
  emulation on an Apple Silicon host and failed in `%prep` with
  `tar: Cannot open: Function not implemented`. That is an emulation
  limitation, not evidence about `wl`; the install/resolve steps all succeeded.
  **Unverified natively**: a real x86_64 build is a task for the image
  backlog (it falls out of any CI build of the wl variant).
- `wl` is proprietary and taints the kernel; RPM Fusion distributes it in
  `nonfree`. Check redistribution terms before publishing a prebuilt `wl` module
  inside a public ghcr image (not verified here). Building from the
  `akmod` at image-build time is the stableOS approach.
- The akmod must be rebuilt for every kernel in every rebuilt base digest, so a
  `wl` variant couples to the base-image update cadence (digest PRs will
  occasionally fail until RPM Fusion catches up to a new kernel).

## 4. Headless Wi-Fi setup with NetworkManager

All routes end in NetworkManager keyfiles. Keyfiles must be mode `600` or NM
ignores them. Image-owned profiles go in `/usr/lib/NetworkManager/system-connections/`;
per-machine ones in `/etc/NetworkManager/system-connections/`.
[doc: https://gitlab.com/fedora/bootc/docs/-/raw/main/modules/ROOT/pages/sysconfig-network-configuration.adoc]

Without configuration, NM does DHCP on every wired interface with a cable
plugged in [same doc], so **Ethernet needs zero setup**.

Options for Wi-Fi, in order of fit:

1. **`nmtui` on the text console** (keyboard + monitor, no desktop). `nmtui` is a
   text UI and is in the base image [image; doc:
   https://networkmanager.dev/docs/api/latest/nmtui.html]. `nmcli device wifi
   connect <ssid> --ask` is the scriptable equivalent
   [doc: https://networkmanager.dev/docs/api/latest/nmcli.html].
   Needs `NetworkManager-wifi` baked in. This is the non-technical path, and it
   is not desktop-dependent.
2. **Anaconda kickstart** `network --device=<wlan> --essid=... --wpakey=...`
   [doc: https://pykickstart.readthedocs.io/en/latest/kickstart-docs.html]. Fedora
   documents kickstart Wi-Fi as supported for bare-metal bootc installs
   (stored per-machine under `/etc/NetworkManager/system-connections/`).
   [doc: sysconfig-network-configuration.adoc above]
3. **Pre-generate a keyfile** with `nmcli --offline connection add type wifi ...`
   (prints keyfile, no daemon needed) and drop it on the installed disk at mode
   600. [doc: sysconfig-network-configuration.adoc]
4. **Via bootc-image-builder / image-builder config: NOT possible.** For bootc
   image types the supported blueprint customizations are `kernel.append`,
   `bootloader`, `user` (name, password, **ssh `key`**, groups, ...), `group`,
   `filesystem`, `disk`, `installer.kickstart` and `installer.modules`.
   `hostname`, `files`/`directories`, `network`, and `firewall` are
   **not supported for bootc**.
   [doc: https://osbuild.org/docs/user-guide/blueprint-reference/, items marked
   "bootc"] So a Wi-Fi profile cannot be pre-seeded via TOML; it can only go in
   via the `installer.kickstart` route (anaconda-iso type) or after install.
5. **Bake a profile into the image: rejected.** The image is public on ghcr and
   a PSK is a secret; this also violates the map's "secrets never baked" rule.

SSH access for contributors (map premise): `customizations.user.key` in the
builder config, or `bootc install ... --root-ssh-authorized-keys`, or a
kickstart `sshkey`. [doc: blueprint-reference; bare-metal.adoc at
https://gitlab.com/fedora/bootc/docs/-/raw/main/modules/ROOT/pages/bare-metal.adoc]

Chicken-and-egg: a Wi-Fi-only box with no display cannot be configured over
SSH. It needs either console access once, or kickstart/pre-generated keyfile at
install time. Ethernet avoids this entirely.

## 5. Is a desktop required?

**No.** Evidence: `nmtui` is text-mode, ships in the base image, and
`NetworkManager-wifi` is a 330 KiB package away. Kickstart and offline-generated
keyfiles remove even the console need. A desktop (COSMIC etc.) adds a large
attack and update surface to a CI-only machine for no capability the TUI lacks.
Caveat: "non-technical" users still need a monitor+keyboard, or Ethernet, for
first boot; docs should lead with Ethernet.

## 6. Raspberry Pi 4/5 notes (scope: Wi-Fi/driver only)

**[image, aarch64]** the image carries `bcm2711-rpi-4-b.dtb`,
`bcm2712-rpi-5-b.dtb`, the `brcmfmac` driver, and board NVRAM files
`brcmfmac43455-sdio.raspberrypi,4-model-b.txt.xz` and `...,5-model-b.txt.xz`
(+ `cyfmac43455-sdio.bin`). The Pi's onboard Wi-Fi therefore needs only
`NetworkManager-wifi`. **Not researched**: how a Pi boots a bootc image
(firmware/U-Boot/UEFI, install media). The Fedora bootc docs do not cover
Raspberry Pi; that belongs to the install-flow tickets (the issue blocks #8,
#10, #11).

## 7. Open points handed forward

- `wl` variant: separate tag vs build arg vs not at all (Ethernet works).
  Natively verify the x86_64 akmod build and check `wl` redistribution terms.
- `bootc-image-builder` is being deprecated in favor of `image-builder`
  (`--bootc-ref`); the bib repo is still unarchived and active, but its README
  is a stub pointing to `osbuild/image-builder`. RHEL 10 compat is promised; for
  a new project prefer `image-builder` or plain `bootc install`.
  [doc: https://osbuild.org/docs/bootc/deprecation-notice/]
- The Intel Wi-Fi case: no `iwlwifi` firmware package is in the base image; a
  contributor PC with Intel Wi-Fi would need that added (not researched further;
  Ethernet remains the baseline).

## Sources

- Fedora bootc docs (base images, network config, bare metal):
  https://gitlab.com/fedora/bootc/docs/-/tree/main/modules/ROOT/pages
- Fedora release lifecycle: https://docs.fedoraproject.org/en-US/releases/lifecycle/
- Quay registry API: https://quay.io/api/v1/repository/fedora/fedora-bootc/tag/
- osbuild blueprint reference: https://osbuild.org/docs/user-guide/blueprint-reference/
- osbuild bib deprecation: https://osbuild.org/docs/bootc/deprecation-notice/
- NetworkManager: https://networkmanager.dev/docs/api/latest/nmtui.html,
  https://networkmanager.dev/docs/api/latest/nmcli.html
- pykickstart `network`: https://pykickstart.readthedocs.io/en/latest/kickstart-docs.html
- Reference impl: `nateinaction/stableOS` `Containerfile`, `renovate.json`
- Target box: read-only `lspci -nn`, `lsmod`, `ip link` over ssh
