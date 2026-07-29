package picker_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sorodrigo/hop/picker"
	"github.com/sorodrigo/hop/tmux"
)

func threeSessions() []tmux.Session {
	return []tmux.Session{
		{Name: "quiet-otter", Windows: 2},
		{Name: "bold-cypress", Windows: 1},
		{Name: "amber-falcon", Windows: 3},
	}
}

func TestStartsOnTheFirstSession(t *testing.T) {
	m := picker.New(threeSessions())

	if got := highlighted(t, m); got != "quiet-otter" {
		t.Errorf("highlighted %q on open, want the first session %q", got, "quiet-otter")
	}
}

func TestArrowKeysMoveTheHighlight(t *testing.T) {
	m := press(t, picker.New(threeSessions()), tea.KeyMsg{Type: tea.KeyDown})

	if got := highlighted(t, m); got != "bold-cypress" {
		t.Errorf("after down, highlighted %q, want %q", got, "bold-cypress")
	}

	m = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if got := highlighted(t, m); got != "quiet-otter" {
		t.Errorf("after down then up, highlighted %q, want %q", got, "quiet-otter")
	}
}

func TestVimKeysMoveTheHighlight(t *testing.T) {
	m := press(t, picker.New(threeSessions()), runes("j"))

	if got := highlighted(t, m); got != "bold-cypress" {
		t.Errorf("after j, highlighted %q, want %q", got, "bold-cypress")
	}

	m = press(t, m, runes("k"))
	if got := highlighted(t, m); got != "quiet-otter" {
		t.Errorf("after j then k, highlighted %q, want %q", got, "quiet-otter")
	}
}

func TestHighlightWrapsAtBothEnds(t *testing.T) {
	// Wrapping means the last session is one keystroke away from the top.
	m := press(t, picker.New(threeSessions()), tea.KeyMsg{Type: tea.KeyUp})
	if got := highlighted(t, m); got != "amber-falcon" {
		t.Errorf("up from the first row highlighted %q, want the last row %q", got, "amber-falcon")
	}

	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if got := highlighted(t, m); got != "quiet-otter" {
		t.Errorf("down from the last row highlighted %q, want the first row %q", got, "quiet-otter")
	}
}

func TestEnterChoosesTheHighlightedSession(t *testing.T) {
	m := press(t, picker.New(threeSessions()), tea.KeyMsg{Type: tea.KeyDown})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	got, ok := m.Chosen()
	if !ok {
		t.Fatal("Chosen reported nothing after enter")
	}
	if got != "bold-cypress" {
		t.Errorf("Chosen = %q, want the highlighted session %q", got, "bold-cypress")
	}
}

func TestQuitKeysChooseNothing(t *testing.T) {
	keys := map[string]tea.KeyMsg{
		"q":      runes("q"),
		"esc":    {Type: tea.KeyEsc},
		"ctrl+c": {Type: tea.KeyCtrlC},
	}

	for name, key := range keys {
		t.Run(name, func(t *testing.T) {
			m := press(t, picker.New(threeSessions()), key)

			if got, ok := m.Chosen(); ok {
				t.Errorf("Chosen = %q after %s, want no selection", got, name)
			}
		})
	}
}

func TestQuitKeysStopTheProgram(t *testing.T) {
	// Without a quit command the picker would hang after the keypress.
	_, cmd := picker.New(threeSessions()).Update(runes("q"))

	if cmd == nil {
		t.Fatal("q returned no command, want tea.Quit")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Errorf("q returned %T, want tea.QuitMsg", cmd())
	}
}

func TestEnterStopsTheProgram(t *testing.T) {
	_, cmd := picker.New(threeSessions()).Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("enter returned no command, want tea.Quit")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Errorf("enter returned %T, want tea.QuitMsg", cmd())
	}
}

func TestEmptySessionListIsSurvivable(t *testing.T) {
	// The picker should never be opened with no sessions, but a crash here
	// would be a terrible failure mode for a race with a session ending.
	m := picker.New(nil)

	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if got, ok := m.Chosen(); ok {
		t.Errorf("Chosen = %q from an empty picker, want no selection", got)
	}
}

func TestViewListsEverySession(t *testing.T) {
	view := plain(picker.New(threeSessions()).View())

	for _, want := range []string{"quiet-otter", "bold-cypress", "amber-falcon"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing session %q:\n%s", want, view)
		}
	}
}

func TestViewShowsWindowCounts(t *testing.T) {
	view := plain(picker.New(threeSessions()).View())

	if !strings.Contains(view, "2 windows") {
		t.Errorf("view does not show the window count for quiet-otter:\n%s", view)
	}
	if !strings.Contains(view, "1 window") {
		t.Errorf("view does not singularise a single window:\n%s", view)
	}
}

func TestViewFlagsSessionsSomeoneIsAlreadyIn(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "quiet-otter", Windows: 1, Attached: true},
		{Name: "bold-cypress", Windows: 1},
	}

	view := plain(picker.New(sessions).View())

	otter, cypress := lineFor(t, view, "quiet-otter"), lineFor(t, view, "bold-cypress")
	if !strings.Contains(otter, "attached") {
		t.Errorf("attached session is not flagged:\n%s", otter)
	}
	if strings.Contains(cypress, "attached") {
		t.Errorf("detached session is flagged as attached:\n%s", cypress)
	}
}

func TestViewHighlightsExactlyOneRow(t *testing.T) {
	view := plain(picker.New(threeSessions()).View())

	if got := strings.Count(view, picker.Cursor); got != 1 {
		t.Errorf("view marks %d rows with the cursor, want exactly 1:\n%s", got, view)
	}
}

func TestViewExplainsTheControls(t *testing.T) {
	view := strings.ToLower(plain(picker.New(threeSessions()).View()))

	for _, want := range []string{"enter", "quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not mention %q in its help line:\n%s", want, view)
		}
	}
}

func TestRelativeTimeReadsNaturally(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{5 * time.Second, "just now"},
		{90 * time.Second, "1m ago"},
		{45 * time.Minute, "45m ago"},
		{2 * time.Hour, "2h ago"},
		{25 * time.Hour, "1d ago"},
		{72 * time.Hour, "3d ago"},
	}

	for _, c := range cases {
		if got := picker.RelativeTime(c.in); got != c.want {
			t.Errorf("RelativeTime(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// press applies a key to the model and returns the updated picker.
func press(t *testing.T, m picker.Model, key tea.KeyMsg) picker.Model {
	t.Helper()
	next, _ := m.Update(key)
	updated, ok := next.(picker.Model)
	if !ok {
		t.Fatalf("Update returned %T, want picker.Model", next)
	}
	return updated
}

func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// highlighted reads the cursor row back out of the rendered view, so the
// tests assert on what the user actually sees rather than internal state.
func highlighted(t *testing.T, m picker.Model) string {
	t.Helper()
	for _, line := range strings.Split(plain(m.View()), "\n") {
		if strings.Contains(line, picker.Cursor) {
			return strings.Fields(strings.TrimSpace(strings.ReplaceAll(line, picker.Cursor, "")))[0]
		}
	}
	t.Fatalf("no row is highlighted in view:\n%s", plain(m.View()))
	return ""
}

// lineFor returns the rendered row mentioning name.
func lineFor(t *testing.T, view, name string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, name) {
			return line
		}
	}
	t.Fatalf("no row for %q in view:\n%s", name, view)
	return ""
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansi.ReplaceAllString(s, "") }
