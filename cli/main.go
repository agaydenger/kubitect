package main

import (
	"cli/cmd"
	"cli/ui"
	"os"
)

func main() {
	err := cmd.Execute()

	if err != nil {
		ui.PrintBlockE(err)
		os.Exit(1)
	}
}
