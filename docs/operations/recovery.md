# Recovery

## Locked out, or the tailnet is unreachable

The console is the way in. Log in at the physical keyboard as `ops` with the **console password** set at [install](../setup/install.md). It works only at the console and for `sudo`; it does not work over ssh, because `sshd` is disabled.

From the console you can rejoin the tailnet:

```sh
sudo tailscale up --ssh
```

## Forgotten console password

There is no remote reset. [Reinstall](../setup/install.md), then [register the runner](../setup/runner.md) again. Remove the old registration first if the machine is still in the runner list ([dead machine](../setup/runner.md#dead-machine)).

## Bad update

```sh
sudo bootc rollback
sudo systemctl reboot
```

If the machine does not boot far enough to log in, choose the previous deployment in the boot menu, then run the commands above.

## Runner will not start

```sh
systemctl status actions-runner
journalctl -u actions-runner -e
```

A runner that was never registered is skipped by design. Register it: [runner](../setup/runner.md).

## Machine will not update

```sh
journalctl -u proves-update -e
sudo bootc status
```

If verification fails, the image on the registry is not signed by the key baked into this machine. Do not work around it: ask the maintainers, because a spoofed image would have your CI secrets. See [security](../security.md).

## Nothing works

Reinstall from the ISO. The machine holds no state worth keeping other than the runner registration, which you recreate with a new token.
