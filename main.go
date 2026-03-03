package main

import (
	"Curl2json/internal/cli"
	"os"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}