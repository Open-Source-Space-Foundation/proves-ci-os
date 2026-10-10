# Stage image updates automatically and reboot only when the runner is idle

- Status: accepted

Fedora's stock `bootc-fetch-apply-updates.timer` reboots without regard for a running job, and a mid-job reboot fails someone's CI run. A timer runs `bootc upgrade` (stage only). A `proves-update` script then reboots only when no job is running: it stops the Runner (bounded by the unit's five-minute stop timeout) and reboots.

## Decision

- Weekly, Monday 09:00 UTC, with a randomized delay, shortly after the weekly image build (see the CI pipeline).
- If a job is running, poll every minute for at most **72 hours**, then **force** the update. Every loop has this numeric bound.
- `sudo proves-update` runs the same path on demand.
- Machines follow `:latest`, built from `main` after CI passes. There is no staging channel in v1.
- Rollback is manual: `sudo bootc rollback` and a reboot, documented, with the console and Tailscale SSH as the way in.

## Considered Options

- **Stock auto-reboot timer**: simplest, but fails jobs.
- **Manual only**: machines drift past GitHub's 30-day staleness limit for runners.
- **Greenboot automatic rollback**: deferred to a separate issue; a bad image that breaks networking needs someone at the console either way.

## Consequences

- Good: jobs are not killed by routine updates, and a missed window costs a week because the update is already staged.
- Bad: after 72 hours of a busy runner, a job can still be killed. The bound is a deliberate choice.
- Bad: with no staging channel, a bad `main` reaches every machine. CI structure tests and the verify-before-`latest` step in [ADR 0003](0003-verified-signed-updates.md) are the gates.
