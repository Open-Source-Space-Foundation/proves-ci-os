package main

import (
	"strings"
	"testing"
)

const openocdManager = `{"customManagers":[{
  "customType":"regex",
  "managerFilePatterns":["/(^|/)Containerfile$/"],
  "matchStrings":["# renovate: datasource=(?<datasource>\\S+) depName=(?<depName>\\S+)\\nARG OPENOCD_REF=(?<currentValue>\\S+)"]
}]}`

func mustManagers(t *testing.T, cfg string) []manager {
	t.Helper()
	ms, err := parseManagers([]byte(cfg))
	if err != nil {
		t.Fatalf("parseManagers: %v", err)
	}
	return ms
}

func TestParseArgs(t *testing.T) {
	cf := "ARG A=1\nARG B\nFROM x\nARG C=\"two words\"\n"
	got := parseArgs(cf)
	if len(got) != 2 || got[0].name != "A" || got[1].name != "C" {
		t.Fatalf("parseArgs = %+v, want A and C only", got)
	}
}

func TestCheck(t *testing.T) {
	ms := mustManagers(t, openocdManager)
	annotated := "# renovate: datasource=github-tags depName=raspberrypi/openocd\nARG OPENOCD_REF=sdk-2.2.0\n"
	cases := []struct {
		name    string
		file    string
		content string
		want    string // substring of the single expected problem; "" means none
	}{
		{"matched", "Containerfile", annotated, ""},
		{"reformatted so no manager matches", "Containerfile", "ARG OPENOCD_REF=sdk-2.2.0\n", "OPENOCD_REF: matched by 0"},
		{"unknown ARG", "Containerfile", annotated + "ARG NEW_PIN=1\n", "NEW_PIN: matched by 0"},
		{"image ARG left to dockerfile manager", "Containerfile", "ARG BASE=quay.io/x/y:1@sha256:abc\nFROM ${BASE}\n", ""},
		{"file not covered by manager", "other/Containerfile.x", annotated, "OPENOCD_REF: matched by 0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := check(c.file, c.content, ms)
			switch {
			case c.want == "" && len(got) != 0:
				t.Fatalf("unexpected problems: %v", got)
			case c.want != "" && (len(got) != 1 || !strings.Contains(got[0], c.want)):
				t.Fatalf("problems = %v, want one containing %q", got, c.want)
			}
		})
	}
}

func TestCheckTwoManagersIsAProblem(t *testing.T) {
	ms := mustManagers(t, openocdManager)
	ms = append(ms, ms[0])
	got := check("Containerfile", "# renovate: datasource=a depName=b\nARG OPENOCD_REF=1\n", ms)
	if len(got) != 1 || !strings.Contains(got[0], "matched by 2") {
		t.Fatalf("problems = %v, want matched by 2", got)
	}
}

func TestCheckMultiLineMatchCoversEveryARGInSpan(t *testing.T) {
	ms := mustManagers(t, `{"customManagers":[{
	  "customType":"regex",
	  "managerFilePatterns":["/Containerfile$/"],
	  "matchStrings":["ARG RUNNER_VERSION=(?<currentValue>\\S+)\\nARG RUNNER_SHA256=(?<currentDigest>\\S+)"]
	}]}`)
	got := check("Containerfile", "ARG RUNNER_VERSION=2.1\nARG RUNNER_SHA256=ab\n", ms)
	if len(got) != 0 {
		t.Fatalf("unexpected problems: %v", got)
	}
}

func TestParseManagersFailsClosed(t *testing.T) {
	bad := map[string]string{
		"invalid json":      `{`,
		"glob file pattern": `{"customManagers":[{"customType":"regex","managerFilePatterns":["Containerfile"],"matchStrings":["x"]}]}`,
		"bad regex":         `{"customManagers":[{"customType":"regex","managerFilePatterns":["/x/"],"matchStrings":["("]}]}`,
		"no matchStrings":   `{"customManagers":[{"customType":"regex","managerFilePatterns":["/x/"]}]}`,
	}
	for name, cfg := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := parseManagers([]byte(cfg)); err == nil {
				t.Fatal("want error")
			}
		})
	}
}
