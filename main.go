package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LFroesch/gitty/internal/git"
	"github.com/LFroesch/gitty/internal/logger"
)

func main() {
	// Initialize logger
	if err := logger.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not initialize logger: %v\n", err)
	}
	defer logger.Close()

	// Check if we're in a git repo
	cwd, _ := os.Getwd()
	if !git.IsRepo(cwd) {
		fmt.Fprintln(os.Stderr, "Error: Not a git repository")
		os.Exit(1)
	}

	// Run the TUI
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
