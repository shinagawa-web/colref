package main

import (
	"io"
	"testing"
)

func TestRootCmd_Help(t *testing.T) {
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
