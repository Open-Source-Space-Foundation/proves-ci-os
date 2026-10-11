# Install from a custom ISO that embeds the Image

- Status: proposed

A Contributor installs a CI machine from an Anaconda ISO built by `image-builder` that embeds the Image and a baked kickstart, so install works offline once the wired link is up and does not depend on the registry. The installer wipes the whole internal disk and asks only for the `ops` console password (hashed, never published), the hostname (the default Runner name) and the target disk. The kickstart fixes the accounts, wired DHCP, the Image reference, locale and UTC.

The ISO is trusted by a published checksum plus a cosign signature on the file. It is built on a release tag or manually, not on every merge, because installed machines update themselves with `bootc`. It is published as a GitHub Release asset if it fits 2 GiB, otherwise as a `ghcr.io` OCI artifact. The CI job that builds it runs on the organization's `deathstar` self-hosted runners (a `runs-on` of `group: deathstar` and the `Linux` and `X64` labels), not GitHub-hosted ones.

## Considered Options

- **Stock Fedora ISO plus a published kickstart**: nothing custom to host, but the boot step needs a kernel argument and prompts depend on partial kickstarts.
- **`bootc install to-disk` from a live system**: simplest tooling, but no user creation, so first-boot setup grows.
- **Custom ISO**: chosen, for one artifact and one documented path.

## Status: proposed

The mechanism is unproven. Research found that `image-builder` documents no `anaconda-iso` type for bootc images; `bootc-generic-iso` ignores the blueprint, and only the discouraged `bootc-installer` type accepts a baked kickstart. Whether the password, hostname and disk prompts work with the `bootc` kickstart command, the real ISO size and the bootloader landing on the target firmware need a spike. This ADR is updated to `accepted`, or superseded, when the spike finishes.

Build half (issue #28): `iso/` and `.github/workflows/iso.yml` implement the `bootc-generic-iso` route with the kickstart in `interactive-defaults.ks`, which is on a deprecation path. The workflow has not yet run, so the ISO size, the prompt flow and the bootloader path are still unmeasured and the status stays `proposed`.

`image-builder` runs from its container (`ghcr.io/osbuild/image-builder-cli`, digest-pinned and bumped by Renovate) so the build host needs only podman and root, and may be any distribution, such as the Ubuntu `deathstar` runner. This is also unproven until the first run.
