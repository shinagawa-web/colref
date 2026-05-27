package main

import (
	"fmt"
	"os"
)

var (
	exit    = os.Exit
	version = "dev"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		exit(1)
	}
}
