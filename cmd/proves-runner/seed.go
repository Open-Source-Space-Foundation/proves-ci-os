package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// limits bound what a seed tarball may unpack to (resource use is a function
// of input we already limited). The pinned runner tarball is ~230 MB unpacked
// to well under 1 GiB, with ~11.5k entries.
type limits struct {
	maxEntries int
	maxBytes   int64
}

var defaultLimits = limits{maxEntries: 50_000, maxBytes: 2 << 30}

const (
	// marker is written last, so its presence means the seed is complete.
	marker  = "config.sh"
	envFile = ".env"
	envSeed = "LANG=C.UTF-8\n"
)

// isProtected names the registration state the seed must never overwrite.
func isProtected(name string) bool {
	return name == ".runner" || strings.HasPrefix(name, ".credentials")
}

// seedRunner unpacks the runner tarball into dest unless dest already holds a
// complete seed (or a self-updated runner). It is safe to run twice: a
// finished seed is left alone, and an interrupted one is completed without
// touching .runner or .credentials*. Every write goes through an os.Root, so
// a hostile archive cannot reach outside dest.
func seedRunner(open func() (io.ReadCloser, error), dest string, lim limits) error {
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	// Files are closed (and checked) where written; this is only the directory handle.
	defer func() { _ = root.Close() }()

	if _, err := root.Lstat(marker); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := unpack(open, root, lim, func(name string) bool { return name != marker }); err != nil {
		return fmt.Errorf("unpack seed: %w", err)
	}
	if err := seedEnv(root); err != nil {
		return err
	}
	if err := unpack(open, root, lim, func(name string) bool { return name == marker }); err != nil {
		return fmt.Errorf("unpack seed: %w", err)
	}
	return nil
}

// seedEnv creates .env with LANG=C.UTF-8 when absent and leaves an existing
// one alone. env.sh appends to .env without overwriting what is there.
func seedEnv(root *os.Root) error {
	f, err := root.OpenFile(envFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := f.WriteString(envSeed); err != nil {
		return errors.Join(err, f.Close())
	}
	return f.Close()
}

func unpack(open func() (io.ReadCloser, error), root *os.Root, lim limits, want func(string) bool) error {
	src, err := open()
	if err != nil {
		return err
	}
	// Read-only handles: a close error cannot lose data.
	defer func() { _ = src.Close() }()
	gz, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	var written int64
	for n := 0; ; n++ {
		if n >= lim.maxEntries {
			return fmt.Errorf("more than %d entries", lim.maxEntries)
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(hdr.Name)
		if name == "." {
			continue
		}
		if !filepath.IsLocal(name) {
			return fmt.Errorf("entry %q escapes the destination", hdr.Name)
		}
		if !want(name) {
			continue
		}
		size, err := extract(root, hdr, name, tr, lim.maxBytes-written)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		written += size
	}
}

// extract writes one entry and returns the bytes of file content written.
// budget is how many more bytes the archive may write.
func extract(root *os.Root, hdr *tar.Header, name string, tr io.Reader, budget int64) (int64, error) {
	if isProtected(name) {
		if _, err := root.Lstat(name); err == nil {
			return 0, nil
		}
	}
	mode := fs.FileMode(hdr.Mode & 0o777)
	switch hdr.Typeflag {
	case tar.TypeDir:
		return 0, root.MkdirAll(name, mode|0o700)
	case tar.TypeReg:
		return writeFile(root, name, mode, tr, budget)
	case tar.TypeSymlink:
		return 0, writeSymlink(root, name, hdr.Linkname)
	default:
		return 0, fmt.Errorf("unsupported entry type %q", hdr.Typeflag)
	}
}

func writeFile(root *os.Root, name string, mode fs.FileMode, r io.Reader, budget int64) (int64, error) {
	if err := root.MkdirAll(path.Dir(name), 0o750); err != nil {
		return 0, err
	}
	// Replace, never write through: an earlier run may have left a symlink.
	if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, io.LimitReader(r, budget+1))
	if err == nil && n > budget {
		err = errors.New("archive exceeds the size limit")
	}
	return n, errors.Join(err, f.Close())
}

func writeSymlink(root *os.Root, name, target string) error {
	if !filepath.IsLocal(path.Join(path.Dir(name), target)) || path.IsAbs(target) {
		return fmt.Errorf("symlink target %q escapes the destination", target)
	}
	if err := root.MkdirAll(path.Dir(name), 0o750); err != nil {
		return err
	}
	// The lexical check above only equals the real location when no parent
	// is itself a symlink.
	if err := rejectSymlinkParents(root, path.Dir(name)); err != nil {
		return err
	}
	if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return root.Symlink(target, name)
}

func rejectSymlinkParents(root *os.Root, dir string) error {
	parts := strings.Split(dir, "/")
	for i := range parts {
		p := strings.Join(parts[:i+1], "/")
		if p == "." {
			continue
		}
		info, err := root.Lstat(p)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("parent %q is a symlink", p)
		}
	}
	return nil
}
