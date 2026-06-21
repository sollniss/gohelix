package ui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("5")). // magenta
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("13")). // bright magenta
			Bold(true)

	normalStyle = lipgloss.NewStyle() // default terminal foreground

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // bright black (gray)

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")). // yellow
			Italic(true).
			MarginBottom(1)

	passStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("2")) // green

	failStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("1")) // red

	recordStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("11")) // bright yellow

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")). // bright black (gray)
			MarginTop(1)

	scoreStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")) // yellow
)
