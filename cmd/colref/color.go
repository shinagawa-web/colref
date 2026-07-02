package main

import (
	"fmt"
	"os"
)

// ANSI color codes. Kept as raw escape sequences (mirroring the palette style
// used elsewhere in the shinagawa-web toolchain, e.g. gomarklint's
// internal/output/text.go) to avoid pulling in a color-library dependency.
const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

// palette applies ANSI colors to output fragments. When disabled, every method
// returns its input unchanged, so the plain layout stays byte-for-byte
// identical to the pre-color output.
type palette struct {
	enabled bool
}

func (p palette) wrap(code, s string) string {
	if !p.enabled {
		return s
	}
	return code + s + ansiReset
}

// green emphasizes a positive heading ("References found").
func (p palette) green(s string) string { return p.wrap(ansiGreen, s) }

// yellow emphasizes a cautionary heading ("No references found").
func (p palette) yellow(s string) string { return p.wrap(ansiYellow, s) }

// path colors a file path.
func (p palette) path(s string) string { return p.wrap(ansiCyan, s) }

// line colors a ":line" suffix.
func (p palette) line(s string) string { return p.wrap(ansiGray, s) }

// dim de-emphasizes secondary text (e.g. the manual-verification hint).
func (p palette) dim(s string) string { return p.wrap(ansiGray, s) }

// resolveColor decides whether color should be emitted. It is pure so the
// decision logic is trivially testable.
//
//   - "always": always on (overrides NO_COLOR and a non-TTY destination)
//   - "never":  always off
//   - "auto":   on only when writing to a TTY and NO_COLOR is unset/empty
func resolveColor(mode string, isTTY, noColor bool) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	default: // "auto"
		return isTTY && !noColor
	}
}

// validateColor rejects unknown --color values.
func validateColor(mode string) error {
	switch mode {
	case "auto", "always", "never":
		return nil
	default:
		return fmt.Errorf("unknown --color %q: supported values are auto, always, never", mode)
	}
}

// fileIsTerminal reports whether f refers to a character device, used as a
// dependency-free proxy for "is a terminal".
//
// Deliberate tradeoff: dummy character devices such as /dev/null also read as
// terminals here, so `--color=auto` would emit ANSI codes when stdout is
// redirected to one. That is harmless — such output is discarded — and the
// cases that matter are handled correctly: regular files and pipes are not
// character devices, so color is disabled for `> file` and `| cmd`. A precise
// TTY check would need golang.org/x/term; we avoid that dependency here.
func fileIsTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// newPalette builds the palette from the --color flag and stdout's capabilities,
// honoring the NO_COLOR convention (https://no-color.org/).
func newPalette() palette {
	noColor := os.Getenv("NO_COLOR") != ""
	return palette{enabled: resolveColor(flagColor, fileIsTerminal(os.Stdout), noColor)}
}
