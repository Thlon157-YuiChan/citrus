package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/chihaya/citrus/internal/gui"
	"github.com/chihaya/citrus/internal/tui"
)

func main() {
	mode := flag.String("mode", "tui", "ui mode: tui or gui")
	flag.Parse()

	switch *mode {
	case "tui":
		if err := tui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "gui":
		if err := gui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s\n", *mode)
		os.Exit(2)
	}
}
