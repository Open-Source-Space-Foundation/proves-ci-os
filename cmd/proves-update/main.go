// Command proves-update stages the latest Image with `bootc upgrade` and
// reboots into it once no CI job is running (ADR 0006). The weekly timer and
// `sudo proves-update` run the same path.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/open-source-space-foundation/proves-ci-os/internal/update"
)

// runnerUnit is the Runner service in files/usr/lib/systemd/system.
const runnerUnit = "actions-runner.service"

// Upper bounds on a single command, so a hung bootc cannot hold the timer
// service forever. The stop bound covers the unit's own five-minute timeout.
const (
	upgradeTimeout = 30 * time.Minute
	quickTimeout   = 10 * time.Minute
)

type host struct{}

// run executes name with args (never through a shell) and returns stdout and
// the exit code (-1 when the command did not run to completion).
func run(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return stdout.Bytes(), exit.ExitCode(), err
	}
	return stdout.Bytes(), -1, err // could not start, or killed by the timeout
}

func (host) Stage(ctx context.Context) error {
	_, _, err := run(ctx, upgradeTimeout, "bootc", "upgrade")
	return err
}

func (host) Staged(ctx context.Context) (bool, error) {
	out, _, err := run(ctx, quickTimeout, "bootc", "status", "--json")
	if err != nil {
		return false, err
	}
	return update.ParseStaged(out)
}

func (host) JobRunning(ctx context.Context) (bool, error) {
	_, code, err := run(ctx, quickTimeout, "pgrep", "-x", "Runner.Worker")
	if err != nil && code < 0 {
		return false, err
	}
	return update.JobRunningFromExit(code)
}

func (host) StopRunner(ctx context.Context) error {
	_, _, err := run(ctx, quickTimeout, "systemctl", "stop", runnerUnit)
	return err
}

func (host) Reboot(ctx context.Context) error {
	_, _, err := run(ctx, quickTimeout, "systemctl", "reboot")
	return err
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := realMain(log); err != nil {
		log.Error("update failed", "err", err)
		os.Exit(1)
	}
}

func realMain(log *slog.Logger) error {
	if len(os.Args) > 1 {
		return fmt.Errorf("usage: proves-update (takes no arguments)")
	}
	if os.Geteuid() != 0 {
		return errors.New("must run as root: use sudo proves-update")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	res, err := update.Run(ctx, host{}, sleep, update.DefaultConfig)
	if err != nil {
		return err
	}
	switch res {
	case update.NothingStaged:
		log.Info("no staged update, nothing to do")
	case update.Rebooted:
		log.Info("runner idle, rebooting into staged update")
	case update.Forced:
		log.Warn("max wait elapsed with a job running, forced reboot", "max_wait", update.DefaultConfig.MaxWait)
	}
	return nil
}
