package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type probeModel struct{}

func (probeModel) Init() tea.Cmd                       { return nil }
func (probeModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return probeModel{}, nil }
func (probeModel) View() string                        { return "glade tui" }

func TestBubbleTeaProbeCompiles(t *testing.T) {
	var _ tea.Model = probeModel{}
}
