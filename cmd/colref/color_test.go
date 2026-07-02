package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveColor(t *testing.T) {
	cases := []struct {
		mode    string
		isTTY   bool
		noColor bool
		want    bool
	}{
		{"always", false, true, true}, // always overrides non-TTY and NO_COLOR
		{"always", true, false, true},
		{"never", true, false, false}, // never overrides a TTY
		{"never", false, true, false},
		{"auto", true, false, true},   // TTY, NO_COLOR unset → on
		{"auto", false, false, false}, // not a TTY → off
		{"auto", true, true, false},   // NO_COLOR set → off
		{"auto", false, true, false},
		{"", true, false, true}, // empty mode behaves as auto
	}
	for _, c := range cases {
		if got := resolveColor(c.mode, c.isTTY, c.noColor); got != c.want {
			t.Errorf("resolveColor(%q, isTTY=%v, noColor=%v) = %v, want %v",
				c.mode, c.isTTY, c.noColor, got, c.want)
		}
	}
}

func TestValidateColor(t *testing.T) {
	for _, ok := range []string{"auto", "always", "never"} {
		if err := validateColor(ok); err != nil {
			t.Errorf("validateColor(%q) = %v, want nil", ok, err)
		}
	}
	if err := validateColor("rainbow"); err == nil {
		t.Error("validateColor(\"rainbow\") = nil, want error")
	}
}

func TestPalette(t *testing.T) {
	on := palette{enabled: true}
	if got := on.green("x"); got != ansiGreen+"x"+ansiReset {
		t.Errorf("green when enabled = %q", got)
	}
	// Every colored fragment must be wrapped when enabled.
	for name, got := range map[string]string{
		"green":  on.green("x"),
		"yellow": on.yellow("x"),
		"path":   on.path("x"),
		"line":   on.line("x"),
		"dim":    on.dim("x"),
	} {
		if !strings.HasPrefix(got, "\033[") || !strings.HasSuffix(got, ansiReset) {
			t.Errorf("%s when enabled = %q, want wrapped in ANSI codes", name, got)
		}
	}

	off := palette{enabled: false}
	for name, got := range map[string]string{
		"green":  off.green("x"),
		"yellow": off.yellow("x"),
		"path":   off.path("x"),
		"line":   off.line("x"),
		"dim":    off.dim("x"),
	} {
		if got != "x" {
			t.Errorf("%s when disabled = %q, want %q (unchanged)", name, got, "x")
		}
	}
}

func TestFileIsTerminal(t *testing.T) {
	// /dev/null is a character device, so it reports as a terminal. This is the
	// documented, deliberate tradeoff of the ModeCharDevice proxy (see
	// fileIsTerminal) — output to a dummy device is discarded, so it's harmless.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	if !fileIsTerminal(devNull) {
		t.Errorf("fileIsTerminal(%s) = false, want true (char device)", os.DevNull)
	}
	if err := devNull.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// A regular file is not a terminal.
	regular := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(regular, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := os.Open(regular)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if fileIsTerminal(f) {
		t.Errorf("fileIsTerminal(regular file) = true, want false")
	}
	// Stat on a closed file errors → treated as not a terminal.
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if fileIsTerminal(f) {
		t.Errorf("fileIsTerminal(closed file) = true, want false")
	}
}

func TestNewPalette(t *testing.T) {
	orig := flagColor
	t.Cleanup(func() { flagColor = orig })

	flagColor = "always"
	if !newPalette().enabled {
		t.Error("newPalette() with --color=always should be enabled regardless of TTY")
	}

	flagColor = "never"
	if newPalette().enabled {
		t.Error("newPalette() with --color=never should be disabled")
	}
}
