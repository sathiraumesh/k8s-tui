package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func main() {

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFD7")). // text color: cream
		Background(lipgloss.Color("#5F5FFF")).
		Padding(0, 2)

	inactive := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")). // gray text, no background
		Padding(0, 2)

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top,
		style.Render("Pods"),
		inactive.Render("Deployments"),
		inactive.Render("Services"),
	)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("tab switch • q quit")

	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#5F5FFF"))

	panel := box.Width(40).Height(4).Render("> nginx\n  redis\n  api")
	screen := lipgloss.JoinVertical(lipgloss.Left, tabBar, panel, footer)

	// fmt.Println(box.Width(10).Height(3).Render("nginx"))
	fmt.Println(screen)

	fmt.Println()
	fmt.Println("tab bar:", lipgloss.Width(tabBar), "wide x", lipgloss.Height(tabBar), "tall")
	fmt.Println("panel:  ", lipgloss.Width(panel), "wide x", lipgloss.Height(panel), "tall")
	fmt.Println("footer: ", lipgloss.Width(footer), "wide x", lipgloss.Height(footer), "tall")
	fmt.Println("screen: ", lipgloss.Width(screen), "wide x", lipgloss.Height(screen), "tall")

}
