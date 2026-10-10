// Command check-renovate-pins asserts that every pinned ARG in the
// Containerfile is matched by exactly one Renovate custom manager (ADR 0002).
// A reformatted Containerfile otherwise stops a pin from being bumped, and
// nothing would notice.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const (
	configPath        = "renovate.json"
	containerfilePath = "Containerfile"
)

type manager struct {
	filePatterns []*regexp.Regexp
	matchStrings []*regexp.Regexp
}

type arg struct {
	name       string
	start, end int // byte span of the ARG line, excluding the newline
}

var (
	argLine  = regexp.MustCompile(`(?m)^ARG[ \t]+([A-Za-z_][A-Za-z0-9_]*)=.*$`)
	fromLine = regexp.MustCompile(`(?m)^FROM[ \t].*$`)
	varRef   = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?`)
)

// parseManagers extracts the regex custom managers from a renovate.json body.
// Anything it cannot interpret is an error: an unreadable manager must not
// look like "no manager".
func parseManagers(config []byte) ([]manager, error) {
	var cfg struct {
		CustomManagers []struct {
			CustomType          string   `json:"customType"`
			ManagerFilePatterns []string `json:"managerFilePatterns"`
			MatchStrings        []string `json:"matchStrings"`
		} `json:"customManagers"`
	}
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}
	managers := make([]manager, 0, len(cfg.CustomManagers))
	for i, cm := range cfg.CustomManagers {
		if cm.CustomType != "regex" {
			return nil, fmt.Errorf("customManagers[%d]: customType %q is not supported", i, cm.CustomType)
		}
		if len(cm.MatchStrings) == 0 {
			return nil, fmt.Errorf("customManagers[%d]: no matchStrings", i)
		}
		m := manager{}
		for _, p := range cm.ManagerFilePatterns {
			re, err := compileFilePattern(p)
			if err != nil {
				return nil, fmt.Errorf("customManagers[%d]: %w", i, err)
			}
			m.filePatterns = append(m.filePatterns, re)
		}
		for _, s := range cm.MatchStrings {
			// Renovate applies matchStrings with the multiline flag.
			re, err := regexp.Compile("(?m)" + s)
			if err != nil {
				return nil, fmt.Errorf("customManagers[%d]: matchString %q: %w", i, s, err)
			}
			m.matchStrings = append(m.matchStrings, re)
		}
		managers = append(managers, m)
	}
	return managers, nil
}

// compileFilePattern accepts only the /regex/ form of managerFilePatterns;
// globs would need a second matcher that could disagree with Renovate's.
func compileFilePattern(p string) (*regexp.Regexp, error) {
	if len(p) < 2 || !strings.HasPrefix(p, "/") || !strings.HasSuffix(p, "/") {
		return nil, fmt.Errorf("managerFilePatterns %q: only /regex/ patterns are supported", p)
	}
	re, err := regexp.Compile(p[1 : len(p)-1])
	if err != nil {
		return nil, fmt.Errorf("managerFilePatterns %q: %w", p, err)
	}
	return re, nil
}

// parseArgs returns every ARG that carries a default value (a pin).
func parseArgs(containerfile string) []arg {
	var args []arg
	for _, loc := range argLine.FindAllStringSubmatchIndex(containerfile, -1) {
		args = append(args, arg{
			name:  containerfile[loc[2]:loc[3]],
			start: loc[0],
			end:   loc[1],
		})
	}
	return args
}

// imageArgs names the ARGs used as `FROM ${NAME}`; the dockerfile manager
// owns those, so custom managers must not.
func imageArgs(containerfile string) map[string]bool {
	names := map[string]bool{}
	for _, from := range fromLine.FindAllString(containerfile, -1) {
		for _, m := range varRef.FindAllStringSubmatch(from, -1) {
			names[m[1]] = true
		}
	}
	return names
}

// covers reports whether any of m's matchStrings, applied to content,
// produces a match overlapping the byte span of a.
func (m manager) covers(path, content string, a arg) bool {
	applies := len(m.filePatterns) == 0
	for _, p := range m.filePatterns {
		applies = applies || p.MatchString(path)
	}
	if !applies {
		return false
	}
	for _, re := range m.matchStrings {
		for _, loc := range re.FindAllStringIndex(content, -1) {
			if loc[0] < a.end && a.start < loc[1] {
				return true
			}
		}
	}
	return false
}

// check returns one message per pinned ARG that is not matched by exactly
// one custom manager.
func check(path, content string, managers []manager) []string {
	images := imageArgs(content)
	var problems []string
	for _, a := range parseArgs(content) {
		if images[a.name] {
			continue
		}
		n := 0
		for _, m := range managers {
			if m.covers(path, content, a) {
				n++
			}
		}
		if n != 1 {
			problems = append(problems, fmt.Sprintf("%s: %s: matched by %d custom managers, want exactly 1", path, a.name, n))
		}
	}
	return problems
}

func run(stdout io.Writer) error {
	config, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(containerfilePath)
	if err != nil {
		return err
	}
	managers, err := parseManagers(config)
	if err != nil {
		return err
	}
	problems := check(containerfilePath, string(content), managers)
	for _, p := range problems {
		if _, err := fmt.Fprintln(stdout, p); err != nil {
			return err
		}
	}
	if len(problems) > 0 {
		return errors.New("pinned ARGs are not matched by exactly one custom manager")
	}
	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "check-renovate-pins:", err)
		os.Exit(1)
	}
}
