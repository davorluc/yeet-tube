package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"yeet-tube/tui"
)

func main() {
	p := tea.NewProgram(
		tui.InitialModel(),
		tea.WithAltScreen(), // <-- enable full-screen / alternate buffer
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running Yeet-Tube: %v\n", err)
		os.Exit(1)
	}
}
