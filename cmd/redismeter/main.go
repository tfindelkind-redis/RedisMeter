// Package main is the entry point for the RedisMeter CLI.
package main

import (
	"os"

	"github.com/tfindelkind-redis/redismeter/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
