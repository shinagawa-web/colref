package main

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestRootCmd_Help(t *testing.T) {
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRootCmd_Version(t *testing.T) {
	// Reset flag state left by any previous Execute call so cobra does not
	// short-circuit to Help before reaching the version flag check.
	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})

	rootCmd.Version = "v1.2.3"
	buf := new(strings.Builder)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--version"})
	if err := rootCmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "v1.2.3") {
		t.Errorf("version output %q does not contain v1.2.3", buf.String())
	}
}
