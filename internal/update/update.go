// Package update stages a bootc image and reboots into it only when no CI job
// is running, forcing the reboot after a fixed wait (ADR 0006).
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// System is the set of effects Run needs. Defined here, at the consumer.
type System interface {
	// Stage runs `bootc upgrade` without applying it. Safe to repeat.
	Stage(ctx context.Context) error
	// Staged reports whether a new deployment is waiting for a reboot.
	Staged(ctx context.Context) (bool, error)
	// JobRunning reports whether a Runner.Worker process exists.
	JobRunning(ctx context.Context) (bool, error)
	// StopRunner stops the Runner service, bounded by the unit's stop timeout.
	StopRunner(ctx context.Context) error
	Reboot(ctx context.Context) error
}

// Config bounds the idle wait. The poll count is MaxWait / PollInterval.
type Config struct {
	PollInterval time.Duration
	MaxWait      time.Duration
}

// DefaultConfig is ADR 0006: poll every minute for at most 72 hours.
var DefaultConfig = Config{PollInterval: time.Minute, MaxWait: 72 * time.Hour}

func (c Config) validate() error {
	if c.PollInterval <= 0 || c.MaxWait < c.PollInterval {
		return fmt.Errorf("invalid config %+v: need 0 < PollInterval <= MaxWait", c)
	}
	return nil
}

// Result says what Run did.
type Result int

const (
	NothingStaged Result = iota // no new image, no reboot
	Rebooted                    // the Runner was idle
	Forced                      // MaxWait elapsed with a job still running
)

// Run stages an update and, if one is waiting, reboots once the Runner is
// idle or MaxWait has passed. If it cannot tell whether a job is running, or
// cannot stop the Runner, it does not reboot. Running it twice is safe: with
// nothing staged it does nothing. sleep is injected so tests do not wait.
func Run(ctx context.Context, sys System, sleep func(context.Context, time.Duration) error, cfg Config) (Result, error) {
	if err := cfg.validate(); err != nil {
		return NothingStaged, err
	}
	if err := sys.Stage(ctx); err != nil {
		return NothingStaged, fmt.Errorf("stage update: %w", err)
	}
	staged, err := sys.Staged(ctx)
	if err != nil {
		return NothingStaged, fmt.Errorf("check staged update: %w", err)
	}
	if !staged {
		return NothingStaged, nil
	}
	forced, err := waitForIdle(ctx, sys, sleep, cfg)
	if err != nil {
		return NothingStaged, err
	}
	if err := sys.StopRunner(ctx); err != nil {
		return NothingStaged, fmt.Errorf("stop runner: %w", err)
	}
	if err := sys.Reboot(ctx); err != nil {
		return NothingStaged, fmt.Errorf("reboot: %w", err)
	}
	if forced {
		return Forced, nil
	}
	return Rebooted, nil
}

// waitForIdle polls at most MaxWait/PollInterval+1 times. It returns true when
// the wait ran out with a job still running.
func waitForIdle(ctx context.Context, sys System, sleep func(context.Context, time.Duration) error, cfg Config) (bool, error) {
	maxPolls := int(cfg.MaxWait / cfg.PollInterval)
	for i := 0; i <= maxPolls; i++ {
		running, err := sys.JobRunning(ctx)
		if err != nil {
			return false, fmt.Errorf("check for running job: %w", err)
		}
		if !running {
			return false, nil
		}
		if i == maxPolls {
			break
		}
		if err := sleep(ctx, cfg.PollInterval); err != nil {
			return false, fmt.Errorf("wait for idle: %w", err)
		}
	}
	return true, nil
}

// ParseStaged reads `bootc status --json`. A missing or null status.staged
// means nothing is staged; a document without status is an error.
func ParseStaged(statusJSON []byte) (bool, error) {
	var doc struct {
		Status *struct {
			Staged json.RawMessage `json:"staged"`
		} `json:"status"`
	}
	if err := json.Unmarshal(statusJSON, &doc); err != nil {
		return false, fmt.Errorf("parse bootc status: %w", err)
	}
	if doc.Status == nil {
		return false, errors.New("bootc status has no status field")
	}
	staged := doc.Status.Staged
	return len(staged) > 0 && string(staged) != "null", nil
}

// JobRunningFromExit maps pgrep's exit code: 0 matched, 1 no match, anything
// else (usage error, no permission) is an error, never "no job".
func JobRunningFromExit(code int) (bool, error) {
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("pgrep exited %d", code)
	}
}
