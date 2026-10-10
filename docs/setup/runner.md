# Runner

The GitHub Actions runner starts on boot but stays idle until you register it. The Image ships a pinned runner as a seed and copies it to `/var/lib/runner/actions-runner` on first boot; after registration the runner updates itself ([ADR 0004](../adr/0004-hybrid-runner-versioning.md)). Check `/var/lib/runner/actions-runner/bin/Runner.Listener --version` for the version in use, which can differ from the one in the Image.

All commands run on the machine over [Tailscale SSH](tailscale-ssh.md) as `ops`.

## 1. Get a registration token

An organization owner mints a token at *Organization settings → Actions → Runners → New self-hosted runner*. It expires after one hour. Treat it as a secret: it is never baked into the Image, and `proves-runner` accepts it only on stdin or from a file, never as an argument.

## 2. Register

```sh
sudo proves-runner register --labels integration
```

Paste the token and press Ctrl-D, or pass `--token-file <path>`. Flags:

| Flag | Meaning |
| --- | --- |
| `--labels` | Required. Comma-separated. Use `integration` for a [rig](#rigs). |
| `--name` | Runner name. Defaults to the hostname. |
| `--runnergroup` | Runner group. Defaults to the organization's default group. See [security](../security.md). |
| `--url` | Organization URL. Defaults to `https://github.com/Open-Source-Space-Foundation`. |
| `--token-file` | File holding the token. `-` (default) reads stdin. |
| `--replace` | Register again, replacing an existing runner of the same name. |

On success the runner service is enabled and started. Registering a machine that is already registered does nothing unless you pass `--replace`.

### Replace a registration

To re-register under the same name (a new token, a new group or new labels):

```sh
sudo proves-runner register --labels integration --replace
```

The old registration is set aside and restored if the new one fails. Additions you made to the runner's `.env` are kept.

## Unregister

With a *remove* token from the same page:

```sh
sudo proves-runner unregister
```

## Dead machine

If the machine is gone or cannot boot, you cannot unregister it. An organization owner removes it at *Organization settings → Actions → Runners → ⋯ → Remove runner*. Then [reinstall](install.md) and register again.

## Rigs

A **rig** is a CI machine with flight boards and a power supply attached. Register it with the `integration` label so hardware-in-the-loop jobs target it; a machine without boards must not have that label. The Image ships the udev rules (`/dev/ttyBOARD`, `/dev/ttyPWR`, `/dev/ttyDBG`), OpenOCD and `korad_control` at the paths the core reference workflow expects.

### `GDS_PASSTHROUGH`

Optional. The radio job uses `GDS_PASSTHROUGH` from the runner's `.env` as the LoRa passthrough tty, falling back to its own default when unset. To set it:

```sh
echo 'GDS_PASSTHROUGH=/dev/ttyACM2' | sudo -u runner tee -a /var/lib/runner/actions-runner/.env
sudo systemctl restart actions-runner
```

(`ops` has no write access to the runner directory; `sudo -u runner` is the supported route.)

## Troubleshooting

```sh
systemctl status actions-runner
journalctl -u actions-runner -e
```

`ops` can read these without extra privileges through the `systemd-journal` group.
