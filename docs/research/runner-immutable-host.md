# GitHub Actions runner on an immutable systemd (bootc) host

Resolves issue #5. Researched 2026-10-08 against `actions/runner` `main` (latest release v2.338.0) and docs.github.com. Source paths are in `actions/runner` unless noted; `src/...` links are to `https://github.com/actions/runner/blob/main/<path>`.

## Summary

- The runner is a self-contained directory (a tarball), not a package. It is not installed into `/usr`; it runs from a writable directory and mutates that directory (state files, `_work`, self-update).
- On bootc the whole runner directory must live in `/var` (e.g. `/var/lib/github-runner`), owned by an unprivileged user. The image carries only the unit, the user, the dependencies and a first-boot installer.
- Use a hand-written unit. The official `svc.sh` writes into `/etc/systemd/system` at runtime, requires root and is a poor fit for an image build.
- Self-update is on by default and rewrites the runner directory. Either let it (state in `/var` makes this safe) or disable it with `--disableupdate` and bump the version by shipping a new image.

## 1. Install layout

The release tarball (`actions-runner-linux-{x64,arm64,arm}-<ver>.tar.gz`, asset names from the v2.338.0 release) unpacks to a directory the runner treats as its "root". Files and directories involved (from `src/Runner.Common/HostContext.cs`, `Constants.cs`, `src/Misc/layoutbin/`):

| Path under root | Purpose | Mutable |
|---|---|---|
| `bin/`, `externals/` | listener binaries, bundled Node (`externals/node24`) | replaced on self-update |
| `config.sh`, `run.sh`, `env.sh`, `safe_sleep.sh` | entry scripts | replaced on self-update |
| `.runner` | registration settings (agent id, name, pool, URL, work folder, ephemeral/disableupdate) | written by `config.sh` |
| `.credentials`, `.credentials_rsaparams` | OAuth scheme and the runner's RSA private key | written by `config.sh` |
| `.service` | name of the installed unit (written only by `svc.sh`) | optional |
| `.path`, `.env` | PATH snapshot and env vars the runner applies to jobs | written by `config.sh`, editable |
| `_work/` | job workspaces, cached actions, `_update` staging | heavy writes |
| `_diag/` | runner and worker logs, self-update logs | grows |

`config.sh` refuses to run as root unless `RUNNER_ALLOW_RUNASROOT` is set (`src/Misc/layoutroot/config.sh`: "Must not run with sudo"). So registration must happen as the runner user. `config.sh` also snapshots the config user's environment into `.path`/`.env`.

## 2. Official `svc.sh` unit vs hand-written unit

`svc.sh` is generated from `src/Misc/layoutbin/systemd.svc.sh.template`. Behavior:

- Must run as root (`Must run as sudo`). `install [user]` renders `actions.runner.service.template` with `User`, `Description` and `RunnerRoot` (`pwd`), writes `/etc/systemd/system/<name>`, runs `restorecon` if SELinux is enforcing, `chmod 664`, `daemon-reload`, copies `bin/runsvc.sh` to `./runsvc.sh`, `systemctl enable`s it and writes `.service`.
- It fails if the unit file already exists (`error: exists ...`), so it is not idempotent.
- `GITHUB_ACTIONS_RUNNER_SERVICE_TEMPLATE` can point at a custom template. docs.github.com also names `actions-runner/bin/actions.runner.service.template` as the customization reference.

The stock template (`src/Misc/layoutbin/actions.runner.service.template`):

```ini
[Unit]
Description={{Description}}
After=network-online.target

[Service]
ExecStart={{RunnerRoot}}/runsvc.sh
User={{User}}
WorkingDirectory={{RunnerRoot}}
KillMode=process
KillSignal=SIGTERM
TimeoutStopSec=5min

[Install]
WantedBy=multi-user.target
```

`runsvc.sh` (`src/Misc/layoutbin/runsvc.sh`) loads `.path` into `PATH`, then runs `./externals/node24/bin/node ./bin/RunnerService.js`, and converts SIGTERM to SIGINT for graceful stop. `RunnerService.js` supervises `Runner.Listener` and restarts it on exit codes it treats as restartable (updating, retryable).

Recommendation: ship a hand-written unit in the image (`/usr/lib/systemd/system/`), modeled on the stock template, with these deviations:

- `User=` / `Group=` an unprivileged system user created via `sysusers.d`.
- `StateDirectory=github-runner` (bootc docs recommend `StateDirectory=` or `tmpfiles.d` for `/var`; image-shipped `/var` content is unpacked only on first install). Set `WorkingDirectory=/var/lib/github-runner`, `ExecStart=` to a script there.
- Keep `KillMode=process`, `KillSignal=SIGTERM`, `TimeoutStopSec=5min` (these let a running job finish on stop and are the upstream defaults).
- Keep `runsvc.sh` as the entry point or replicate its two jobs (PATH from `.path`, SIGTERM-to-SIGINT). Upstream says custom templates must still go through `runsvc.sh`.
- Add `Restart=always` plus `RestartSec` and `StartLimit*`; upstream's template has no restart policy.
- Order after `network-online.target` and add `Wants=network-online.target` (upstream only has `After=`).
- Gate on `ConditionPathExists=/var/lib/github-runner/.runner` so an unregistered host does not crash-loop; registration is a separate step (section 6).

Image-time `svc.sh install` is not viable: it needs the runner directory present at build and writes a unit whose `RunnerRoot` would be wrong or in `/var`.

## 3. Self-update

Mechanism (`src/Runner.Listener/Runner.cs`, `SelfUpdater.cs`, `src/Misc/layoutbin/update.sh.template`):

1. The service sends the listener a refresh message with a target version.
2. The listener downloads the tarball to `_work/_update`, verifies a hash, extracts, and copies `bin` and `externals` into the root as `bin.<ver>` and `externals.<ver>` (up to `RunnerDownloadRetryMaxAttempts = 3` attempts).
3. The listener exits with code 3 (`RunnerUpdating`, or 4 for run-once). The service wrapper runs `update.sh`, which repoints the `bin` and `externals` symlinks to the new versions. Older version directories are pruned, keeping the current and latest.
4. The listener restarts. `.runner`, `.credentials*` and `_work` are untouched.

Consequences on an immutable host:

- The runner root must be writable by the runner user. It cannot sit under `/usr` or `/opt` baked into the image. It must be in `/var`.
- Updated binaries persist in `/var` and survive reboot, but an image update that re-seeds nothing leaves the old runner in `/var`. Updating the image does not update the runner, and vice versa. Decide who owns the runner version.
- Policy from docs.github.com ("Self-hosted runners reference"): "You will be required to update your runner version within 30 days of a new version being made available. If you do not perform a software update within 30 days, the GitHub Actions service will not queue jobs to your runner." This applies to runners with updates disabled too. Pinning means rebuilding the image at least monthly.

Pin / disable:

- `config.sh --disableupdate` ("Disable self-hosted runner automatic update to the latest released version", `ConfigurationManager.cs`/`Runner.cs` help text). It is stored as `DisableUpdate` on the agent registration (server-side) and in `.runner`, so it is fixed at registration time. Changing it requires re-registration (`--replace`). With JIT config the flag is not a parameter (see section 5).
- The server may still refuse jobs for outdated runners (30-day rule). `ACTIONS_RUNNER_RETURN_VERSION_DEPRECATED_EXIT_CODE=1` makes `run.sh` exit with code 7 when the version is deprecated (`run.sh`, `Constants.cs`), a hook for failing loudly.

Two viable strategies:

- A (recommended starting point, to confirm in the design issue): keep self-update on, runner in `/var`, image only supplies the first-boot seed. Least ops work; runner stays current without image rebuilds.
- B: `--disableupdate`, bake the runner tarball into `/usr/lib/github-runner/` in the image (pinned by version and checksum), copy to `/var` on first boot or on version mismatch. Reproducible and auditable, but requires a monthly image rebuild and the copy logic must be idempotent and not clobber `.runner`/`.credentials`.

## 4. Fedora dependencies (.NET)

`src/Misc/layoutbin/installdependencies.sh` and `docs/start/envlinux.md` list for Fedora (`/etc/fedora-release` branch uses `dnf`):

```
dnf install -y lttng-ust openssl-libs krb5-libs zlib libicu
```

Notes:

- The script requires root and installs at runtime. In an image build, install these packages directly in the Containerfile and skip the script. `config.sh` verifies with `ldd ./bin/libcoreclr.so` etc. and fails with "Dependencies is missing" if any are absent, so `ldd`/`glibc-common` must also exist.
- The runner bundles its own .NET runtime in `bin/` (self-contained) and its own Node in `externals/`; no system .NET SDK is needed.
- Fedora is listed as supported from 29+ (docs.github.com reference). Fedora version currently used by the image is within range; support is "best effort" in practice as the platform list is not an upstream-tested matrix.
- Jobs themselves need whatever tools the workflows call (git, curl, tar, etc.), which is an image-content decision outside this question. Also, `run.sh` can call `update-ca-trust` with sudo when `RUNNER_UPDATE_CA_CERTS` is set; avoid that on an immutable host (bake CA trust into the image).
- The runner's `sudo` use in `run.sh` and the `installdependencies.sh` path imply the unprivileged user needs no sudo for normal operation.

## 5. aarch64

The v2.338.0 release has `actions-runner-linux-arm64-2.338.0.tar.gz` alongside `x64` and `arm` (32-bit). docs.github.com lists ARM64 for Linux, macOS and Windows with the qualifier "(currently in public preview)" and ARM32 for Linux. Treat aarch64 as supported but labeled preview. Key points for the image build:

- Select tarball by arch (`x64` for x86_64, `arm64` for aarch64) and verify the checksum published in the release notes.
- Default labels for a Linux ARM64 runner are `self-hosted`, `Linux`, `ARM64` (labels are set from `VarUtil.OS` and `VarUtil.OSArchitecture` in `ConfigurationManager.cs`); x86_64 gets `X64`. Workflows can distinguish architectures without custom labels.
- The runner repo has flags (`actions_runner_deprecate_linux_arm32`, `actions_runner_kill_linux_arm32` in `Constants.cs`) hinting ARM32 is being retired; arm64 has no such flag.

## 6. Registration token and JIT config APIs (org level)

From docs.github.com REST "Self-hosted runners" (org endpoints). All require an `admin:org` classic PAT or equivalent app/fine-grained permission (organization self-hosted runners: read/write); `repo` scope additionally for private repositories.

| Purpose | Endpoint | Notes |
|---|---|---|
| Registration token | `POST /orgs/{org}/actions/runners/registration-token` | Response `token`, `expires_at`; "expires after one hour". Used with `config.sh --url https://github.com/<org> --token <t>`. |
| Remove token | `POST /orgs/{org}/actions/runners/remove-token` | One hour. Used with `config.sh remove --token <t>`. |
| JIT config | `POST /orgs/{org}/actions/runners/generate-jitconfig` | Body: `name` (req), `runner_group_id` (req), `labels` (req, 1-100 items), `work_folder` (opt, default `_work`). Response: `runner` object and `encoded_jit_config`. |
| List / delete | `GET /orgs/{org}/actions/runners`, `DELETE /orgs/{org}/actions/runners/{runner_id}` | Delete returns 204. |
| Labels | `GET/POST/PUT/DELETE /orgs/{org}/actions/runners/{runner_id}/labels` | Read, add, set, remove. |

Runner-side:

- `config.sh` accepts `--token`, or `--pat` (the runner itself calls `.../orgs/{org}/actions/runners/{registration|remove}-token`; see `GetJITRunnerTokenAsync`). Avoid `--pat` on hosts: the long-lived secret would need to be on the machine. Prefer fetching a registration token elsewhere and delivering it at first boot.
- JIT: `./run.sh --jitconfig <encoded_jit_config>` (`Constants.cs` `Args.JitConfig`; `Runner.cs` base64-decodes JSON and applies it, no `config.sh` and no `.runner`/`.credentials` written by registration). JIT runners are ephemeral by design: docs.github.com says "you can create ephemeral, just-in-time runners using the REST API" and the service de-registers the runner after one job.
- Secrets handling: `token`, `pat`, `jitconfig` and `windowslogonpassword` are in the runner's `Secrets` list so they are masked in logs (`Constants.cs`).

For a persistent runner, only the registration-token path applies. JIT is out of scope unless the project moves to ephemeral runners.

## 7. Org-level runner groups and labels

- `config.sh --runnergroup <name>`; default group "Default" if omitted (`ConfigurationManager.cs` `GetRunnerGroupName(defaultPool?.Name)`). Group must exist before registration. Group visibility (which repos can use it) is an org setting, not a runner setting.
- `--labels a,b,c` adds custom labels on top of the defaults (`self-hosted`, OS, arch). `--no-default-labels` removes the defaults but is rejected without `--labels` ("Disabling default labels ... without specifying --labels is not supported").
- Labels set at registration; with `--replace` re-registering the same name replaces them (`UpdateExistingAgent` clears and resets the label list). Labels can also be changed afterwards through the REST labels endpoints without touching the host.
- Unattended registration needs `--unattended` plus `--url`, `--token`, `--name`, `--runnergroup`, `--labels`, optionally `--replace`, `--work`.
- Only org owners can register org-level runners (docs.github.com "Adding self-hosted runners").
- Runner groups for organization runners may be restricted by plan (Enterprise features such as runner group access control for private repos); not verified here against docs, check before relying on it.

## 8. Persistent vs ephemeral implications

- GitHub: "GitHub recommends implementing autoscaling with ephemeral self-hosted runners; autoscaling with persistent self-hosted runners is not recommended." Persistent runners are fine for fixed hosts.
- Persistent runner on this OS:
  - Job state accumulates in `_work` and (outside the runner) in the home directory, caches, container storage. Needs a cleanup policy (disk). Workflows can leave state that influences later jobs.
  - Registration survives reboots and OS upgrades because `.runner`/`.credentials` are in `/var`. A rollback of the image does not roll back them.
  - Identity: the runner keeps a server-side registration. If `/var` is lost or the machine is cloned, two hosts may share one registration (session conflict, exit code 5 `SessionConflict`); re-register with `--replace` on a new name.
  - Reprovisioning needs a fresh registration token (valid 1 hour) at first boot, or the runner has to be removed (`remove-token` or REST delete) to avoid orphaned offline entries.
- Ephemeral (`config.sh --ephemeral`, or JIT): one job then exit and de-register; wipe `_work` and re-register for each job. Not the target here, but the image should not preclude it.

## 9. State that must live in `/var`

bootc facts (`bootc-dev/bootc` `docs/src/bootc-filesystem.7.md`): `/usr` is read-only with composefs; `/etc` is persistent with a 3-way merge on upgrades; `/var` "persists by default" and is shared across deployments, and content shipped in `/var` in the image "is unpacked only from the initial image - subsequent changes ... are not automatically applied." `tmpfiles.d` or `StateDirectory=` is the recommended way to create `/var` directories.

Therefore, in `/var/lib/github-runner` (runner root, all owned by the runner user):

- `.runner`, `.credentials`, `.credentials_rsaparams` (registration identity; secrets, mode 0600/0700 directory),
- `.env`, `.path` (environment snapshot; reproducible from the image if you generate them at first boot instead),
- `bin.<ver>/`, `externals.<ver>/` and the `bin`/`externals` symlinks (when self-update is on),
- `_work/` (large, churny), `_diag/` (logs),
- the runner user's `$HOME` (`~/.cache`, `~/.docker`, etc.) if jobs write there; give the user a home under `/var/lib` or `/var/home`.

Belongs in the image (`/usr`, or `/etc` for defaults): the unit and `sysusers.d`/`tmpfiles.d` snippets, Fedora dependency packages, CA trust, optional pinned tarball for strategy B, the first-boot registration script. The registration token itself must be supplied at runtime (cloud-init/ignition/credential file in `/etc` or `/run`), never baked in.

## Open points for later issues

- Strategy A vs B for versions (section 3).
- How the registration token is delivered at first boot and who mints it (REST call with `admin:org`, 1-hour validity).
- Cleanup policy for `_work` and container/storage on a persistent runner.
- Confirm runner-group and `--ephemeral`/`--disableupdate` behavior on the target GitHub plan.
- Unverified claims: ARM64 "public preview" status is as worded on the docs page at the time of reading; the runner's restart-on-exit-code logic in `RunnerService.js` was inferred from exit code constants, not read line by line.

## Sources

- https://github.com/actions/runner/tree/main/src/Misc/layoutbin (`systemd.svc.sh.template`, `actions.runner.service.template`, `runsvc.sh`, `update.sh.template`, `installdependencies.sh`)
- https://github.com/actions/runner/blob/main/src/Misc/layoutroot/config.sh, `run.sh`
- https://github.com/actions/runner/blob/main/src/Runner.Listener/Runner.cs, `SelfUpdater.cs`, `Configuration/ConfigurationManager.cs`
- https://github.com/actions/runner/blob/main/src/Runner.Common/Constants.cs, `HostContext.cs`
- https://github.com/actions/runner/blob/main/docs/start/envlinux.md
- https://github.com/actions/runner/releases/tag/v2.338.0
- https://docs.github.com/en/actions/reference/runners/self-hosted-runners
- https://docs.github.com/en/rest/actions/self-hosted-runners
- https://docs.github.com/en/actions/how-tos/manage-runners/self-hosted-runners/add-runners
- https://docs.github.com/en/actions/how-tos/manage-runners/self-hosted-runners/configure-the-application
- https://github.com/bootc-dev/bootc/blob/main/docs/src/bootc-filesystem.7.md
