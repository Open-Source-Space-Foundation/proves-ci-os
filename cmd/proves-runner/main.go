// Command proves-runner seeds, registers and unregisters the GitHub Actions
// runner on a CI machine (ADR 0004).
//
//	proves-runner seed
//	proves-runner register --labels L [--name N] [--runnergroup G] [--url U] [--token-file F] [--replace]
//	proves-runner unregister [--token-file F]
//
// Tokens are read from stdin or a file, never from arguments.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
	"time"
)

const (
	seedTarball = "/usr/lib/proves-ci-os/actions-runner.tar.gz"
	// Bounds on external processes: config.sh talks to GitHub, systemctl is local.
	configTimeout  = 5 * time.Minute
	systemdTimeout = time.Minute
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "proves-runner:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: proves-runner seed|register|unregister [flags]")
	}
	switch args[0] {
	case "seed":
		return seedRunner(func() (io.ReadCloser, error) { return os.Open(seedTarball) }, runnerDir, defaultLimits)
	case "register":
		return runRegister(args[1:], stdin, stdout)
	case "unregister":
		return runUnregister(args[1:], stdin, stdout)
	default:
		return fmt.Errorf("unknown command %q (want seed, register or unregister)", args[0])
	}
}

func runRegister(args []string, stdin io.Reader, stdout io.Writer) error {
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	o, err := parseRegister(args, host)
	if err != nil {
		return err
	}
	token, err := tokenFrom(o.tokenFile, stdin)
	if err != nil {
		return err
	}
	if err := requireRoot(); err != nil {
		return err
	}
	return register(context.Background(), runnerDir, o, token, execute, stdout)
}

func runUnregister(args []string, stdin io.Reader, stdout io.Writer) error {
	tokenFile, err := parseUnregister(args)
	if err != nil {
		return err
	}
	token, err := tokenFrom(tokenFile, stdin)
	if err != nil {
		return err
	}
	if err := requireRoot(); err != nil {
		return err
	}
	return unregister(context.Background(), runnerDir, token, execute, stdout)
}

func tokenFrom(path string, stdin io.Reader) (string, error) {
	if path == "-" {
		return readToken(stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	// Read-only handle: a close error cannot lose data.
	defer func() { _ = f.Close() }()
	return readToken(f)
}

func requireRoot() error {
	if os.Geteuid() != 0 {
		return errors.New("must run as root (use sudo): it starts config.sh as the runner account and manages the service")
	}
	return nil
}

// execute runs c, as the runner account when asked. It is the only place the
// program starts processes; everything else is pure or filesystem-only.
func execute(ctx context.Context, c cmdSpec) error {
	timeout := systemdTimeout
	if c.asRunner {
		timeout = configTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.args[0], c.args[1:]...)
	cmd.Dir = c.dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if c.asRunner {
		cred, err := runnerCredential()
		if err != nil {
			return err
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: cred}
		cmd.Env = c.env // deliberately not os.Environ(): see configEnv
	}
	return cmd.Run()
}

func runnerCredential() (*syscall.Credential, error) {
	u, err := user.Lookup(runnerUser)
	if err != nil {
		return nil, err
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, err
	}
	return &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: []uint32{uint32(gid)}}, nil
}
