# What an aarch64 build of the Image needs

Question (issue #37, map #35): walk the Containerfile, CI and tests for x86_64
assumptions; establish the arm64 runner tarball, whether every build input
builds or installs on aarch64, the arm self-hosted runner labels, and how a
multi-arch manifest is built and cosign-signed in the existing workflow.

Date: 2026-10-10. Repo state: `origin/main` at 2dbc7e5. "Verified" means I ran
it; "Not verified" means I could not.

## Answer in brief

- The Image builds natively on aarch64 with three edits (below), and all 49
  container-structure tests pass on the result. Verified on a native arm64
  podman (Apple silicon, `linux/arm64`), not under emulation.
- The edits: (1) arm64 runner tarball name and sha256, (2) add `which` to the
  `hil` stage, (3) `--platform linux/arm64` in the `structure-test` target.
- The one surprise: `quay.io/fedora/fedora-bootc:44` ships `which` on amd64 but
  not on arm64, and the OpenOCD `./bootstrap` needs it.
- Not established: the labels of the deathstar arm runner (the token lacks
  `admin:org`), and whether the baked signature policy accepts a signed manifest
  list end to end.

## 1. Runner seed tarball

- Release v2.338.0 is the latest, not a draft, not a prerelease, published
  2026-10-06T13:55:11Z. Source: `gh release view v2.338.0 -R actions/runner`.
- Asset: `actions-runner-linux-arm64-2.338.0.tar.gz`, 140,012,091 bytes.
- sha256: `628b4a7258487b80c1d3c221095a7ded349f5ae0175fd3dfab708d07428f041b`.
  The release notes carry it between `BEGIN SHA linux-arm64` markers and the
  GitHub asset `digest` field agrees. Same sources.
- The x64 pin in the Containerfile (`af4b794c...`) matches the release, so the
  arm64 value comes from the same table.
- The Containerfile hardcodes `actions-runner-linux-x64-` in the seed stage
  (line 57) and has one `RUNNER_SHA256`. Renovate's regex
  (`renovate.json`, "Runner seed tarball") matches `ARG RUNNER_VERSION` followed
  by `ARG RUNNER_SHA256` on adjacent lines, and `cmd/check-renovate-pins`
  asserts one manager per ARG. A second sha ARG for arm64 means changing the
  regex to capture two digests, or choosing the sha at build time from a table.
  Not designed here.
- Verified: the arm64 tarball seeded into the aarch64 image extracts and
  `bin/Runner.Listener --version` prints `2.338.0` inside the image, so the
  .NET runtime and the `lttng-ust`, `libicu`, `krb5-libs` deps are sufficient.
  `libcoreclr.so`, `libhostfxr.so` are present.

## 2. Build inputs on aarch64

All verified by building the Containerfile with `podman build --platform
linux/arm64` (full log not kept; the result is `localhost/t-arm`, 2.84 GB).

| Input | Result |
| --- | --- |
| `fedora-bootc:44` base | Multi-arch; pulls and runs as aarch64. Pinned digest `c24cce42...` is the manifest list, so one pin serves both arches. |
| Runtime RPMs (`git make cmake gcc gcc-c++ curl psmisc lsof procps-ng usbutils jq screen tailscale NetworkManager lttng-ust openssl-libs krb5-libs zlib-ng-compat libicu glibc-common libusb1 hidapi jimtcl python3.13`) | All install. aarch64 builds seen: `tailscale-1.104.1-1.aarch64`, `lttng-ust-2.14.0-5.fc44.aarch64`, `python3.13-3.13.15-1.fc44.aarch64`, `jimtcl-0.83-4.fc44.aarch64`. No Wi-Fi packages pulled (the `test -z` guard passed). |
| Tailscale repo | `tailscale.repo` uses `baseurl=https://pkgs.tailscale.com/stable/fedora/$basearch`; `.../aarch64/repodata/repomd.xml` and `.../x86_64/...` both return HTTP 200. Source: the repo file and curl. |
| `hil` stage build deps | Install. Build fails at `./bootstrap` unless `which` is installed (below). |
| RPi OpenOCD fork at `cd4873400` | Builds and runs on aarch64 after the `which` fix: `Open On-Chip Debugger 0.12.0+dev-00002-gcd4873400`; the binary is `ELF 64-bit LSB executable, ARM aarch64`. CMSIS-DAP is the only adapter enabled, so no Pi-specific GPIO driver is built (not needed). `openocd -c "adapter list"` runs. |
| korad_control venv | Builds. `pyserial`, `pyyaml`, `future`, `iso8601` install (pure Python or wheels) and `korad_control.py --help` runs. |
| Go binaries | Built in the same build; `golang:1.26` is multi-arch and `CGO_ENABLED=0` needs no toolchain change. `proves-runner` and `proves-update` are in the final image and the structure tests that execute them pass. |

### The `which` finding

On amd64, `command -v which` gives `/usr/sbin/which`. On arm64 it prints
nothing. The OpenOCD `bootstrap` script uses `which libtoolize` / `which
glibtoolize`, finds neither, and exits `Error: libtool is required` even
though `libtool` is installed (`/usr/bin/libtoolize` exists). Fix: add `which`
to the `hil` stage's `dnf install` list. The final stage also lacks `which` on
arm64; nothing in this repo needs it there, but the runner's `config.sh`
and core-reference workflows were written against x86_64 hosts that have it.
Not checked beyond this repo.

### Installer ISO (out of scope, noted)

`iso/Containerfile` installs `grub2-efi-x64-cdboot grub2-pc-modules`. On
fedora 44 aarch64 the package is `grub2-efi-aa64-cdboot`; `grub2-efi-x64-cdboot`
is not offered. `grub2-pc-modules` is noarch but is BIOS-only and irrelevant on
arm. `iso.yaml` is arch-neutral. Whether an aarch64 installer ISO is wanted is
for the Pi boot question (#36, #38); this only records the package name.

## 3. Tests and docs

x86_64 assumptions, from `grep -rniE "x86|amd64|x64|aarch|arm64"`:

- `Makefile` `structure-test`: `--platform linux/amd64`. Verified that with
  `--platform linux/arm64` all 49 tests pass on the arm image.
- `Makefile` `structure-test` loads the image into docker via `podman save |
  docker load`; the loaded image keeps its arch, so no other change.
- `CONTEXT.md` line 11 defines a CI machine as "a physical x86_64 machine";
  `README.md` lines 13 and 32 say x86_64 only. Docs, not code.
- `flake.nix` already lists both `aarch64-linux` and `x86_64-linux`.
- `tests/structure/*.yaml` have no arch-specific paths; the openocd and
  korad_control paths are identical.
- `.github/workflows/ci.yml` has no arch assumptions beyond `runs-on:
  ubuntu-24.04` (x64) and the `podman save` of a single-arch image.

## 4. Runner labels (arm self-hosted, group deathstar)

Not established. `gh api orgs/.../actions/runners` and `.../runner-groups`
return 403 ("needs admin:org") for this token, and no arm runner appears in
recent job history.

What is known:

- In the last 40 runs each of proves-core-reference and proves-ci-os, the
  deathstar group's only runner is `northland-mac-mini` (an x86_64 Mac mini by
  the repo's own docs), job label `deathstar`. The `integration` group has
  `spacelab-ubuntu-macmini`, label `integration`. The `labels` on a job record
  are the labels the job requested, not the runner's full set, so these do not
  list the runner's system labels.
- `runs-on: deathstar` in this repo is a group name or a label, not an
  architecture selector; the iso workflow cannot currently distinguish arch.
- The runner assigns default labels at registration: `self-hosted`, the OS and
  the architecture. Values are `X64` and `ARM64` (and `X86`, `ARM`) for
  arch, from `ConfigurationManager.cs` (lines 659-661) and
  `VarUtil.OSArchitecture` in actions/runner v2.338.0. Case matters in the
  source; label matching in `runs-on` is documented as case-insensitive (not
  re-checked here). So an arm machine registered with defaults carries
  `self-hosted`, `Linux`, `ARM64` and whatever `proves-runner register --labels`
  adds (`--labels` is required by the tool).
- GitHub-hosted alternative: `ubuntu-24.04-arm` (arm64) is listed in
  actions/runner-images README. Pricing/availability for this org not checked.

To close this gap an org admin can run `gh api
orgs/Open-Source-Space-Foundation/actions/runners --jq '.runners[] | {name,
labels: [.labels[].name]}'` with `admin:org`, or read the Runners settings
page.

## 5. Multi-arch manifest and signing in the existing workflow

Current `ci.yml` publish job:

1. `build-and-test` builds one image with `make build`, saves it as an
   oci-archive artifact, so the tested bytes are the published bytes.
2. `publish` does `skopeo copy oci-archive:image.tar docker://REPO:$GITHUB_SHA`
   and records the digest (`--digestfile`).
3. `cosign sign --key env://COSIGN_PRIVATE_KEY --new-bundle-format=false
   --use-signing-config=false --tlog-upload=false
   --registry-referrers-mode=legacy REPO@DIGEST`. Signs by digest, no tlog.
4. Verifies with `skopeo copy` inside a clean container using only the baked
   `policy.json`, `registries.d`, and public key.
5. Moves `latest` and the date tag with `skopeo copy --all
   --preserve-digests` from the signed digest.

What this means for multi-arch:

- Step 5 already copies every manifest in a list (`--all`) and preserves
  digests, so tag moves need no change.
- The signature attaches to a digest. For multi-arch, the thing to sign is the
  digest of the manifest list (index); cosign signs whatever digest it is
  given, including an index. Each per-arch manifest could be signed as well.
- Build options, none run here: (a) native per-arch jobs (`ubuntu-24.04` and
  `ubuntu-24.04-arm`, or the deathstar arm runner), each pushing a by-sha
  image, then a job that assembles the list with `podman manifest create/add/
  push` (or `skopeo`/`docker buildx imagetools create`), signs the list digest,
  then verifies; (b) one QEMU cross build, which the earlier research
  (`research/raspberry-pi-bootc`, citing osbuild docs) calls experimental and
  8-10x slower.
- The test gate "publish the exact image that passed" must become one tested
  artifact per arch; the structure-test job needs a matrix, and the
  `image.tar` artifact name becomes per-arch.

Not verified, and a real risk for #39:

- Whether `bootc`/`skopeo` with `sigstoreSigned` in the baked `policy.json`
  accepts a signature made over the manifest list when the client then resolves
  to a per-arch manifest, or requires a signature per arch digest. This is
  containers/image behavior I did not test. A cheap test: sign a throwaway
  two-arch index on ghcr.io with the existing flags, then run the existing
  "Verify with only the baked policy" step against `REPO@INDEX_DIGEST` and
  against `REPO@ARM64_DIGEST`.
- `--registry-referrers-mode=legacy` stores the signature as a `.sig` tag
  derived from the digest; that works for an index digest the same as for an
  image digest.

## What #39 needs, in order

1. Containerfile: arch-select the runner tarball and sha (and change the
   Renovate regex and `check-renovate-pins`); add `which` to `hil`.
2. Makefile: parameterize `--platform` (default amd64).
3. CI: per-arch build and test matrix; a job that assembles and signs the
   index, then verify both the index and each arch digest with the baked policy.
4. Decide the arm runner for the iso workflow once the labels are known.
5. Docs: `CONTEXT.md`, `README.md` x86_64 wording.

## Sources

- `gh release view v2.338.0 -R actions/runner` (assets, digests, release notes).
- https://github.com/actions/runner/blob/v2.338.0/src/Runner.Listener/Configuration/ConfigurationManager.cs and
  https://github.com/actions/runner/blob/v2.338.0/src/Runner.Common/Util/VarUtil.cs (default labels).
- https://pkgs.tailscale.com/stable/fedora/tailscale.repo and `.../aarch64/repodata/repomd.xml`.
- https://github.com/raspberrypi/openocd (ref `cd4873400`, `bootstrap`).
- https://github.com/dj-on-github/korad_control.
- https://github.com/actions/runner-images (README, `ubuntu-24.04-arm`).
- In-repo: `Containerfile`, `Makefile`, `.github/workflows/ci.yml`,
  `.github/workflows/iso.yml`, `iso/Containerfile`, `renovate.json`,
  `docs/adr/0004-hybrid-runner-versioning.md`, `docs/adr/0008-installer-iso.md`.
- Local runs: native arm64 podman 5.8.7; `dnf` in `fedora-bootc:44` arm64;
  `container-structure-test` 49/49 pass.
