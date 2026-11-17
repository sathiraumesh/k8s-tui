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
		state: mvc.New(client).WithSplash(), // the start-up screen until the nodes load
	}
}

// Run starts the TUI full-screen and blocks until the user quits.
func (a *App) Run() error {
	// The app reads the mouse (mvc/mouse.go): clicks, the wheel one row a
	// step, dragging to select and copy, and a right-click menu. With
	// K8S_TUI_MOUSE=0 the mouse is left to the terminal.
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if os.Getenv("K8S_TUI_MOUSE") != "0" {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(a.state, opts...)
	_, err := p.Run()
	return err
}
