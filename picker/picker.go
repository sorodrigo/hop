// Package picker renders the interactive session list used by `hop --resume`.
package picker

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sorodrigo/hop/tmux"
)

// Cursor marks the highlighted row.
const Cursor = "❯ "

// Styles adapt to light and dark terminals; lipgloss drops the escape codes
// entirely when output is not a terminal, which keeps the view testable.
var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#0B5FFF", Dark: "#7AA2F7"})
	metaStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6C6C6C", Dark: "#9A9A9A"})
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#6C6C6C"})
)

// Model is the bubbletea model behind the picker. Navigation is pure state,
// so the whole interaction is testable without a terminal.
type Model struct {
	sessions []tmux.Session
	cursor   int
	chosen   string
	picked   bool
}

// New returns a picker over the given sessions.
func New(sessions []tmux.Session) Model {
	return Model{sessions: sessions}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "up", "k":
		m.cursor = wrap(m.cursor-1, len(m.sessions))
	case "down", "j":
		m.cursor = wrap(m.cursor+1, len(m.sessions))
	case "enter":
		if len(m.sessions) == 0 {
			return m, tea.Quit
		}
		m.chosen = m.sessions[m.cursor].Name
		m.picked = true
		return m, tea.Quit
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if len(m.sessions) == 0 {
		return metaStyle.Render("No sessions on the remote host.") + "\n"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Pick a session") + "\n\n")

	width := 0
	for _, session := range m.sessions {
		if n := utf8.RuneCountInString(session.Name); n > width {
			width = n
		}
	}

	for i, session := range m.sessions {
		label := "  " + session.Name
		if i == m.cursor {
			label = Cursor + session.Name
		}
		label += strings.Repeat(" ", width-utf8.RuneCountInString(session.Name))

		if i == m.cursor {
			label = selectedStyle.Render(label)
		}
		b.WriteString(label + "   " + metaStyle.Render(meta(session)) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("↑/↓ navigate · enter attach · q quit") + "\n")
	return b.String()
}

// Chosen reports the session the user selected, if any.
func (m Model) Chosen() (string, bool) {
	return m.chosen, m.picked
}

// Run displays the picker and returns the chosen session name, or "" if the
// user quit without choosing.
func Run(sessions []tmux.Session) (string, error) {
	final, err := tea.NewProgram(New(sessions)).Run()
	if err != nil {
		return "", fmt.Errorf("hop: session picker: %w", err)
	}

	model, ok := final.(Model)
	if !ok {
		return "", fmt.Errorf("hop: session picker returned %T", final)
	}

	name, _ := model.Chosen()
	return name, nil
}

// RelativeTime renders a duration the way the list labels session activity.
func RelativeTime(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// meta is the dimmed trailing column: window count, attachment, last activity.
func meta(session tmux.Session) string {
	parts := []string{windows(session.Windows)}

	if session.Attached {
		parts = append(parts, "attached")
	}
	// A zero activity time means the caller built the session by hand rather
	// than parsing tmux, so there is nothing meaningful to render.
	if !session.Activity.IsZero() {
		parts = append(parts, RelativeTime(time.Since(session.Activity)))
	}

	return strings.Join(parts, "   ")
}

func windows(n int) string {
	if n == 1 {
		return "1 window"
	}
	return fmt.Sprintf("%d windows", n)
}

// wrap keeps the cursor inside the list, rolling over at both ends.
func wrap(i, n int) int {
	if n == 0 {
		return 0
	}
	return ((i % n) + n) % n
}
