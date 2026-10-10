package update

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fake records the order of effects and answers JobRunning from a script;
// once the script is exhausted the last answer repeats.
type fake struct {
	calls      []string
	staged     bool
	running    []bool
	pollErr    error
	stageErr   error
	stopErr    error
	rebootErr  error
	runningIdx int
}

func (f *fake) Stage(context.Context) error {
	f.calls = append(f.calls, "stage")
	return f.stageErr
}

func (f *fake) Staged(context.Context) (bool, error) {
	f.calls = append(f.calls, "staged")
	return f.staged, nil
}

func (f *fake) JobRunning(context.Context) (bool, error) {
	f.calls = append(f.calls, "job")
	if f.pollErr != nil {
		return false, f.pollErr
	}
	i := min(f.runningIdx, len(f.running)-1)
	f.runningIdx++
	return f.running[i], nil
}

func (f *fake) StopRunner(context.Context) error {
	f.calls = append(f.calls, "stop")
	return f.stopErr
}

func (f *fake) Reboot(context.Context) error {
	f.calls = append(f.calls, "reboot")
	return f.rebootErr
}

func count(calls []string, name string) int {
	n := 0
	for _, c := range calls {
		if c == name {
			n++
		}
	}
	return n
}

var cfg = Config{PollInterval: time.Minute, MaxWait: 72 * time.Hour}

func noSleep(context.Context, time.Duration) error { return nil }

func TestRunNothingStagedDoesNotReboot(t *testing.T) {
	f := &fake{staged: false, running: []bool{false}}
	res, err := Run(context.Background(), f, noSleep, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res != NothingStaged || count(f.calls, "reboot") != 0 || count(f.calls, "job") != 0 {
		t.Fatalf("res=%v calls=%v", res, f.calls)
	}
}

func TestRunIdleRebootsAfterStoppingRunner(t *testing.T) {
	f := &fake{staged: true, running: []bool{false}}
	res, err := Run(context.Background(), f, noSleep, cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"stage", "staged", "job", "stop", "reboot"}
	if res != Rebooted || !equal(f.calls, want) {
		t.Fatalf("res=%v calls=%v want %v", res, f.calls, want)
	}
}

func TestRunWaitsForJobToFinish(t *testing.T) {
	f := &fake{staged: true, running: []bool{true, true, false}}
	res, err := Run(context.Background(), f, noSleep, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res != Rebooted || count(f.calls, "job") != 3 || count(f.calls, "reboot") != 1 {
		t.Fatalf("res=%v calls=%v", res, f.calls)
	}
}

func TestRunForcesAfterMaxWait(t *testing.T) {
	f := &fake{staged: true, running: []bool{true}}
	sleeps := 0
	sleep := func(context.Context, time.Duration) error { sleeps++; return nil }
	res, err := Run(context.Background(), f, sleep, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// 72h at one-minute polls: 4320 sleeps, 4321 checks, then force.
	if res != Forced || sleeps != 4320 || count(f.calls, "job") != 4321 {
		t.Fatalf("res=%v sleeps=%d jobs=%d", res, sleeps, count(f.calls, "job"))
	}
	if count(f.calls, "stop") != 1 || count(f.calls, "reboot") != 1 {
		t.Fatalf("calls tail=%v", f.calls[len(f.calls)-2:])
	}
}

func TestRunFailsClosedWhenJobStateUnknown(t *testing.T) {
	f := &fake{staged: true, running: []bool{false}, pollErr: errors.New("pgrep broke")}
	_, err := Run(context.Background(), f, noSleep, cfg)
	if err == nil || count(f.calls, "reboot") != 0 || count(f.calls, "stop") != 0 {
		t.Fatalf("err=%v calls=%v", err, f.calls)
	}
}

func TestRunDoesNotRebootIfRunnerWontStop(t *testing.T) {
	f := &fake{staged: true, running: []bool{false}, stopErr: errors.New("stop failed")}
	_, err := Run(context.Background(), f, noSleep, cfg)
	if err == nil || count(f.calls, "reboot") != 0 {
		t.Fatalf("err=%v calls=%v", err, f.calls)
	}
}

func TestRunStageErrorStops(t *testing.T) {
	f := &fake{stageErr: errors.New("no network"), running: []bool{false}}
	_, err := Run(context.Background(), f, noSleep, cfg)
	if err == nil || len(f.calls) != 1 {
		t.Fatalf("err=%v calls=%v", err, f.calls)
	}
}

func TestRunSleepCancellationAborts(t *testing.T) {
	f := &fake{staged: true, running: []bool{true}}
	cancelled := func(context.Context, time.Duration) error { return context.Canceled }
	_, err := Run(context.Background(), f, cancelled, cfg)
	if !errors.Is(err, context.Canceled) || count(f.calls, "reboot") != 0 {
		t.Fatalf("err=%v calls=%v", err, f.calls)
	}
}

func TestConfigValidate(t *testing.T) {
	bad := []Config{
		{},
		{PollInterval: time.Minute},
		{PollInterval: 0, MaxWait: time.Hour},
		{PollInterval: time.Hour, MaxWait: time.Minute},
	}
	for _, c := range bad {
		if c.validate() == nil {
			t.Errorf("validate(%+v) = nil, want error", c)
		}
	}
	if err := cfg.validate(); err != nil {
		t.Errorf("validate(default) = %v", err)
	}
}

func TestParseStaged(t *testing.T) {
	cases := []struct {
		name, in string
		want     bool
		wantErr  bool
	}{
		{"staged", `{"status":{"staged":{"image":{}},"booted":{}}}`, true, false},
		{"null", `{"status":{"staged":null,"booted":{}}}`, false, false},
		{"absent", `{"status":{"booted":{}}}`, false, false},
		{"garbage", `not json`, false, true},
		{"no status", `{}`, false, true},
	}
	for _, c := range cases {
		got, err := ParseStaged([]byte(c.in))
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("%s: got %v, %v", c.name, got, err)
		}
	}
}

func TestJobRunningFromExit(t *testing.T) {
	for code, want := range map[int]bool{0: true, 1: false} {
		got, err := JobRunningFromExit(code)
		if err != nil || got != want {
			t.Errorf("exit %d: got %v, %v", code, got, err)
		}
	}
	for _, code := range []int{2, 3, -1} {
		if _, err := JobRunningFromExit(code); err == nil {
			t.Errorf("exit %d: want error", code)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
