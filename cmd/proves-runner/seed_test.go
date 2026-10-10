package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type entry struct {
	name, body, link string
	typ              byte
}

func reg(name, body string) entry { return entry{name: name, body: body, typ: tar.TypeReg} }

func tarball(t *testing.T, entries ...entry) func() (io.ReadCloser, error) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0o755, Linkname: e.link}
		if e.typ == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(buf.Bytes())), nil }
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSeedUnpacksAndSeedsEnv(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "actions-runner")
	open := tarball(t,
		entry{name: "./", typ: tar.TypeDir},
		entry{name: "./bin", typ: tar.TypeDir},
		reg("./bin/runsvc.sh", "svc"),
		entry{name: "./bin/link", typ: tar.TypeSymlink, link: "runsvc.sh"},
		reg("./config.sh", "cfg"),
	)
	if err := seedRunner(open, dest, defaultLimits); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dest, "bin/link")); got != "svc" {
		t.Fatalf("symlink content = %q", got)
	}
	if got := read(t, filepath.Join(dest, ".env")); got != "LANG=C.UTF-8\n" {
		t.Fatalf(".env = %q", got)
	}
	if got := read(t, filepath.Join(dest, "config.sh")); got != "cfg" {
		t.Fatalf("config.sh = %q", got)
	}
}

func TestSeedTwiceLeavesACompleteSeedAlone(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "r")
	open := tarball(t, reg("./bin/x", "old"), reg("./config.sh", "cfg"))
	if err := seedRunner(open, dest, defaultLimits); err != nil {
		t.Fatal(err)
	}
	// The runner self-updates in place; a later seed must not downgrade it.
	if err := os.WriteFile(filepath.Join(dest, "bin/x"), []byte("updated"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := seedRunner(open, dest, defaultLimits); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dest, "bin/x")); got != "updated" {
		t.Fatalf("bin/x = %q, want it untouched", got)
	}
}

func TestSeedNeverOverwritesRegistrationOrEnv(t *testing.T) {
	dest := t.TempDir()
	keep := map[string]string{
		".runner":                "reg",
		".credentials":           "cred",
		".credentials_rsaparams": "key",
		".env":                   "LANG=en_US.UTF-8\nGDS=1\n",
	}
	for name, body := range keep {
		if err := os.WriteFile(filepath.Join(dest, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	open := tarball(t, reg("./.runner", "new"), reg("./.credentials", "new"),
		reg("./.credentials_rsaparams", "new"), reg("./config.sh", "cfg"))
	if err := seedRunner(open, dest, defaultLimits); err != nil {
		t.Fatal(err)
	}
	for name, body := range keep {
		if got := read(t, filepath.Join(dest, name)); got != body {
			t.Errorf("%s = %q, want %q", name, got, body)
		}
	}
}

func TestSeedRejectsHostileArchives(t *testing.T) {
	cases := map[string][]entry{
		"parent traversal":   {reg("../evil", "x")},
		"absolute path":      {reg("/etc/evil", "x")},
		"escaping symlink":   {{name: "l", typ: tar.TypeSymlink, link: "../../etc"}},
		"absolute symlink":   {{name: "l", typ: tar.TypeSymlink, link: "/etc"}},
		"write through link": {{name: "d", typ: tar.TypeSymlink, link: "."}, {name: "d/l", typ: tar.TypeSymlink, link: "../x"}},
		"hard link":          {{name: "h", typ: tar.TypeLink, link: "config.sh"}},
		"device":             {{name: "dev", typ: tar.TypeChar}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			dest := filepath.Join(parent, "r")
			err := seedRunner(tarball(t, append(entries, reg("config.sh", "cfg"))...), dest, defaultLimits)
			if err == nil {
				t.Fatal("want error")
			}
			if _, statErr := os.Lstat(filepath.Join(dest, "config.sh")); statErr == nil {
				t.Fatal("config.sh written despite failure; a half seed would look complete")
			}
			if _, statErr := os.Lstat(filepath.Join(parent, "evil")); statErr == nil {
				t.Fatal("escaped the destination")
			}
		})
	}
}

func TestSeedEnforcesLimits(t *testing.T) {
	open := tarball(t, reg("a", "x"), reg("b", "x"), reg("config.sh", "x"))
	if err := seedRunner(open, t.TempDir(), limits{maxEntries: 2, maxBytes: 1 << 20}); err == nil || !strings.Contains(err.Error(), "entries") {
		t.Fatalf("entries limit: err = %v", err)
	}
	big := tarball(t, reg("a", strings.Repeat("x", 100)), reg("config.sh", "x"))
	if err := seedRunner(big, t.TempDir(), limits{maxEntries: 10, maxBytes: 50}); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("size limit: err = %v", err)
	}
}
