package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	runnerDir  = "/var/lib/runner/actions-runner"
	runnerUser = "runner"
	service    = "actions-runner.service"
	defaultURL = "https://github.com/Open-Source-Space-Foundation"

	// The runner reads ACTIONS_RUNNER_INPUT_<NAME> as if it were --<name>, which
	// keeps the token out of argv, where any local user could read it.
	tokenEnv = "ACTIONS_RUNNER_INPUT_TOKEN"
	// config.sh snapshots PATH into .path, and jobs run with it; fix it here so
	// the sudo caller's environment does not leak into every job.
	jobPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin"

	maxTokenBytes = 4096
)

var labelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type registerOpts struct {
	url, name, group, tokenFile string
	labels                      []string
	replace                     bool
}

// cmdSpec describes a process to run. asRunner drops to the runner account.
type cmdSpec struct {
	args     []string
	env      []string
	dir      string
	asRunner bool
}

type runFn func(ctx context.Context, c cmdSpec) error

func parseRegister(args []string, hostname string) (registerOpts, error) {
	o := registerOpts{}
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	var labels string
	fs.StringVar(&o.url, "url", defaultURL, "organization URL to register with")
	fs.StringVar(&o.name, "name", hostname, "runner name")
	fs.StringVar(&o.group, "runnergroup", "", "runner group (default: the organization's default group)")
	fs.StringVar(&labels, "labels", "", "comma-separated labels, required (for example: integration)")
	fs.StringVar(&o.tokenFile, "token-file", "-", "file holding the registration token; - reads stdin")
	fs.BoolVar(&o.replace, "replace", false, "register again, replacing an existing runner of the same name")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	var err error
	if o.labels, err = parseLabels(labels); err != nil {
		return o, err
	}
	if o.name == "" {
		return o, errors.New("--name is empty and the hostname is unknown")
	}
	if err := validateURL(o.url); err != nil {
		return o, err
	}
	return o, nil
}

func parseUnregister(args []string) (tokenFile string, err error) {
	fs := flag.NewFlagSet("unregister", flag.ContinueOnError)
	fs.StringVar(&tokenFile, "token-file", "-", "file holding the remove token; - reads stdin")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if fs.NArg() != 0 {
		return "", fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	return tokenFile, nil
}

func parseLabels(s string) ([]string, error) {
	if s == "" {
		return nil, errors.New("--labels is required")
	}
	labels := strings.Split(s, ",")
	for _, l := range labels {
		if !labelPattern.MatchString(l) {
			return nil, fmt.Errorf("invalid label %q", l)
		}
	}
	return labels, nil
}

// validateURL rejects anything but a plain https URL; credentials in a URL
// would end up in .runner and the process list.
func validateURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid --url: %w", err)
	}
	if u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("--url %q must be an https URL without credentials", s)
	}
	return nil
}

// readToken reads a token from r, bounded, and rejects empty or multi-word input.
func readToken(r io.Reader) (string, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxTokenBytes+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxTokenBytes {
		return "", errors.New("token is too long")
	}
	t := strings.TrimSpace(string(b))
	if t == "" || strings.ContainsAny(t, " \t\r\n") {
		return "", errors.New("token must be a single non-empty word")
	}
	return t, nil
}

func configureArgs(o registerOpts) []string {
	args := []string{"./config.sh", "--unattended", "--url", o.url, "--name", o.name, "--labels", strings.Join(o.labels, ",")}
	if o.group != "" {
		args = append(args, "--runnergroup", o.group)
	}
	if o.replace {
		args = append(args, "--replace")
	}
	return args
}

func configEnv(token string) []string {
	return []string{"PATH=" + jobPath, "LANG=C.UTF-8", "HOME=" + filepath.Dir(runnerDir), tokenEnv + "=" + token}
}

// mergeEnv returns after plus any non-empty line of before that after lacks,
// so additions made to .env by hand or by the image survive config.sh.
func mergeEnv(before, after string) string {
	have := strings.Split(after, "\n")
	out := after
	for line := range strings.SplitSeq(before, "\n") {
		if line == "" || slices.Contains(have, line) {
			continue
		}
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += line + "\n"
	}
	return out
}

func exists(p string) (bool, error) {
	_, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func readOptional(p string) (string, error) {
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// register configures the runner against the organization and starts it. It
// does nothing when the runner is already registered unless o.replace is set.
func register(ctx context.Context, dir string, o registerOpts, token string, run runFn, out io.Writer) error {
	seededOK, err := exists(filepath.Join(dir, marker))
	if err != nil {
		return err
	}
	if !seededOK {
		return fmt.Errorf("runner is not seeded (%s missing); run: systemctl start actions-runner-seed.service", marker)
	}
	registered, err := exists(filepath.Join(dir, ".runner"))
	if err != nil {
		return err
	}
	if registered && !o.replace {
		_, err := fmt.Fprintln(out, "already registered; pass --replace to register again")
		return err
	}
	envPath := filepath.Join(dir, envFile)
	before, err := readOptional(envPath)
	if err != nil {
		return err
	}
	if o.replace {
		// config.sh refuses to run over an existing registration, and --replace
		// only takes over the name on the server side, so drop the local state.
		for _, f := range []string{".runner", ".credentials", ".credentials_rsaparams"} {
			if err := os.Remove(filepath.Join(dir, f)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	if err := run(ctx, cmdSpec{args: configureArgs(o), env: configEnv(token), dir: dir, asRunner: true}); err != nil {
		return fmt.Errorf("config.sh: %w", err)
	}
	after, err := readOptional(envPath)
	if err != nil {
		return err
	}
	if merged := mergeEnv(before, after); merged != after {
		// Writing the existing file keeps its owner and mode.
		if err := os.WriteFile(envPath, []byte(merged), 0o640); err != nil {
			return err
		}
	}
	for _, verb := range []string{"enable", "restart"} {
		if err := run(ctx, cmdSpec{args: []string{"systemctl", verb, service}}); err != nil {
			return fmt.Errorf("systemctl %s: %w", verb, err)
		}
	}
	_, err = fmt.Fprintln(out, "registered; the runner is starting")
	return err
}

// unregister stops the runner and removes its registration. Nothing to do
// when the runner is not registered.
func unregister(ctx context.Context, dir, token string, run runFn, out io.Writer) error {
	registered, err := exists(filepath.Join(dir, ".runner"))
	if err != nil {
		return err
	}
	if !registered {
		_, err := fmt.Fprintln(out, "not registered; nothing to do")
		return err
	}
	if err := run(ctx, cmdSpec{args: []string{"systemctl", "stop", service}}); err != nil {
		return fmt.Errorf("systemctl stop: %w", err)
	}
	remove := cmdSpec{args: []string{"./config.sh", "remove", "--unattended"}, env: configEnv(token), dir: dir, asRunner: true}
	if err := run(ctx, remove); err != nil {
		return fmt.Errorf("config.sh remove: %w", err)
	}
	_, err = fmt.Fprintln(out, "unregistered")
	return err
}
