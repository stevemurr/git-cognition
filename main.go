package main

import (
	"os"

	"github.com/stevemurr/git-cognition/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
