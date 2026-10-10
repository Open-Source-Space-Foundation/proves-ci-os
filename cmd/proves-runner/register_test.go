package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseRegister(t *testing.T) {
	o, err := parseRegister([]string{"--labels", "integration,rig-1", "--name", "n1", "--runnergroup", "g"}, "host")
	if err != nil {
		t.Fatal(err)
	}
	if o.url != defaultURL || o.name != "n1" || o.group != "g" || !slices.Equal(o.labels, []string{"integration", "rig-1"}) || o.tokenFile != "-" {
		t.Fatalf("opts = %+v", o)
	}
	o, err = parseRegister([]string{"--labels", "x"}, "host")
	if err != nil || o.name != "host" {
		t.Fatalf("default name: %+v, %v", o, err)
	}
}

func TestParseRegisterFailsClosed(t *testing.T) {
	bad := map[string][]string{
		"no labels":        {},
		"empty label":      {"--labels", "a,,b"},
		"label with space": {"--labels", "a b"},
		"http url":         {"--labels", "x", "--url", "http://github.com/o"},
		"url credentials":  {"--labels", "x", "--url", "https://u:p@github.com/o"},
		"token as flag":    {"--labels", "x", "--token", "abc"},
		"positional":       {"--labels", "x", "extra"},
	}
	for name, args := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := parseRegister(args, "host"); err == nil {
				t.Fatal("want error")
			}
		})
	}
	if _, err := parseRegister([]string{"--labels", "x"}, ""); err == nil {
		t.Fatal("empty hostname and no --name: want error")
	}
}

func TestReadToken(t *testing.T) {
	if got, err := readToken(strings.NewReader("  ABC123\n")); err != nil || got != "ABC123" {
		t.Fatalf("got %q, %v", got, err)
	}
	for name, in := range map[string]string{
		"empty": "", "blank": " \n", "two words": "a b", "too long": strings.Repeat("a", maxTokenBytes+1),
	} {
		if _, err := readToken(strings.NewReader(in)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestConfigureArgs(t *testing.T) {
	got := configureArgs(registerOpts{url: "https://github.com/o", name: "n", labels: []string{"a", "b"}, group: "g", replace: true})
	want := []string{"./config.sh", "--unattended", "--url", "https://github.com/o", "--name", "n", "--labels", "a,b", "--runnergroup", "g", "--replace"}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %v", got)
	}
	for _, a := range got {
		if strings.Contains(a, "token") {
			t.Fatalf("token must not be on argv: %v", got)
		}
	}
}

func TestMergeEnv(t *testing.T) {
	cases := []struct{ before, after, want string }{
		{"", "A=1\n", "A=1\n"},
		{"A=1\nB=2\n", "A=1\n", "A=1\nB=2\n"},
		{"A=1\n", "A=1\nC=3\n", "A=1\nC=3\n"},
		{"B=2\n", "A=1", "A=1\nB=2\n"},
	}
	for _, c := range cases {
		if got := mergeEnv(c.before, c.after); got != c.want {
			t.Errorf("mergeEnv(%q,%q) = %q, want %q", c.before, c.after, got, c.want)
		}
	}
}

func seeded(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["config.sh"] = "cfg"
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

type recorder struct {
	calls []cmdSpec
	onRun func(cmdSpec) error
}

func (r *recorder) run(_ context.Context, c cmdSpec) error {
	r.calls = append(r.calls, c)
	if r.onRun != nil {
		return r.onRun(c)
	}
	return nil
}

func opts() registerOpts {
	return registerOpts{url: defaultURL, name: "n", labels: []string{"integration"}}
}

func TestRegisterRunsConfigAsRunnerThenStartsService(t *testing.T) {
	dir := seeded(t, map[string]string{".env": "LANG=C.UTF-8\nGDS=1\n"})
	rec := &recorder{onRun: func(c cmdSpec) error {
		if c.asRunner { // config.sh rewrites .env, dropping the hand-added line
			return os.WriteFile(filepath.Join(dir, ".env"), []byte("LANG=C.UTF-8\nNVM_BIN=/x\n"), 0o640)
		}
		return nil
	}}
	var out bytes.Buffer
	if err := register(context.Background(), dir, opts(), "TOK", rec.run, &out); err != nil {
		t.Fatal(err)
	}
	if len(rec.calls) != 3 || !rec.calls[0].asRunner || rec.calls[0].dir != dir {
		t.Fatalf("calls = %+v", rec.calls)
	}
	if !slices.Contains(rec.calls[0].env, tokenEnv+"=TOK") {
		t.Fatalf("token not passed via env: %v", rec.calls[0].env)
	}
	if !slices.Equal(rec.calls[1].args, []string{"systemctl", "enable", service}) || !slices.Equal(rec.calls[2].args, []string{"systemctl", "restart", service}) {
		t.Fatalf("service calls = %v %v", rec.calls[1].args, rec.calls[2].args)
	}
	if got := read(t, filepath.Join(dir, ".env")); got != "LANG=C.UTF-8\nNVM_BIN=/x\nGDS=1\n" {
		t.Fatalf(".env = %q", got)
	}
}

func TestRegisterIsIdempotent(t *testing.T) {
	dir := seeded(t, map[string]string{".runner": "reg"})
	rec := &recorder{}
	if err := register(context.Background(), dir, opts(), "TOK", rec.run, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("already registered: ran %+v", rec.calls)
	}
}

func TestRegisterReplaceDropsLocalRegistration(t *testing.T) {
	dir := seeded(t, map[string]string{".runner": "r", ".credentials": "c", ".credentials_rsaparams": "k"})
	rec := &recorder{onRun: func(c cmdSpec) error {
		if c.asRunner {
			for _, f := range []string{".runner", ".credentials", ".credentials_rsaparams"} {
				if _, err := os.Lstat(filepath.Join(dir, f)); err == nil {
					t.Errorf("%s still present when config.sh ran", f)
				}
			}
		}
		return nil
	}}
	o := opts()
	o.replace = true
	if err := register(context.Background(), dir, o, "TOK", rec.run, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(rec.calls[0].args, "--replace") {
		t.Fatalf("args = %v", rec.calls[0].args)
	}
}

func TestRegisterRequiresSeed(t *testing.T) {
	rec := &recorder{}
	if err := register(context.Background(), t.TempDir(), opts(), "TOK", rec.run, &bytes.Buffer{}); err == nil || len(rec.calls) != 0 {
		t.Fatalf("err = %v, calls = %v", err, rec.calls)
	}
}

func TestRegisterSurfacesConfigFailure(t *testing.T) {
	dir := seeded(t, map[string]string{})
	rec := &recorder{onRun: func(cmdSpec) error { return errors.New("boom") }}
	err := register(context.Background(), dir, opts(), "TOK", rec.run, &bytes.Buffer{})
	if err == nil || len(rec.calls) != 1 {
		t.Fatalf("err = %v, calls = %d: service must not start after a failed config", err, len(rec.calls))
	}
	if strings.Contains(err.Error(), "TOK") {
		t.Fatal("token leaked into error")
	}
}

func TestUnregister(t *testing.T) {
	dir := seeded(t, map[string]string{".runner": "r"})
	rec := &recorder{}
	if err := unregister(context.Background(), dir, "TOK", rec.run, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if len(rec.calls) != 2 || !slices.Equal(rec.calls[0].args, []string{"systemctl", "stop", service}) ||
		!slices.Equal(rec.calls[1].args, []string{"./config.sh", "remove", "--unattended"}) || !rec.calls[1].asRunner {
		t.Fatalf("calls = %+v", rec.calls)
	}
}

func TestUnregisterWhenNotRegistered(t *testing.T) {
	rec := &recorder{}
	if err := unregister(context.Background(), seeded(t, map[string]string{}), "TOK", rec.run, &bytes.Buffer{}); err != nil || len(rec.calls) != 0 {
		t.Fatalf("err = %v, calls = %v", err, rec.calls)
	}
}
