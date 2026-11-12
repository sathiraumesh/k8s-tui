package app

import (
	"fmt"
	"k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// App wires the pieces together and runs the TUI.
type App struct {
	state mvc.Model
}

// New creates the App with its starting state.
func New() *App {
	client, err := kubernetes.NewClient()

	if err != nil {
		fmt.Fprintln(os.Stderr, "unable to create kubernetes client:", err)
		os.Exit(1)
	}

	return &App{
		state: mvc.New(client),
	}
}

// Run starts the TUI full-screen and blocks until the user quits.
func (a *App) Run() error {
	p := tea.NewProgram(a.state, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
