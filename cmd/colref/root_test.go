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
		if err := f.Value.Set(f.DefValue); err != nil {
			t.Fatalf("reset flag %q: %v", f.Name, err)
		}
		f.Changed = false
	})

	prevVersion := rootCmd.Version
	t.Cleanup(func() {
		rootCmd.Version = prevVersion
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	rootCmd.Version = "v1.2.3"
	buf := new(strings.Builder)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--version"})
	if err := rootCmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "colref v1.2.3\n" {
		t.Errorf("version output = %q, want %q", got, "colref v1.2.3\n")
	}
}
