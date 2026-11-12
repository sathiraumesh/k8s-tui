package app

import (
	"k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc"

	tea "github.com/charmbracelet/bubbletea"
)

// App wires the pieces together and runs the TUI.
type App struct {
	state  mvc.Model
	client *kubernetes.Client // not used yet; nil until Kubernetes is wired in
}

// New creates the App with its starting state.
func New() *App {
	return &App{
		state: mvc.New(),
	}
}

// Run starts the TUI full-screen and blocks until the user quits.
func (a *App) Run() error {
	p := tea.NewProgram(a.state, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
