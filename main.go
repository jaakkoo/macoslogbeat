package main

import (
	"os"

	"github.com/jaakkoo/macoslogbeat/cmd"

	_ "github.com/jaakkoo/macoslogbeat/include"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
