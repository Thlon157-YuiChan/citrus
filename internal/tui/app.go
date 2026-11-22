package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct{}

func (model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (model) View() string {
	return "Citrus (TUI)\npress q to quit\n"
}

func Run() error {
	p := tea.NewProgram(model{})
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
