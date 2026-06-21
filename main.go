package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sollniss/gohelix/challenges"
	"github.com/sollniss/gohelix/ui"
)

func main() {
	challenge := flag.Int("c", 0, "start a specific challenge by number (1-20)")
	flag.Parse()

	chs, err := challenges.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading challenges: %v\n", err)
		os.Exit(1)
	}

	var m ui.Model
	if *challenge > 0 {
		if *challenge > len(chs) {
			fmt.Fprintf(os.Stderr, "Invalid challenge number: %d (must be 1-%d)\n", *challenge, len(chs))
			os.Exit(1)
		}
		m = ui.NewAt(chs, *challenge-1)
	} else {
		m = ui.New(chs)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
