package main

import (
	"io"
	"testing"
)

func TestMain_Error(t *testing.T) {
	origExit := exit
	var code int
	exit = func(c int) { code = c }
	defer func() { exit = origExit }()

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	// "check" without required --model/--field flags triggers an error.
	rootCmd.SetArgs([]string{"check"})
	main()

	if code != 1 {
		t.Errorf("want exit code 1, got %d", code)
	}
}
