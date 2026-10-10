# Does keyless cosign verification work in the baked bootc policy?

Resolves issue #16 (part of map #1, blocks #11). Researched 2026-10-09.

## Answer

**Not in a form that pins the repo's workflow. Plan on the key-pair fallback.**

`containers/image` `policy.json` does have a keyless mode (`sigstoreSigned` with `fulcio` and a mandatory Rekor public key), and the check itself runs offline. But the `fulcio` block can only pin the signer by `subjectEmail` (a certificate email SAN) plus `oidcIssuer`. A GitHub Actions keyless certificate identifies the workflow with a URI SAN, not an email. The upstream code does not match URI SANs, and `fulcio` has no field for one. So a policy cannot say "only signatures from workflow X in repo Y". Every field the policy accepts is either unsatisfiable by a GitHub Actions certificate or does not pin the repo.

## Findings

### 1. `sigstoreSigned` keyless config (containers/image)

Source: [containers-policy.json(5)](https://github.com/containers/image/blob/main/docs/containers-policy.json.5.md).

- Exactly one of `keyPath`, `keyPaths`, `keyData`, `keyDatas`, `fulcio`, `pki` must be present.
- `fulcio` takes `caPath` or `caData` (the Fulcio public key), `oidcIssuer` (mandatory) and `subjectEmail` (mandatory). No other identity field is documented.
- A Rekor public key (`rekorPublicKeyPath`, `rekorPublicKeyPaths`, `rekorPublicKeyData`, `rekorPublicKeyDatas`; at most one form) is "mandatory if `fulcio` is specified". Signatures must then carry a Rekor signed entry timestamp.
- Cosign signatures carry only a repository, so `signedIdentity` is limited to `matchRepository` / `exactRepository`. The man page notes this does not protect against swapping a signed image for an unexpected tag.

Config validation, source: [`signature/policy_config_sigstore.go`](https://github.com/containers/image/blob/main/signature/policy_config_sigstore.go). The `fulcio` constructor requires `caPath`/`caData` (not both), `oidcIssuer` and `subjectEmail`. No URI-style identity field exists.

### 2. The identity gap (the blocker)

Source: [`signature/fulcio_cert.go`](https://github.com/containers/image/blob/main/signature/fulcio_cert.go).

- The certificate check is `slices.Contains(cert.EmailAddresses, f.subjectEmail)`. Only email SANs are checked.
- A code comment says (paraphrased) that cosign also matches URI SANs and GitHub workflow values, which Fulcio can issue, and that containers/image does not yet. That is an acknowledged FIXME.
- The OIDC issuer extension is checked and must equal `oidcIssuer`.

For GitHub Actions the Fulcio certificate carries the workflow as a URI SAN, with the issuer extension set to `https://token.actions.githubusercontent.com`. That the SAN is a URI rather than an email is consistent with the cosign docs example identity `https://github.com/<org>/<repo>/.github/workflows/<file>@<ref>` (secondary source: [Sigstore verify docs](https://docs.sigstore.dev/cosign/verifying/verify/) show `--certificate-identity`/`--certificate-oidc-issuer` but only with an email example). **Unverified:** I did not inspect a real GitHub-issued certificate to confirm there is no email SAN. A GitHub Actions identity has no email, so a `subjectEmail` match is not expected to succeed.

Pinning only `oidcIssuer` is not an option, because `subjectEmail` is mandatory and no wildcard or regexp is documented. Even if some email could be matched, it would not bind to this repo or workflow.

**Unverified:** whether a newer containers/image release than `main` as read on 2026-10-09, or the version shipped in `fedora-bootc:44`, adds URI/regexp subject matching. The `main` branch does not. Check the shipped `containers-common`/`skopeo`/`bootc` versions in the image before relying on this.

### 3. Offline verification

Source: [`signature/internal/rekor_set.go`](https://github.com/containers/image/blob/main/signature/internal/rekor_set.go). Rekor SET verification canonicalizes the payload, hashes it and checks an ECDSA signature against the supplied Rekor public key, using only local data. The Fulcio CA and Rekor key come from the policy (path or inline data), so no network call to Fulcio, Rekor or TUF is needed at verify time. Fetching the signature artifact from the registry still needs the network. The SCT check is deliberately omitted because Rekor SETs contain the full certificate (per code comments).

Consequence: trust roots must be baked into the image and rotated by shipping a new image. Sigstore public-good roots rotate via TUF; **unverified:** how often, and what that means for a long-lived baked-in Fulcio CA / Rekor key.

### 4. `registries.d`

Source: [containers-registries.d(5)](https://github.com/containers/image/blob/main/docs/containers-registries.d.5.md). `use-sigstore-attachments: true` is required, per registry scope (`docker:` section), for sigstore signatures to be read. The default is off ("Sigstore attachments are not read or written"). This applies to the key-pair path too. The `policy.json` man page repeats that registry-hosted images need this option. The legacy `lookaside` URL is for the older GPG signature store and is not needed for sigstore attachments.

### 5. Signature format compatibility

- containers/image reads cosign "simple signing" layers. Source: [`internal/signature/sigstore.go`](https://github.com/containers/image/blob/main/internal/signature/sigstore.go) defines media type `application/vnd.dev.cosign.simplesigning.v1+json` and annotations `dev.cosignproject.cosign/signature`, `dev.sigstore.cosign/certificate`, `dev.sigstore.cosign/chain`, `dev.sigstore.cosign/bundle` (the Rekor SET). It also defines the payload type `cosign container image signature` (`signature/internal/sigstore_payload.go`).
- Storage is the legacy cosign form: a signature image tagged by the signed manifest digest. **Unverified:** the exact tag form (`sha256-<digest>.sig`); I did not find it stated in the containers/image docs I read.
- Cosign v3 reportedly defaults to the Sigstore protobuf bundle format and OCI 1.1 referrers. Source for this is secondary only (blog and search snippets), not the [cosign docs](https://docs.sigstore.dev/cosign/signing/overview/) fetched. **Unverified:** that containers/image reads signatures in that form. Nothing I read shows support for the new bundle or referrers layout. **Action for #11:** if the signing job uses cosign v3, find the flags that force legacy simple-signing/`.sig` output (my attempt to confirm them from `cosign_sign.md` found no such flags), and test a `skopeo`/`bootc` pull against a real signature before committing to any design. Reproducible test: sign a throwaway image in GHCR and run `skopeo copy` with the candidate `policy.json`.

### 6. bootc enforcement

Sources: [bootc-switch(8)](https://github.com/bootc-dev/bootc/blob/main/docs/src/man/bootc-switch.8.md), [bootc-install-config(5)](https://www.mankier.com/5/bootc-install-config) (mirror of the bootc man page; the upstream page was not fetched).

- bootc relies on the system `containers-policy.json`. `--enforce-container-sigpolicy` (and the install-config key `enforce-container-sigpolicy`, default `false`) rejects pulls when the default policy is `insecureAcceptAnything`. `--target-no-signature-verification` is a deprecated no-op.
- So a baked policy is honored only if the default entry is not `insecureAcceptAnything`; a restrictive default (`reject`) plus a scoped `sigstoreSigned` entry for the GHCR repo achieves the same without the flag.
- **Unverified:** whether `bootc upgrade` (as opposed to `switch`, which has the flag) enforces the policy by default on `fedora-bootc:44`. The `bootc-upgrade(8)` man page has no signature text.
- **Unverified:** that a policy.json under `/etc/containers/` in the image is the file bootc consults on the running host. The bootc docs I reached (bootc.dev front page, Fedora bootc docs) were thin or behind an anti-bot page; the Fedora page returned no content.

### 7. GitHub Actions OIDC values

Source: [GitHub OIDC concepts](https://docs.github.com/en/actions/concepts/security/openid-connect).

- Issuer: `https://token.actions.githubusercontent.com`.
- Tokens carry `job_workflow_ref` (e.g. `octo-org/octo-automation/.github/workflows/oidc.yml@refs/heads/main`), `workflow`, `ref`, `run_id`, etc. The `sub` claim format varies by repo creation date; newer repos use an immutable default subject including owner and repo IDs.
- These values are what a cosign CLI verification would pin (`--certificate-oidc-issuer` plus `--certificate-identity` or a regexp). containers/image cannot consume them (section 2).
- **Unverified:** the exact Fulcio SAN string for this repo's workflow. Derive it empirically from a real certificate (`cosign verify` output) rather than constructing it by hand.

## Fallback: key-pair policy requirements

A keyed `sigstoreSigned` entry works with the pieces above:

1. Generate a cosign key pair (`cosign generate-key-pair`). Store the private key and password as GitHub Actions secrets (or use a KMS); sign in CI with `cosign sign --key`. Never commit the private key.
2. Bake the public key into the image (e.g. `/etc/pki/containers/<name>.pub`). Reference it from `policy.json` with `keyPath` (or `keyPaths` to allow rotation with overlap).
3. `policy.json`: restrictive default (`reject`), plus under `transports.docker` a scope for `ghcr.io/<org>/<image>` with `{"type":"sigstoreSigned","keyPath":"...","signedIdentity":{"type":"matchRepository"}}`. `matchRepository` is the strongest identity cosign signatures allow.
4. `/etc/containers/registries.d/<name>.yaml`: `docker: ghcr.io/<org>/<image>: use-sigstore-attachments: true`.
5. Key rotation means shipping an image with the new public key before signing with the new private key; `keyPaths` lets old and new coexist. Bootstrapping risk: a host can only learn a new key from an image it already trusts.
6. Optionally set `enforce-container-sigpolicy = true` via `bootc install` config as a guard against a policy regressing to `insecureAcceptAnything`.

A key-pair signature gives weaker provenance than keyless (the key, not the workflow, is the identity), and the signing key becomes a high-value secret.

## Open items for downstream work (#11)

- Confirm the shipped container tooling versions in `fedora-bootc:44` and whether they support URI/regexp Fulcio subject matching (section 2). If a future release does, keyless becomes viable and this note should be revisited.
- Verify end to end with a real signed GHCR image: signature format produced by the chosen cosign version, tag/referrer layout, and a successful `skopeo`/`bootc switch` against the baked policy.
- Confirm bootc `upgrade` enforcement and policy file location on the booted host.
- Out of scope here: attestations/SBOM verification and TUF root rotation policy.
