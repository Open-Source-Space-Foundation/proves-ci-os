# Ship a pinned runner seed, then let the runner update itself

- Status: accepted

The GitHub Actions runner mutates its own directory and updates itself by default, which conflicts with an immutable `/usr`. The Image ships a pinned, checksum-verified runner tarball as a seed in `/usr/lib/`, tracked by Renovate. First boot copies it into `~runner/actions-runner` on `/var`, never overwriting an existing `.runner` or `.credentials*`. After registration, the runner's self-update keeps it current.

## Considered Options

- **Pin and disable self-update** (`--disableupdate`): fully reproducible, but GitHub stops queuing jobs to runners more than 30 days stale, so the Image would need a monthly rebuild and every machine a monthly update.
- **Download at first boot**: no seed in the image, but first boot needs the network and an unpinned artifact.
- **Seed plus self-update**: chosen. No network needed at first boot, reproducible start, and staleness is the runner's problem.

## Consequences

- Good: the machine works from the first boot and does not depend on the update schedule to stay eligible for jobs.
- Bad: the running runner version differs from the seed, so the version in the Image is not the version in use. Docs say where to look.
- The runner lives at `~runner/actions-runner` because the core reference workflow reads `~/actions-runner/.env` by absolute path. Moving to an environment-variable path is a possible later change there.
