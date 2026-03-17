package main

import (
	"fmt"

	"github.com/izzoa/polycode/internal/tui"
)

// printBanner prints the polycode ASCII art logo to stdout.
func printBanner() {
	fmt.Println(tui.ASCIIArt)
	fmt.Printf("  polycode %s — multi-model consensus coding assistant\n\n", version)
}
