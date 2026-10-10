# Sign the Image with a cosign key pair and verify it on every machine

- Status: accepted

The Image is on a public registry (`ghcr.io/open-source-space-foundation/proves-ci-os`) and the Runner holds CI secrets, so a CI machine must boot only images this project built. CI signs the image digest with a cosign **key pair**. The public key and a containers-image `policy.json` that requires a valid signature for our repository are baked into the Image, so every CI machine enforces it on `bootc upgrade`, and the next image must carry a signature from the same key.

Derived from [stableOS ADR 0003](https://github.com/nateinaction/stableOS/blob/main/docs/adrs/0003-verified-signed-updates.md).

## Considered Options

- **Key pair, enforced by a baked policy**: verifies offline, no dependency on OIDC, Fulcio or Rekor being reachable. Chosen.
- **Keyless (GitHub OIDC)**: no key to hold, but the containers/image `fulcio` policy matches only `oidcIssuer` and `subjectEmail`, not the URI SAN that identifies a GitHub Actions workflow, so it cannot pin our workflow. Ruled out by research.
- **No signing**: registry access control and TLS only. A spoofed image would boot with Runner secrets.

## Decision

- Sign by digest in a GitHub `environment` named `release` restricted to `main`, which holds the private key (password-protected, password as a separate secret). PR jobs and forks cannot read it.
- Sign with `cosign sign --new-bundle-format=false --use-signing-config=false --key env://…`. cosign v3 defaults (new bundle format, OCI referrers) are not supported by containers/image yet.
- Publish by `sha` tag, sign, then verify in a clean container that has only the baked `policy.json`, the `registries.d` entry and the public key (`skopeo copy`). Only then move `latest`. A failed sign or verify leaves `latest` on the last good image.
- The policy is otherwise permissive so base-image and other pulls keep working.

## Consequences

- Good: a spoofed or tampered image is rejected at upgrade time, offline, and users can check with `cosign verify --key cosign.pub`.
- Bad: the private key is a long-lived secret. Loss or compromise is serious, and rotation needs an image carrying the new key (trusting both during the change) on every machine. Rotation is documented in `docs/operations/updates.md`.
- Bad: if signing breaks, machines cannot take updates. The verify step before `latest` moves exists to prevent this.
- Open: confirm once that a real `bootc upgrade` behaves like `skopeo` with this policy.
